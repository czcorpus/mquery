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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mquery/results"
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
	ResultType results.ResultType `json:"resultType"`
	Channel    string             `json:"channel"`
	Func       string             `json:"func"`
	Args       json.RawMessage    `json:"args"`
}

type FreqDistribArgs struct {
	CorpusPath  string `json:"corpusPath"`
	SubcPath    string `json:"subcPath"`
	Query       string `json:"query"`
	Crit        string `json:"crit"`
	IsTextTypes bool   `json:"isTextTypes"`
	FreqLimit   int    `json:"freqLimit"`
	MaxResults  int    `json:"maxResults"`
}

type CollocationsArgs struct {
	CorpusPath string `json:"corpusPath"`
	SubcPath   string `json:"subcPath"`
	Query      string `json:"query"`
	Attr       string `json:"attr"`
	CollFn     string `json:"collFn"`
	MinFreq    int64  `json:"minFreq"`
	MaxItems   int    `json:"maxItems"`
}

type ConcSizeArgs struct {
	CorpusPath string `json:"corpusPath"`
	Query      string `json:"query"`
}

type ConcExampleArgs struct {
	CorpusPath    string   `json:"corpusPath"`
	QueryLemma    string   `json:"queryLemma"`
	Query         string   `json:"query"`
	Attrs         []string `json:"attrs"`
	ParentIdxAttr string   `json:"parentIdxAttr"`
	MaxItems      int      `json:"maxItems"`
}

type CalcCollFreqDataArgs struct {
	CorpusPath string   `json:"corpusPath"`
	SubcPath   string   `json:"subcPath"`
	Attrs      []string `json:"attrs"`

	// Structs any structure involved in possible text type
	// freq. distribution must be here so we can prepare
	// intermediate data
	Structs        []string `json:"structs"`
	MktokencovPath string   `json:"mktokencovPath"`
}

func (q Query) ToJSON() (string, error) {
	ans, err := json.Marshal(q)
	if err != nil {
		return "", err
	}
	return string(ans), nil
}

func DecodeQuery(q string) (Query, error) {
	var ans Query
	err := json.Unmarshal([]byte(q), &ans)
	return ans, err
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
}

func (a *Adapter) TestConnection(timeout time.Duration) error {

	tick := time.NewTicker(2 * time.Second)
	timeoutCh := time.After(timeout)
	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("failed to connect to the Redis server at %s", a.conf.ServerInfo())
		case <-tick.C:
			_, err := a.redis.Ping(a.ctx).Result()
			if err == nil {
				return nil
			}
			log.Info().
				Err(err).
				Str("server", a.conf.ServerInfo()).
				Msg("waiting for Redis server...")
		}
	}
}

// SomeoneListens tests if there is a listener for a channel
// specified in the provided `query`. If false, then there
// is nobody interested in the query anymore.
func (a *Adapter) SomeoneListens(query Query) (bool, error) {
	cmd := a.redis.PubSubNumSub(a.ctx, query.Channel)
	if cmd.Err() != nil {
		return false, fmt.Errorf("failed to check channel listeners: %w", cmd.Err())
	}
	return cmd.Val()[query.Channel] > 0, nil
}

// PublishQuery publishes a new query and returns a channel
// by which a respective result will be returned. In case the
// process fails during the calculation, a respective error
// is packed into the WorkerResult value. The error returned
// by this method means that the publishing itself failed.
func (a *Adapter) PublishQuery(query Query) (<-chan *WorkerResult, error) {
	query.Channel = fmt.Sprintf("%s:%s", a.channelResultPrefix, uuid.New().String())
	log.Debug().
		Str("channel", query.Channel).
		Str("func", query.Func).
		Any("args", query.Args).
		Msg("publishing query")

	msg, err := query.ToJSON()
	if err != nil {
		return nil, err
	}
	sub := a.redis.Subscribe(a.ctx, query.Channel)

	if err := a.redis.LPush(a.ctx, DefaultQueueKey, msg).Err(); err != nil {
		return nil, err
	}
	ans := make(chan *WorkerResult)

	// now we wait for response and send result via `ans`
	go func() {
		defer func() {
			sub.Close()
			close(ans)
		}()

		result := new(WorkerResult)
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
					result.AttachValue(
						&results.ErrorResult{
							ResultType: query.ResultType,
							Error:      cmd.Err().Error(),
						},
					)

				} else {
					err := json.Unmarshal([]byte(cmd.Val()), &result)
					if err != nil {
						result.AttachValue(&results.ErrorResult{Error: err.Error()})
					}
				}
				ans <- result
				tmr.Stop()
				return
			case <-tmr.C:
				result.AttachValue(
					&results.ErrorResult{
						Error: fmt.Sprintf("worker result waiting timeout (%v)", DefaultQueryAnswerTimeout),
					},
				)
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
func (a *Adapter) PublishResult(channelName string, value *WorkerResult) error {
	log.Debug().
		Str("channel", channelName).
		Str("resultType", value.ResultType.String()).
		Msg("publishing result")
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to serialize result: %w", err)
	}
	a.redis.Set(a.ctx, channelName, string(data), DefaultResultExpiration)
	return a.redis.Publish(a.ctx, channelName, channelName).Err()
}

// Subscribe subscribes to query queue.
func (a *Adapter) Subscribe() <-chan *redis.Message {
	sub := a.redis.Subscribe(a.ctx, a.channelQuery)
	return sub.Channel()
}

// NewAdapter is a recommended factory function
// for creating new `Adapter` instances
func NewAdapter(conf *Conf) *Adapter {
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
		ctx:                 context.Background(),
		channelQuery:        chQuery,
		channelResultPrefix: chRes,
		queryAnswerTimeout:  queryAnswerTimeout,
	}
	return ans
}
