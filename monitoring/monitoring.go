// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Institute of the Czech National Corpus,
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
	"fmt"
	"mquery/rdb"
	"time"

	"github.com/czcorpus/hltscl"
	"github.com/rs/zerolog/log"
)

/*
Expected tables:

create table mquery_operations_stats (
  "time" timestamp with time zone NOT NULL,
  num_jobs int,
  num_errors int,
  duration_secs float
);

select create_hypertable('mquery_operations_stats', 'time');
*/

type Conf struct {
	DB hltscl.PgConf `json:"db"`
}

// -----------------------------------

type TimescaleDBWriter struct {
	tableWriter *hltscl.TableWriter
	opsDataCh   chan<- hltscl.Entry
	errCh       <-chan hltscl.WriteError
	location    *time.Location
}

func (sw *TimescaleDBWriter) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("about to close StatusWriter")
				return
			case err := <-sw.errCh:
				log.Error().
					Err(err.Err).
					Str("entry", err.Entry.String()).
					Msg("error writing data to TimescaleDB")
				fmt.Println("reporting timescale write err: ", err.Err)
			}
		}
	}()
}

func (sw *TimescaleDBWriter) Stop(ctx context.Context) error {
	log.Warn().Msg("stopping StatusWriter")
	return nil
}

func (sw *TimescaleDBWriter) Write(item rdb.JobLog) {
	if sw.tableWriter != nil {
		var numErr int
		if item.Err != nil {
			numErr++
		}
		sw.opsDataCh <- *sw.tableWriter.NewEntry(time.Now().In(sw.location)).
			Int("num_jobs", 1).
			Int("num_errors", numErr).
			Float("duration_secs", item.TimeSpent().Seconds())
	}
}

func NewTimescaleDBWriter(
	ctx context.Context,
	conf hltscl.PgConf,
	tz *time.Location,
	onError func(err error),
) (*TimescaleDBWriter, error) {

	conn, err := hltscl.CreatePool(conf)
	if err != nil {
		return nil, err
	}
	twriter := hltscl.NewTableWriter(conn, "mquery_operations_stats", "time", tz)
	opsDataCh, errCh := twriter.Activate(
		ctx,
		hltscl.WithTimeout(20*time.Second),
	)

	return &TimescaleDBWriter{
		tableWriter: twriter,
		opsDataCh:   opsDataCh,
		errCh:       errCh,
		location:    tz,
	}, nil
}
