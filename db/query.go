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

package db

import (
	"database/sql"
	"mquery/rdb"
)

type Backend struct {
	db *sql.DB
}

func (b *Backend) Insert(query string, args []any) (int64, error) {
	ans, err := b.db.Exec(query, args...)
	if err != nil {
		return -1, err
	}
	lid, err := ans.LastInsertId()
	if err != nil {
		return -1, err
	}
	return lid, nil
}

func (b *Backend) Select(query string, args []any) (*rdb.WorkerResult, error) {
	row := b.db.QueryRow(query, args...)
	ans := new(rdb.WorkerResult)
	err := row.Scan(&ans.Value, &ans.ResultType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ans, err
}

func NewBackend(db *sql.DB) *Backend {
	return &Backend{
		db: db,
	}
}
