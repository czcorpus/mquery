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
	"errors"
	"mquery/rdb"
	"sync"
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

type WorkerJobLogger struct {
	loadData     WorkersLoad
	dataLock     sync.RWMutex
	recentLog    *collections.CircularList[rdb.JobLog]
	tz           *time.Location
	numTicks     int64
	statusWriter StatusWriter
}

func (w *WorkerJobLogger) Log(rec rdb.JobLog) {
	w.dataLock.Lock()
	defer w.dataLock.Unlock()

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
	w.statusWriter.Write(rec)
}

func (w *WorkerJobLogger) TotalLoad() WorkerLoad {
	w.dataLock.RLock()
	defer w.dataLock.RUnlock()
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
	w.dataLock.RLock()
	defer w.dataLock.RUnlock()
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
				return
			case <-ticker.C:
				// TODO report to TimescaleDB (if configured)
				if w.numTicks%ticksPerCleanup == 0 {
					w.dataLock.Lock()
					w.loadData.cleanOldRecords()
					w.dataLock.Unlock()
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

func NewWorkerJobLogger(
	statusWriter StatusWriter,
	tz *time.Location,
) *WorkerJobLogger {
	return &WorkerJobLogger{
		statusWriter: statusWriter,
		tz:           tz,
	}
}
