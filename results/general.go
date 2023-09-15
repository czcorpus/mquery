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

package results

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	ResultWorkerPerformance = "workerPerformance"
)

type JobLog struct {
	WorkerID string    `json:"workerId"`
	Func     string    `json:"func"`
	Begin    time.Time `json:"begin"`
	End      time.Time `json:"end"`
	Err      error     `json:"error"`
}

func (jl *JobLog) ToJSON() (string, error) {
	ans, err := json.Marshal(jl)
	if err != nil {
		return "", err
	}
	return string(ans), nil
}

type WorkerPerformance struct {
	Jobs  []*JobLog `json:"jobs"`
	Error string    `json:"error"`
}

func (res *WorkerPerformance) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *WorkerPerformance) Type() ResultType {
	return ResultWorkerPerformance
}
