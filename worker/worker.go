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
	"errors"
	"fmt"
	"math/rand"
	"mquery/rdb"
	"mquery/results"
	"os"
	"os/exec"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTickerInterval = 2 * time.Second
)

type jobLogger interface {
	Log(rec results.JobLog)
}

type recoveredError struct {
	error
}

type Worker struct {
	ID         string
	messages   <-chan *redis.Message
	radapter   *rdb.Adapter
	exitEvent  chan os.Signal
	ticker     time.Ticker
	jobLogger  jobLogger
	currJobLog *results.JobLog
}

func (w *Worker) publishResult(res results.SerializableResult, channel string) error {
	ans, err := rdb.CreateWorkerResult(res)
	if err != nil {
		return err
	}

	w.currJobLog.End = time.Now()
	w.currJobLog.Err = res.Err()
	w.jobLogger.Log(*w.currJobLog)
	w.currJobLog = nil
	return w.radapter.PublishResult(channel, ans)
}

func (w *Worker) sendPublishingErr(query rdb.Query, err error) {
	if err := w.publishResult(&results.ErrorResult{Func: query.Func, Error: err.Error()}, query.Channel); err != nil {
		log.Error().Err(err).Msg("failed to publish general publishing error")
	}
}

func (w *Worker) runQueryProtected(query rdb.Query) (ansErr error) {
	defer func() {
		if r := recover(); r != nil {
			ansErr = recoveredError{fmt.Errorf(fmt.Sprintf("recovered error: %v", r))}
			return
		}
	}()
	switch query.Func {
	case "corpusInfo":
		var args rdb.CorpusInfoArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.corpusInfo(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			w.sendPublishingErr(query, err)
			return err
		}
	case "freqDistrib":
		var args rdb.FreqDistribArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.freqDistrib(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			w.sendPublishingErr(query, err)
			return err
		}
	case "termFrequency":
		var args rdb.TermFrequencyArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.concSize(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			w.sendPublishingErr(query, err)
			return err
		}
	case "concordance":
		var args rdb.ConcordanceArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.concordance(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			w.sendPublishingErr(query, err)
			return err
		}
	case "collocations":
		var args rdb.CollocationsArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.collocations(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			w.sendPublishingErr(query, err)
			return err
		}
	case "calcCollFreqData":
		var args rdb.CalcCollFreqDataArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.calcCollFreqData(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			w.sendPublishingErr(query, err)
			return err
		}
	default:
		ans := &results.ErrorResult{Error: fmt.Sprintf("unknown query function: %s", query.Func)}
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}
	}
	return nil
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

	w.currJobLog = &results.JobLog{
		WorkerID: w.ID,
		Func:     query.Func,
		Begin:    time.Now(),
	}

	err = w.runQueryProtected(query)
	var rcvErr recoveredError
	if errors.As(err, &rcvErr) {
		ans := &results.ErrorResult{
			Error: fmt.Sprintf("worker panicked: %s", rcvErr.Error()),
			Func:  query.Func,
		}
		if err := w.publishResult(ans, query.Channel); err != nil {
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

func (w *Worker) tokenCoverage(mktokencovPath, subcPath, corpusPath, structure string) error {
	cmd := exec.Command(mktokencovPath, corpusPath, structure, "-s", subcPath)
	return cmd.Run()
}

func NewWorker(
	workerID string,
	radapter *rdb.Adapter,
	messages <-chan *redis.Message,
	exitEvent chan os.Signal,
	jobLogger jobLogger,
) *Worker {
	return &Worker{
		ID:        workerID,
		radapter:  radapter,
		messages:  messages,
		exitEvent: exitEvent,
		ticker:    *time.NewTicker(DefaultTickerInterval),
		jobLogger: jobLogger,
	}
}
