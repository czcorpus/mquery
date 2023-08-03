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
)

var (
	ErrorEmptyQueue = errors.New("no queries in the queue")
)

type Query struct {
	Channel string `json:"channel"`
	Func    string `json:"func"`
	Args    []any  `json:"args"`
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
	channelQuery        string
	channelResultPrefix string
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
		result := new(WorkerResult)

		item := <-sub.Channel()
		cmd := a.redis.Get(a.ctx, item.Payload)
		if cmd.Err() != nil {
			result.AttachValue(&results.ErrorResult{Error: cmd.Err().Error()})

		} else {
			err := json.Unmarshal([]byte(cmd.Val()), &result)
			if err != nil {
				result.AttachValue(&results.ErrorResult{Error: err.Error()})
			}
		}
		ans <- result
		sub.Close()
		close(ans)
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
		Str("resultType", value.ResultType).
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

	ans := &Adapter{
		redis: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", conf.Host, conf.Port),
			Password: conf.Password,
			DB:       conf.DB,
		}),
		ctx:                 context.Background(),
		channelQuery:        chQuery,
		channelResultPrefix: chRes,
	}
	return ans
}
