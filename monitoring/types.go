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
	"mquery/rdb"
	"time"
)

type StatusWriter interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
	Write(rec rdb.JobLog)
}

// ---

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
