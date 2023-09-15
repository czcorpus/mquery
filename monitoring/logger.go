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
	"fmt"
	"mquery/results"
	"time"

	"github.com/rs/zerolog/log"
)

type WorkersLoad map[string]float64

type WorkerJobLogger struct {
	db       *sql.DB
	location *time.Location
}

func (w *WorkerJobLogger) Log(rec results.JobLog) {
	go func() {
		_, err := w.db.Exec(
			"INSERT INTO mquery_load_log (worker_id, start_dt, end_dt, func, err) "+
				"VALUES (?, ?, ?, ?, ?)",
			rec.WorkerID, rec.Begin, rec.End, rec.Func, rec.Err,
		)
		if err != nil {
			log.Error().
				Err(err).
				Str("func", rec.Func).
				Str("jobErr", rec.Err.Error()).
				Msg("failed to store load log")
		}
	}()
}

func (w *WorkerJobLogger) WorkersLoad(fromDT, toDT time.Time) (WorkersLoad, error) {
	query := "SELECT " +
		"SUM(" +
		"TIMESTAMPDIFF(SECOND, start_dt, end_dt) + " +
		"(EXTRACT(MICROSECOND FROM end_dt) - EXTRACT(MICROSECOND FROM start_dt)) / 1000000 " +
		") AS total_seconds, worker_id " +
		"FROM mquery_load_log " +
		"WHERE start_dt >= ? AND end_dt < ? " +
		"GROUP BY worker_id "
	args := []any{fromDT, toDT}
	ans := make(map[string]float64)
	rows, err := w.db.Query(query, args...)
	if err != nil {
		return ans, fmt.Errorf("failed to get total load: %w", err)
	}

	for rows.Next() {
		var load float64
		var workerID string
		rows.Scan(&load, &workerID)
		ans[fmt.Sprintf("worker_%s", workerID)] = load / toDT.Sub(fromDT).Seconds()
	}
	return ans, nil
}

func (w *WorkerJobLogger) TotalLoad(fromDT, toDT time.Time) (float64, error) {
	query := "SELECT AVG(t.total_seconds) FROM ( " +
		"SELECT " +
		"SUM(" +
		"TIMESTAMPDIFF(SECOND, start_dt, end_dt) + " +
		"(EXTRACT(MICROSECOND FROM end_dt) - EXTRACT(MICROSECOND FROM start_dt)) / 1000000 " +
		") AS total_seconds " +
		"FROM mquery_load_log " +
		"WHERE start_dt >= ? AND end_dt < ? " +
		"GROUP BY worker_id " +
		") AS t"
	args := []any{fromDT, toDT}
	row := w.db.QueryRow(query, args...)
	if row.Err() != nil {
		return -1, fmt.Errorf("failed to get total load: %w", row.Err())
	}
	var ans float64
	row.Scan(&ans) // note: err already tested by row.Err() above
	return ans / toDT.Sub(fromDT).Seconds(), nil
}

func (w *WorkerJobLogger) writeTimelineItem() error {
	now := time.Now().In(w.location)
	from := now.Add(-time.Second * 60)
	workerLoads, err := w.WorkersLoad(from, now)
	if err != nil {
		return err
	}
	for workerID, wload := range workerLoads {
		_, err := w.db.Exec(
			"INSERT INTO mquery_load_timeline (dt, wload, worker_id) "+
				"VALUES (?, ?, ?)",
			now, wload, workerID,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to insert mquery load timeline item")
		}
	}
	return nil
}

func (w *WorkerJobLogger) cleanupTimeline() error {
	now := time.Now().In(w.location)
	from := now.Add(-time.Hour * 24 * 14)
	_, err := w.db.Exec("DELETE FROM mquery_load_timeline WHERE dt <= ?", from)
	return err
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

func NewWorkerJobLogger(db *sql.DB, location *time.Location) *WorkerJobLogger {
	return &WorkerJobLogger{
		db:       db,
		location: location,
	}
}
