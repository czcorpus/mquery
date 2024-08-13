// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
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

package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"mquery/rdb"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/rs/zerolog/log"
)

const (
	StaleWorkerLoadTTL       = time.Hour * 24
	tickerIntervalSecs int64 = 10
	recentLogSize            = 100
)

var (
	ErrWorkerNotFound = errors.New("worker not found")
)

type WorkerLoad struct {
	NumJobs       int
	TotalTimeSecs float64
	NumErrors     int
	FirstUpdate   time.Time
	LastUpdate    time.Time
	NumWorkers    int
}

// TotalSpan returns time span covered by the load info
func (wl WorkerLoad) TotalSpan() time.Duration {
	return wl.LastUpdate.Sub(wl.FirstUpdate)
}

func (wl WorkerLoad) AvgLoad() float64 {
	if wl.TotalTimeSecs == 0 {
		return 0
	}
	return wl.TotalTimeSecs / wl.TotalSpan().Seconds() / float64(wl.NumWorkers)
}

func (wl WorkerLoad) MarshalJSON() ([]byte, error) {
	var t0, t1 *time.Time
	if !wl.FirstUpdate.IsZero() {
		t0 = &wl.FirstUpdate
	}
	if !wl.LastUpdate.IsZero() {
		t1 = &wl.LastUpdate
	}
	return json.Marshal(
		struct {
			NumJobs       int        `json:"numJobs"`
			TotalTimeSecs float64    `json:"totalTimeSecs"`
			NumErrors     int        `json:"numErrors"`
			FirstUpdate   *time.Time `json:"firstUpdate,omitempty"`
			LastUpdate    *time.Time `json:"lastUpdate,omitempty"`
			AvgLoad       float64    `json:"avgLoad"`
		}{
			NumJobs:       wl.NumJobs,
			TotalTimeSecs: wl.TotalTimeSecs,
			NumErrors:     wl.NumErrors,
			FirstUpdate:   t0,
			LastUpdate:    t1,
			AvgLoad:       wl.AvgLoad(),
		},
	)
}

// ----------------

type WorkersLoad map[string]WorkerLoad

func (wl WorkersLoad) SumLoad(tz *time.Location) WorkerLoad {
	var ans WorkerLoad
	firstUpdate := time.Now().In(tz)
	for _, item := range wl {
		if item.LastUpdate.After(ans.LastUpdate) {
			ans.LastUpdate = item.LastUpdate
		}
		if item.FirstUpdate.Before(firstUpdate) {
			firstUpdate = item.FirstUpdate
			ans.FirstUpdate = item.FirstUpdate
		}
		ans.NumJobs += item.NumJobs
		ans.TotalTimeSecs += item.TotalTimeSecs
		ans.NumErrors += item.NumErrors
		ans.NumWorkers = len(wl)
	}
	return ans
}

func (wl WorkersLoad) cleanOldRecords() {
	for k, v := range wl {
		if time.Since(v.LastUpdate) > StaleWorkerLoadTTL {
			delete(wl, k)
		}
	}
}

// -----

type WorkerJobLogger struct {
	loadData  WorkersLoad
	recentLog *collections.CircularList[rdb.JobLog]
	tz        *time.Location
	numTicks  int64
}

func (w *WorkerJobLogger) Log(rec rdb.JobLog) {
	entry, ok := w.loadData[rec.WorkerID]
	if !ok {
		entry.FirstUpdate = rec.Begin
	}
	entry.NumJobs++
	entry.LastUpdate = rec.End
	if rec.Err != nil {
		entry.NumErrors++
	}
	entry.TotalTimeSecs += rec.End.Sub(rec.Begin).Seconds()
	w.loadData[rec.WorkerID] = entry
	w.recentLog.Append(rec)
}

func (w *WorkerJobLogger) TotalLoad() WorkerLoad {
	return w.loadData.SumLoad(w.tz)
}

func (w *WorkerJobLogger) RecentLoad() WorkerLoad {
	var ans WorkerLoad
	workers := collections.NewSet[string]()
	w.recentLog.ForEach(func(i int, item rdb.JobLog) bool {
		workers.Add(item.WorkerID)
		if i == 0 {
			ans.FirstUpdate = item.Begin
		}
		ans.LastUpdate = item.End
		if item.Err != nil {
			ans.NumErrors++
		}
		ans.NumJobs++
		ans.TotalTimeSecs += item.End.Sub(item.Begin).Seconds()
		return true
	})
	ans.NumWorkers = workers.Size()
	return ans
}

func (w *WorkerJobLogger) RecentRecords() []rdb.JobLog {
	ans := make([]rdb.JobLog, w.recentLog.Len())
	w.recentLog.ForEach(func(i int, item rdb.JobLog) bool {
		ans[i] = item
		return true
	})
	return ans
}

func (w *WorkerJobLogger) TotalWorkerLoad(workerID string) (WorkerLoad, error) {
	ans, ok := w.loadData[workerID]
	if !ok {
		return ans, ErrWorkerNotFound
	}
	return ans, nil
}

func (w *WorkerJobLogger) RecentWorkerLoad(workerID string) (WorkerLoad, error) {
	var ans WorkerLoad
	var found bool
	w.recentLog.ForEach(func(i int, item rdb.JobLog) bool {
		if item.WorkerID != workerID {
			return true
		}
		if !found {
			ans.FirstUpdate = item.End
			found = true
		}
		ans.LastUpdate = item.End
		if item.Err != nil {
			ans.NumErrors++
		}
		ans.NumJobs++
		ans.TotalTimeSecs += item.End.Sub(item.Begin).Seconds()
		return true
	})
	if found {
		ans.NumWorkers = 1
		return ans, nil
	}
	return ans, ErrWorkerNotFound
}

func (w *WorkerJobLogger) Start(ctx context.Context) {
	ticksPerCleanup := int64(StaleWorkerLoadTTL.Seconds()) / tickerIntervalSecs
	w.loadData = make(WorkersLoad)
	w.recentLog = collections.NewCircularList[rdb.JobLog](recentLogSize)
	log.Info().Msg("starting worker job logger")
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil { // should be always true here
					log.Info().Msg("requesting worker job logger stop")
				}
			case <-ticker.C:
				// TODO report to TimescaleDB (if configured)
				if w.numTicks%ticksPerCleanup == 0 {
					w.loadData.cleanOldRecords()
					w.numTicks = 0

				} else {
					w.numTicks++
				}
			}
		}
	}()

	/*
		go func() {
			ticker := time.NewTicker(time.Hour)
			for range ticker.C {
				w.cleanupTimeline()
			}
		}()
	*/
}

func (w *WorkerJobLogger) Stop(ctx context.Context) error {
	log.Info().Msg("shutting down worker job logger")
	return nil
}

func NewWorkerJobLogger(tz *time.Location) *WorkerJobLogger {
	return &WorkerJobLogger{
		tz: tz,
	}
}
