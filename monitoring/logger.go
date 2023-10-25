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
	"database/sql"
	"mquery/results"
	"time"
)

type WorkersLoad map[string]float64

type WorkerJobLogger struct {
	db       *sql.DB
	location *time.Location
}

func (w *WorkerJobLogger) Log(rec results.JobLog) {
	// TODO
}

func (w *WorkerJobLogger) WorkersLoad(fromDT, toDT time.Time) (WorkersLoad, error) {
	ans := make(map[string]float64)
	// TODO
	return ans, nil
}

func (w *WorkerJobLogger) TotalLoad(fromDT, toDT time.Time) (float64, error) {
	// TODO
	return 0, nil
}

func (w *WorkerJobLogger) writeTimelineItem() error {
	// TODO
	return nil
}

func (w *WorkerJobLogger) cleanupTimeline() error {
	// TODO
	return nil
}

func (w *WorkerJobLogger) GoRunTimelineWriter() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			w.writeTimelineItem()
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Hour)
		for range ticker.C {
			w.cleanupTimeline()
		}
	}()
}

func NewWorkerJobLogger(location *time.Location) *WorkerJobLogger {
	return &WorkerJobLogger{
		location: location,
	}
}
