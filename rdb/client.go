// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package rdb

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"mquery/merror"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	MsgNewQuery                = "newQuery"
	MsgNewResult               = "newResult"
	DefaultQueueKey            = "mqueryQueue"
	DefaultResultChannelPrefix = "mqueryResults"
	DefaultQueryChannel        = "mqueryQueries"
	DefaultResultExpiration    = 10 * time.Minute
	DefaultQueryAnswerTimeout  = 60 * time.Second
)

var (
	ErrorEmptyQueue = errors.New("no queries in the queue")
)

type Query struct {
	Channel string
	Func    string
	Args    any
}

// ----------------------

type CorpusInfoArgs struct {
	CorpusPath string
	Language   string
}

// --------------

type FreqDistribArgs struct {
	CorpusPath  string
	SubcPath    string
	Query       string
	Crit        string
	IsTextTypes bool
	FreqLimit   int
	MaxItems    int
}

// --------------

type CollocationsArgs struct {
	CorpusPath string
	SubcPath   string
	Query      string
	Attr       string
	Measure    string
	SrchRange  [2]int

	// MinFreq is the minimum frequency of the collocate in the collocation
	MinFreq int64

	// MinCorpFreq is the minimum frequency of the collocate in corpus
	MinCorpFreq int64
	MaxItems    int
}

// --------------

type TermFrequencyArgs ConcordanceArgs

// --------------

type ConcordanceArgs struct {
	CorpusPath        string
	SubcPath          string
	Query             string
	QueryLemma        string
	CollQuery         string
	CollLftCtx        int
	CollRgtCtx        int
	Attrs             []string
	ShowStructs       []string
	ShowRefs          []string
	MaxItems          int
	Shuffle           bool
	RowsOffset        int
	MaxContext        int
	ViewContextStruct string
	ParentIdxAttr     string
}

// AsDescription provides a human-readable representation
// suitable e.g. for reporting, error messages etc.
func (args *ConcordanceArgs) AsDescription() string {
	var ans strings.Builder
	ans.WriteString(args.Query)
	if args.CollQuery != "" {
		ans.WriteString(fmt.Sprintf(" (coll: %s)", args.CollQuery))
	}
	return ans.String()
}

// --------------

type CalcCollFreqDataArgs struct {
	CorpusPath string
	SubcPath   string
	Attrs      []string

	// Structs any structure involved in possible text type
	// freq. distribution must be here so we can prepare
	// intermediate data
	Structs        []string
	MktokencovPath string
}

// --------------

type TextTypeNormsArgs struct {
	CorpusPath string
	StructAttr string
}

// ---------------

type TokenContextArgs struct {
	CorpusPath string
	Idx        int64
	KWICLen    int64
	LeftCtx    int64
	RightCtx   int64
	Structs    []string
	Attrs      []string
}

// --------------

type StatusWriter interface {
	Write(rec JobLog)
}

// --------------

func DecodeQuery(q string) (Query, error) {
	var ans Query
	var buf bytes.Buffer
	buf.WriteString(q)
	dec := gob.NewDecoder(&buf)
	if err := dec.Decode(&ans); err != nil {
		return Query{}, err
	}
	return ans, nil
}

// Adapter provides functions for query producers and consumers
// using Redis database. It leverages Redis' PUBSUB functionality
// to notify about incoming data.
type Adapter struct {
	ctx                 context.Context
	redis               *redis.Client
	conf                *Conf
	channelQuery        string
	channelResultPrefix string
	queryAnswerTimeout  time.Duration
	statusWriter        StatusWriter
}

func (a *Adapter) TestConnection(timeout time.Duration) error {

	tick := time.NewTicker(2 * time.Second)
	ctx2, cancelFunc := context.WithTimeout(a.ctx, timeout)
	defer cancelFunc()

	for {
		select {
		case <-ctx2.Done():
			if ctx2.Err() == context.DeadlineExceeded {
				return fmt.Errorf("failed to connect to the Redis server at %s within the timeout period", a.conf.ServerInfo())
			}
			return fmt.Errorf("operation cancelled: %v", ctx2.Err())

		case <-tick.C:
			log.Info().
				Str("server", a.conf.ServerInfo()).
				Msg("waiting for Redis server...")
			_, err := a.redis.Ping(ctx2).Result()
			if err != nil {
				log.Error().Err(err).Msg("...failed to get response from Redis server")

			} else {
				log.Info().Msg("Successfully connected to Redis server")
				return nil
			}
		}
	}
}

// SomeoneListens tests if there is a listener for a channel
// specified in the provided `query`. If false, then there
// is nobody interested in the query anymore.
func (a *Adapter) SomeoneListens(channel string) (bool, error) {
	cmd := a.redis.PubSubNumSub(a.ctx, channel)
	if cmd.Err() != nil {
		return false, fmt.Errorf("failed to check channel listeners: %w", cmd.Err())
	}
	return cmd.Val()[channel] > 0, nil
}

// PublishQuery publishes a new query and returns a channel
// by which a respective result will be returned. In case the
// process fails during the calculation, a respective error
// is packed into the WorkerResult value. The error returned
// by this method means that the publishing itself failed.
func (a *Adapter) PublishQuery(query Query) (<-chan WorkerResult, error) {
	query.Channel = fmt.Sprintf("%s:%s", a.channelResultPrefix, uuid.New().String())
	log.Debug().
		Str("channel", query.Channel).
		Str("func", query.Func).
		Any("args", query.Args).
		Msg("publishing query")

	var msg bytes.Buffer
	enc := gob.NewEncoder(&msg)
	err := enc.Encode(query)
	if err != nil {
		return nil, err
	}
	sub := a.redis.Subscribe(a.ctx, query.Channel)
	if err := a.redis.LPush(a.ctx, DefaultQueueKey, msg.Bytes()).Err(); err != nil {
		return nil, err
	}
	ans := make(chan WorkerResult)

	// now we wait for response and send result via `ans`
	go func() {
		defer func() {
			sub.Close()
			close(ans)
		}()

		tmr := time.NewTimer(a.queryAnswerTimeout)

		for {
			select {
			case item, ok := <-sub.Channel():
				log.Debug().
					Str("channel", query.Channel).
					Bool("closedChannel", !ok).
					Msg("received result")
				cmd := a.redis.Get(a.ctx, item.Payload)
				if cmd.Err() != nil {
					ans <- WorkerResult{
						Value: ErrorResult{
							Func:  query.Func,
							Error: cmd.Err(),
						},
					}
					tmr.Stop()

				} else {
					var buff bytes.Buffer
					buff.WriteString(cmd.Val())
					dec := gob.NewDecoder(&buff)
					var wr WorkerResult
					err := dec.Decode(&wr)
					if err != nil {
						ans <- WorkerResult{
							Value: ErrorResult{
								Func:  query.Func,
								Error: err,
							},
						}
						a.statusWriter.Write(JobLog{
							WorkerID: "-",
							Func:     query.Func,
							Err:      fmt.Errorf("undecodable worker response: %w", err),
						})

					} else {
						ans <- wr
						a.statusWriter.Write(JobLog{
							WorkerID: wr.ID,
							Func:     string(wr.Value.Type()),
							Begin:    wr.ProcBegin,
							End:      wr.ProcEnd,
							Err:      wr.Value.Err(),
						})
					}
					tmr.Stop()
				}
				return
			case <-tmr.C:
				err := merror.TimeoutError{
					Msg: fmt.Sprintf("worker result timeouted (%v)", DefaultQueryAnswerTimeout),
				}
				ans <- WorkerResult{
					Value: ErrorResult{
						Func:  query.Func,
						Error: err,
					},
				}
				a.statusWriter.Write(JobLog{
					WorkerID: "-",
					Func:     query.Func,
					Err:      err,
				})
				return
			}
		}

	}()
	return ans, a.redis.Publish(a.ctx, a.channelQuery, MsgNewQuery).Err()
}

// DequeueQuery looks for a query queued for processing.
// In case nothing is found, ErrorEmptyQueue is returned
// as an error.
func (a *Adapter) DequeueQuery() (Query, error) {
	cmd := a.redis.RPop(a.ctx, DefaultQueueKey)

	if cmd.Val() == "" {
		return Query{}, ErrorEmptyQueue
	}
	if cmd.Err() != nil {
		return Query{}, fmt.Errorf("failed to dequeue query: %w", cmd.Err())
	}
	q, err := DecodeQuery(cmd.Val())
	if err != nil {
		return Query{}, fmt.Errorf("failed to deserialize query: %w", err)
	}
	return q, nil
}

// PublishResult sends notification via Redis PUBSUB mechanism
// and also stores the result so a notified listener can retrieve
// it.
func (a *Adapter) PublishResult(channelName string, value WorkerResult) error {
	log.Debug().
		AnErr("error", value.Value.Err()).
		Str("channel", channelName).
		Str("resultType", string(value.Value.Type())).
		Msg("publishing result")
	if value.Value.Err() != nil && IsUserErrorMsg(value.Value.Err().Error()) {
		value.HasUserError = true
	}
	var msg bytes.Buffer
	enc := gob.NewEncoder(&msg)
	err := enc.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to serialize result: %w", err)
	}
	cmd := a.redis.Set(a.ctx, channelName, msg.Bytes(), DefaultResultExpiration)
	if cmd.Err() != nil {
		return fmt.Errorf("failed to set result to Redis: %w", cmd.Err())
	}
	if err := a.redis.Publish(a.ctx, channelName, channelName).Err(); err != nil {
		return fmt.Errorf("failed to publish on Redis channel: %w", err)
	}
	return nil
}

// Subscribe subscribes to query queue.
func (a *Adapter) Subscribe() <-chan *redis.Message {
	sub := a.redis.Subscribe(a.ctx, a.channelQuery)
	return sub.Channel()
}

// NewAdapter is a recommended factory function
// for creating new `Adapter` instances
func NewAdapter(conf *Conf, ctx context.Context, statusWriter StatusWriter) *Adapter {
	chRes := conf.ChannelResultPrefix
	chQuery := conf.ChannelQuery
	if chRes == "" {
		chRes = DefaultResultChannelPrefix
		log.Warn().
			Str("channel", chRes).
			Msg("Redis channel for results not specified, using default")
	}
	if chQuery == "" {
		chQuery := DefaultQueryChannel
		log.Warn().
			Str("channel", chQuery).
			Msg("Redis channel for queries not specified, using default")
	}
	queryAnswerTimeout := time.Duration(conf.QueryAnswerTimeoutSecs) * time.Second
	if queryAnswerTimeout == 0 {
		queryAnswerTimeout = DefaultQueryAnswerTimeout
		log.Warn().
			Float64("value", queryAnswerTimeout.Seconds()).
			Msg("queryAnswerTimeoutSecs not specified for Redis adapter, using default")
	}
	ans := &Adapter{
		conf: conf,
		redis: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", conf.Host, conf.Port),
			Password: conf.Password,
			DB:       conf.DB,
		}),
		ctx:                 ctx,
		channelQuery:        chQuery,
		channelResultPrefix: chRes,
		queryAnswerTimeout:  queryAnswerTimeout,
		statusWriter:        statusWriter,
	}
	return ans
}
