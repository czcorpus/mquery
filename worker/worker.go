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
	"encoding/json"
	"fmt"
	"math/rand"
	"mquery/corpus/conc"
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
		var args rdb.FreqDistribArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.freqDistrib(args)
		ans.ResultType = query.ResultType
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	case "concSize":
		var args rdb.ConcSizeArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.concSize(args)
		ans.ResultType = query.ResultType
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	case "concExample":
		var args rdb.ConcExampleArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.concExample(args)
		ans.ResultType = query.ResultType
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	case "collocations":
		var args rdb.CollocationsArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.collocations(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	case "calcCollFreqData":
		var args rdb.CalcCollFreqDataArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.calcCollFreqData(args)
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

func (w *Worker) freqDistrib(args rdb.FreqDistribArgs) *results.FreqDistrib {
	var ans results.FreqDistrib
	freqs, err := mango.CalcFreqDist(args.CorpusPath, args.SubcPath, args.Query, args.Crit, args.FreqLimit)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	maxResults := args.MaxResults
	if maxResults == 0 {
		maxResults = MaxFreqResultItems
	}
	mergedFreqs := CompileFreqResult(freqs, freqs.SearchSize, MaxFreqResultItems)
	ans.Freqs = mergedFreqs
	ans.ConcSize = freqs.ConcSize
	ans.CorpusSize = freqs.CorpusSize
	return &ans
}

func (w *Worker) collocations(args rdb.CollocationsArgs) *results.Collocations {
	var ans results.Collocations
	colls, err := mango.GetCollcations(
		args.CorpusPath, args.SubcPath, args.Query, args.Attr, byte(args.CollFn[0]),
		args.MinFreq, args.MaxItems)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.Colls = colls.Colls
	ans.ConcSize = colls.ConcSize
	ans.CorpusSize = colls.CorpusSize
	return &ans
}

func (w *Worker) calcCollFreqData(args rdb.CalcCollFreqDataArgs) *results.CollFreqData {
	for _, attr := range args.Attrs {
		err := mango.CompileSubcFreqs(args.CorpusPath, args.SubcPath, attr)
		if err != nil {
			return &results.CollFreqData{Error: err.Error()}
		}
	}
	return &results.CollFreqData{}
}

func (w *Worker) concSize(args rdb.ConcSizeArgs) *results.ConcSize {
	var ans results.ConcSize
	concSizeInfo, err := mango.GetConcSize(args.CorpusPath, args.Query)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.ConcSize = concSizeInfo.Value
	ans.CorpusSize = concSizeInfo.CorpusSize
	return &ans
}

func (w *Worker) concExample(args rdb.ConcExampleArgs) *results.ConcExample {
	var ans results.ConcExample
	concEx, err := mango.GetConcExamples(args.CorpusPath, args.Query, args.Attrs, args.MaxItems)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	parser := conc.NewLineParser(args.Attrs)
	ans.Lines = parser.Parse(concEx)
	ans.ConcSize = concEx.ConcSize
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
