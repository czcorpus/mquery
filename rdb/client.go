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

type Adapter struct {
	ctx                 context.Context
	c                   *redis.Client
	channelQuery        string
	channelResultPrefix string
	cachePath           string
}

func (a *Adapter) SomeoneListens(query Query) (bool, error) {
	cmd := a.c.PubSubNumSub(a.ctx, query.Channel)
	if cmd.Err() != nil {
		return false, fmt.Errorf("failed to check channel listeners: %w", cmd.Err())
	}
	return cmd.Val()[query.Channel] > 0, nil
}

// PublishQuery publishes a new query and returns query ID
func (a *Adapter) PublishQuery(query Query) (<-chan *WorkerResult, error) {
	query.Channel = fmt.Sprintf("%s:%s", a.channelResultPrefix, uuid.New().String())
	log.Debug().
		Str("channe", query.Channel).
		Str("func", query.Func).
		Any("args", query.Args).
		Msg("publishing query")

	msg, err := query.ToJSON()
	if err != nil {
		return nil, err
	}
	if err := a.c.LPush(a.ctx, DefaultQueueKey, msg).Err(); err != nil {
		return nil, err
	}
	sub := a.c.Subscribe(a.ctx, query.Channel)
	ans := make(chan *WorkerResult)

	// now we wait for response and send result via `ans`
	go func() {
		result := new(WorkerResult)

		item := <-sub.Channel()
		cmd := a.c.Get(a.ctx, item.Payload)
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
	return ans, a.c.Publish(a.ctx, a.channelQuery, MsgNewQuery).Err()
}

func (a *Adapter) DequeueQuery() (Query, error) {
	cmd := a.c.RPop(a.ctx, DefaultQueueKey)
	if cmd.Err() != nil {
		return Query{}, fmt.Errorf("failed to dequeue query: %w", cmd.Err())
	}
	q, err := DecodeQuery(cmd.Val())
	if err != nil {
		return Query{}, fmt.Errorf("failed to deserialize query: %w", err)
	}
	return q, nil
}

func (a *Adapter) PublishResult(channelName string, value *WorkerResult) error {
	log.Debug().
		Str("channel", channelName).
		Str("resultType", value.ResultType).
		Msg("publishing result")
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to serialize result: %w", err)
	}
	a.c.Set(a.ctx, channelName, string(data), DefaultResultExpiration)
	return a.c.Publish(a.ctx, channelName, channelName).Err()
}

func (a *Adapter) Subscribe() <-chan *redis.Message {
	sub := a.c.Subscribe(a.ctx, a.channelQuery)
	return sub.Channel()
}

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
		c: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", conf.Host, conf.Port),
			Password: conf.Password,
			DB:       conf.DB,
		}),
		ctx:                 context.Background(),
		channelQuery:        chQuery,
		channelResultPrefix: chRes,
		cachePath:           conf.CachePath,
	}
	return ans
}
