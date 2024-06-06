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
	"mquery/corpus/baseinfo"
	"mquery/corpus/infoload"
	"mquery/mango"
	"mquery/rdb"
	"mquery/results"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTickerInterval = 2 * time.Second
	MaxFreqResultItems    = 100
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
	var norms map[string]int64
	if args.IsTextTypes {
		attr := extractAttrFromTTCrit(args.Crit)
		norms, err = mango.GetTextTypesNorms(args.CorpusPath, attr)

		if err != nil {
			ans.Error = err.Error()
		}
	}
	mergedFreqs, err := CompileFreqResult(
		freqs, freqs.SearchSize, MaxFreqResultItems, norms)
	ans.Freqs = mergedFreqs
	ans.ConcSize = freqs.ConcSize
	ans.CorpusSize = freqs.CorpusSize
	ans.Fcrit = args.Crit
	return &ans
}

func (w *Worker) collocations(args rdb.CollocationsArgs) *results.Collocations {
	var ans results.Collocations
	msr, err := mango.ImportCollMeasure(args.Measure)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	colls, err := mango.GetCollcations(
		args.CorpusPath,
		args.SubcPath,
		args.Query,
		args.Attr,
		msr,
		args.SrchRange,
		args.MinFreq,
		args.MaxItems,
	)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.Colls = colls.Colls
	ans.ConcSize = colls.ConcSize
	ans.CorpusSize = colls.CorpusSize
	ans.SearchSize = colls.SearchSize
	ans.Measure = args.Measure
	ans.SrchRange = args.SrchRange
	return &ans
}

func (w *Worker) tokenCoverage(mktokencovPath, subcPath, corpusPath, structure string) error {
	cmd := exec.Command(mktokencovPath, corpusPath, structure, "-s", subcPath)
	return cmd.Run()
}

func (w *Worker) calcCollFreqData(args rdb.CalcCollFreqDataArgs) *results.CollFreqData {
	for _, attr := range args.Attrs {
		err := mango.CompileSubcFreqs(args.CorpusPath, args.SubcPath, attr)
		if err != nil {
			return &results.CollFreqData{Error: err.Error()}
		}
	}
	for _, strct := range args.Structs {
		err := w.tokenCoverage(args.MktokencovPath, args.SubcPath, args.CorpusPath, strct)
		if err != nil {
			return &results.CollFreqData{Error: err.Error()}
		}
	}
	return &results.CollFreqData{}
}

func (w *Worker) concSize(args rdb.TermFrequencyArgs) *results.ConcSize {
	var ans results.ConcSize
	concSizeInfo, err := mango.GetConcSize(args.CorpusPath, args.Query)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.Total = concSizeInfo.Value
	ans.CorpusSize = concSizeInfo.CorpusSize
	ans.ARF = concSizeInfo.ARF
	return &ans
}

func (w *Worker) concordance(args rdb.ConcordanceArgs) *results.Concordance {
	var ans results.Concordance
	concEx, err := mango.GetConcordance(
		args.CorpusPath, args.Query, args.Attrs, args.StartLine, args.MaxItems,
		args.MaxContext, args.ViewContextStruct)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	parser := concordance.NewLineParser(args.Attrs)
	ans.Lines = parser.Parse(concEx.Lines)
	ans.ConcSize = concEx.ConcSize
	return &ans
}

func (w *Worker) corpusInfo(args rdb.CorpusInfoArgs) *results.CorpusInfo {
	var ans results.CorpusInfo
	ans.Data = baseinfo.Corpus{Corpname: filepath.Base(args.CorpusPath)}
	t, err := fs.IsFile(args.CorpusPath)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	if !t {
		ans.Error = fmt.Sprintf("Invalid corpus path: %s", args.CorpusPath)
		return &ans
	}
	err = infoload.FillStructAndAttrs(args.CorpusPath, &ans.Data)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.Data.Size, err = mango.GetCorpusSize(args.CorpusPath)
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	ans.Data.Description, err = mango.GetCorpusConf(args.CorpusPath, "INFO")
	if err != nil {
		ans.Error = err.Error()
		return &ans
	}
	return &ans
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
