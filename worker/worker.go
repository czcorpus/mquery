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

package worker

import (
	"fmt"
	"math/rand"
	"mquery/mango"
	"mquery/rdb"
	"mquery/results"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTickerInterval = 2 * time.Second
	MaxFreqResultItems    = 100
)

type Worker struct {
	messages  <-chan *redis.Message
	radapter  *rdb.Adapter
	exitEvent chan os.Signal
	ticker    time.Ticker
}

func (w *Worker) publishResult(res results.SerializableResult, channel string) error {
	ans, err := rdb.CreateWorkerResult(res)
	if err != nil {
		return err
	}
	return w.radapter.PublishResult(channel, ans)
}

func (w *Worker) tryNextQuery() error {
	time.Sleep(time.Duration(rand.Intn(40)) * time.Millisecond)
	query, err := w.radapter.DequeueQuery()
	if err == rdb.ErrorEmptyQueue {
		return nil

	} else if err != nil {
		return err
	}
	log.Debug().
		Str("channel", query.Channel).
		Str("func", query.Func).
		Any("args", query.Args).
		Msg("received query")

	isActive, err := w.radapter.SomeoneListens(query)
	if err != nil {
		return err
	}
	if !isActive {
		log.Warn().
			Str("func", query.Func).
			Str("channel", query.Channel).
			Any("args", query.Args).
			Msg("worker found an inactive query")
		return nil
	}

	switch query.Func {
	case "freqDistrib":
		ans := w.freqDistrib(query)
		ans.ResultType = query.ResultType
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	case "concSize":
		ans := w.concSize(query)
		ans.ResultType = query.ResultType
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	case "collocations":
		ans := w.collocations(query)
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	default:
		ans := &results.ErrorResult{Error: fmt.Sprintf("unknonw query function: %s", query.Func)}
		if err = w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) Listen() {
	for {
		select {
		case <-w.ticker.C:
			w.tryNextQuery()
		case <-w.exitEvent:
			log.Info().Msg("worker exiting")
			return
		case msg := <-w.messages:
			if msg.Payload == rdb.MsgNewQuery {
				w.tryNextQuery()
			}
		}
	}
}

func (w *Worker) freqDistrib(q rdb.Query) *results.FreqDistrib {
	var ans results.FreqDistrib
	corpusPath, ok := q.Args[0].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 0 (corpus ID) for freqDistrib %v", q.Args[0])
		return &ans
	}
	concQuery, ok := q.Args[1].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 1 (query) for freqDistrib %v", q.Args[1])
		return &ans
	}
	fcrit, ok := q.Args[2].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 2 (fcrit) for freqDistrib %v", q.Args[2])
		return &ans
	}
	flimit, ok := q.Args[3].(float64) // q.Args is []any, so json number interprets as float64
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 3 (flimit) for freqDistrib %v", q.Args[3])
		return &ans
	}
	freqs, err := mango.CalcFreqDist(corpusPath, concQuery, fcrit, int(flimit))
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	mergedFreqs := MergeFreqVectors(freqs, freqs.CorpusSize, MaxFreqResultItems)
	ans.Freqs = mergedFreqs
	ans.ConcSize = freqs.ConcSize
	ans.CorpusSize = freqs.CorpusSize
	return &ans
}

func (w *Worker) collocations(q rdb.Query) *results.Collocations {
	var ans results.Collocations

	corpusPath, ok := q.Args[0].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 0 (corpus ID) for collocations %v", q.Args[0])
		return &ans
	}
	concQuery, ok := q.Args[1].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 1 (query) for collocations %v", q.Args[1])
		return &ans
	}
	attrName, ok := q.Args[2].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 2 (attrName) for collocations %v", q.Args[2])
		return &ans
	}
	funcName, ok := q.Args[3].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 3 (attrName) for collocations %v", q.Args[3])
		return &ans
	}
	minFreq, ok := q.Args[4].(float64) // q.Args is []any, so json number interprets as float64
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 4 (minFreq) for collocations %v", q.Args[4])
		return &ans
	}
	maxItems, ok := q.Args[5].(float64) // q.Args is []any, so json number interprets as float64
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 5 (maxItems) for collocations %v", q.Args[5])
		return &ans
	}

	colls, err := mango.GetCollcations(
		corpusPath, concQuery, attrName, byte(funcName[0]), int64(minFreq), int(maxItems))
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.Colls = make([]results.CollItem, len(colls.Colls))
	for i, v := range colls.Colls {
		ans.Colls[i] = results.CollItem{
			Word:  v.Word,
			Value: v.Value,
			Freq:  v.Freq,
		}
	}
	ans.ConcSize = colls.ConcSize
	ans.CorpusSize = colls.CorpusSize
	return &ans
}

func (w *Worker) concSize(q rdb.Query) *results.ConcSize {
	var ans results.ConcSize
	corpusPath, ok := q.Args[0].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 0 (corpus ID) for concSize %v", q.Args[0])
		return &ans
	}
	concQuery, ok := q.Args[1].(string)
	if !ok {
		ans.Error = fmt.Sprintf("invalid argument 1 (query) for concSize %v", q.Args[1])
		return &ans
	}
	concSizeInfo, err := mango.GetConcSize(corpusPath, concQuery)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.ConcSize = concSizeInfo.Value
	ans.CorpusSize = concSizeInfo.CorpusSize
	return &ans
}

func NewWorker(radapter *rdb.Adapter, messages <-chan *redis.Message, exitEvent chan os.Signal) *Worker {
	return &Worker{
		radapter:  radapter,
		messages:  messages,
		exitEvent: exitEvent,
		ticker:    *time.NewTicker(DefaultTickerInterval),
	}
}
