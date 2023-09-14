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
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"mquery/corpus/conc"
	"mquery/mango"
	"mquery/rdb"
	"mquery/results"
	"os"
	"os/exec"
	"path"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	DefaultTickerInterval = 2 * time.Second
	MaxFreqResultItems    = 100
)

type Worker struct {
	ID                  string
	messages            <-chan *redis.Message
	radapter            *rdb.Adapter
	exitEvent           chan os.Signal
	ticker              time.Ticker
	lastJobLog          *results.JobLog
	performanceCacheDir string
}

func (w *Worker) makePerformanceCachePath() string {
	return path.Join(w.performanceCacheDir, w.ID+"-job-logs.jsonl")
}

func (w *Worker) logPerformance() error {
	f, err := os.OpenFile(w.makePerformanceCachePath(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
	if err != nil {
		return err
	}

	defer f.Close()

	log, err := w.lastJobLog.ToJSON()
	if err != nil {
		return err
	}
	if _, err = f.WriteString(log + "\n"); err != nil {
		return err
	}
	return nil
}

func (w *Worker) getPerformance(path string, fromDate, toDate *time.Time) ([]results.JobLog, error) {
	ans := make([]results.JobLog, 0, 100)
	file, err := os.Open(path)
	if err != nil {
		return ans, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		jobLog := results.JobLog{}
		json.Unmarshal([]byte(scanner.Text()), &jobLog)
		if (fromDate != nil && jobLog.Begin.Before(*fromDate)) || (toDate != nil && jobLog.Begin.After(*toDate)) {
			continue
		}
		ans = append(ans, jobLog)
	}

	if err := scanner.Err(); err != nil {
		return ans, err
	}
	return ans, nil
}

func (w *Worker) getAllPerformances(args rdb.WorkerPerformanceArgs) *results.WorkerPerformance {
	var fromDate, toDate *time.Time
	if len(args.FromDate) > 0 {
		date, err := time.Parse("2006-01-02", args.FromDate)
		if err != nil {
			return &results.WorkerPerformance{Error: err.Error()}
		}
		fromDate = &date
	}
	if len(args.ToDate) > 0 {
		date, err := time.Parse("2006-01-02", args.ToDate)
		if err != nil {
			return &results.WorkerPerformance{Error: err.Error()}
		}
		toDate = &date
	}
	entries, err := os.ReadDir(w.performanceCacheDir)
	if err != nil {
		return &results.WorkerPerformance{Error: err.Error()}
	}

	ans := make([]results.JobLog, 0, 100)
	for _, e := range entries {
		ansPart, err := w.getPerformance(path.Join(w.performanceCacheDir, e.Name()), fromDate, toDate)
		if err != nil {
			return &results.WorkerPerformance{Error: err.Error()}
		}
		ans = append(ans, ansPart...)
	}
	sort.Slice(ans, func(i int, j int) bool {
		return ans[i].Begin.Before(ans[j].Begin)
	})
	return &results.WorkerPerformance{Jobs: ans}
}

func (w *Worker) publishResult(res results.SerializableResult, channel string) error {
	ans, err := rdb.CreateWorkerResult(res)
	if err != nil {
		return err
	}
	w.lastJobLog.End = time.Now()
	w.lastJobLog.Err = res.Err()
	if err := w.logPerformance(); err != nil {
		log.Error().Err(err).Msg("Failed to save worker performance")
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

	w.lastJobLog = &results.JobLog{
		WorkerID: w.ID,
		Func:     query.Func,
		Begin:    time.Now(),
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
	case "workerPerformance":
		var args rdb.WorkerPerformanceArgs
		if err := json.Unmarshal(query.Args, &args); err != nil {
			return err
		}
		ans := w.getAllPerformances(args)
		if err := w.publishResult(ans, query.Channel); err != nil {
			return err
		}

	default:
		ans := &results.ErrorResult{Error: fmt.Sprintf("unknown query function: %s", query.Func)}
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
	parser := conc.NewLineParser(args.Attrs, args.ParentIdxAttr)
	ans.Lines = parser.Parse(concEx)
	ans.ConcSize = concEx.ConcSize
	return &ans
}

func NewWorker(radapter *rdb.Adapter, messages <-chan *redis.Message, exitEvent chan os.Signal, workerID string, workerPerformanceCacheDir string) *Worker {
	return &Worker{
		ID:                  workerID,
		radapter:            radapter,
		messages:            messages,
		exitEvent:           exitEvent,
		ticker:              *time.NewTicker(DefaultTickerInterval),
		performanceCacheDir: workerPerformanceCacheDir,
	}
}
