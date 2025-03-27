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
	"context"
	"fmt"
	"math/rand"
	"mquery/merror"
	"mquery/rdb"
	"mquery/rdb/results"
	"os/exec"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTickerInterval = 2 * time.Second
)

type Worker struct {
	ID         string
	messages   <-chan *redis.Message
	radapter   *rdb.Adapter
	ticker     time.Ticker
	normsCache *NormsCache
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-w.ticker.C:
				w.tryNextQuery()
			case <-ctx.Done():
				log.Info().Msg("about to close MQuery worker")
				return
			case msg := <-w.messages:
				if msg.Payload == rdb.MsgNewQuery {
					w.tryNextQuery()
				}
			}
		}
	}()
}

func (w *Worker) Stop(ctx context.Context) error {
	log.Warn().Str("workerId", w.ID).Msg("shutting down MQuery worker")
	return nil
}

func (w *Worker) publishResult(
	res rdb.FuncResult,
	query rdb.Query,
	t0 time.Time,
) error {
	return w.radapter.PublishResult(
		query.Channel,
		rdb.WorkerResult{
			ID:        w.ID,
			Value:     res,
			ProcBegin: t0,
			ProcEnd:   time.Now(),
		})
}

// runQueryProtected runs required function (query)
// and publishes result.
// During normal operations (which includes common errors
// returned from called functions), the function never returns
// the errors as they are returned to the calling client.
// Returned error means a serious problem has been encountered.
// This may happen in the following cases:
// 1) the called backend function panics
// 2) the function is unable to publish its result or error
func (w *Worker) runQueryProtected(query rdb.Query) (ansErr error) {
	defer func() {
		if r := recover(); r != nil {
			ansErr = merror.PanicValueToErr(r)
			return
		}
	}()
	t0 := time.Now()
	switch tArgs := query.Args.(type) {
	case rdb.CorpusInfoArgs:
		ans := w.corpusInfo(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.CorpusInfo{Error: err}, query, t0)
			return
		}
	case rdb.FreqDistribArgs:
		ans := w.freqDistrib(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.FreqDistrib{Error: err}, query, t0)
			return
		}
	case rdb.TermFrequencyArgs:
		ans := w.concSize(rdb.ConcordanceArgs(tArgs))
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.ConcSize{Error: err}, query, t0)
			return
		}
	case rdb.ConcordanceArgs:
		ans := w.concordance(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.Concordance{Error: err}, query, t0)
			return
		}
	case rdb.CollocationsArgs:
		ans := w.collocations(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.Collocations{Error: err}, query, t0)
			return
		}
	case rdb.CalcCollFreqDataArgs:
		ans := w.calcCollFreqData(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.CollFreqData{Error: err}, query, t0)
			return
		}
	case rdb.TextTypeNormsArgs:
		ans := w.textTypeNorms(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.TextTypeNorms{Error: err}, query, t0)
			return
		}
	case rdb.TokenContextArgs:
		ans := w.tokenContext(tArgs)
		if ans.Error != nil {
			ans.Error = wrapError(ans.Error)
		}
		if err := w.publishResult(ans, query, t0); err != nil {
			ansErr = w.publishResult(results.TextTypeNorms{Error: err}, query, t0)
			return
		}
	default:
		ans := rdb.ErrorResult{
			Error: merror.InternalError{
				Msg: fmt.Sprintf("unknown query function: %s", query.Func),
			},
		}
		if ansErr = w.publishResult(ans, query, t0); ansErr != nil {
			return
		}
	}
	return nil
}

func (w *Worker) tryNextQuery() {

	time.Sleep(time.Duration(rand.Intn(40)) * time.Millisecond)
	query, err := w.radapter.DequeueQuery()
	if err == rdb.ErrorEmptyQueue {
		return

	} else if err != nil {
		log.Error().Err(err).Msg("failed to fetch next job")
		return
	}
	log.Info().
		Str("workerId", w.ID).
		Str("channel", query.Channel).
		Str("func", query.Func).
		Any("args", query.Args).
		Msg("received query")

	isActive, err := w.radapter.SomeoneListens(query.Channel)
	if err != nil {
		log.Error().Err(err).Msg("failed to test channel listeners")
		return
	}
	if !isActive {
		log.Warn().
			Str("func", query.Func).
			Str("channel", query.Channel).
			Any("args", query.Args).
			Msg("worker found an inactive query")
		return
	}

	if err := w.runQueryProtected(query); err != nil {
		// if we're here, a more serious error likely occured,
		// but we still try to publish the result (even if the
		// publishing might have been the cause of the problem)
		if err2 := w.publishResult(
			rdb.ErrorResult{
				Error: wrapError(err),
				Func:  query.Func,
			},
			query,
			time.Now(),
		); err2 != nil {
			log.Error().Err(err2).Msg("failed to return worker error back to client")
		}
	}
}

func (w *Worker) tokenCoverage(mktokencovPath, subcPath, corpusPath, structure string) error {
	cmd := exec.Command(mktokencovPath, corpusPath, structure, "-s", subcPath)
	return cmd.Run()
}

func NewWorker(
	workerID string,
	radapter *rdb.Adapter,
	messages <-chan *redis.Message,
) *Worker {
	return &Worker{
		ID:         workerID,
		radapter:   radapter,
		messages:   messages,
		ticker:     *time.NewTicker(DefaultTickerInterval),
		normsCache: NewNormsCache(),
	}
}
