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

package rdb

import (
	"encoding/json"
	"fmt"
	"mquery/results"
)

type WorkerResult struct {
	ResultType results.ResultType `json:"resultType"`
	Value      json.RawMessage    `json:"value"`
}

func (wr *WorkerResult) AttachValue(value results.SerializableResult) error {
	rawValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	wr.Value = rawValue
	return nil
}

func CreateWorkerResult(value results.SerializableResult) (*WorkerResult, error) {
	rawValue, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &WorkerResult{Value: rawValue, ResultType: value.Type()}, nil
}

func DeserializeFreqDistribResult(w *WorkerResult) (results.FreqDistrib, error) {
	var ans results.FreqDistrib
	err := json.Unmarshal(w.Value, &ans)
	if err != nil {
		return ans, fmt.Errorf("failed to deserialize FreqDistrib: %w", err)
	}
	return ans, nil
}

func DeserializeConcSizeResult(w *WorkerResult) (results.ConcSize, error) {
	var ans results.ConcSize
	err := json.Unmarshal(w.Value, &ans)
	if err != nil {
		return ans, fmt.Errorf("failed to deserialize ConcSize: %w", err)
	}
	return ans, nil
}

func DeserializeConcExampleResult(w *WorkerResult) (results.ConcExample, error) {
	var ans results.ConcExample
	err := json.Unmarshal(w.Value, &ans)
	if err != nil {
		return ans, fmt.Errorf("failed to deserialize ConcExample: %w", err)
	}
	return ans, nil
}

func DeserializeCollocationsResult(w *WorkerResult) (results.Collocations, error) {
	var ans results.Collocations
	err := json.Unmarshal(w.Value, &ans)
	if err != nil {
		return ans, fmt.Errorf("failed to deserialize Collocations: %w", err)
	}
	return ans, nil
}

func DeserializeCollFreqDataResult(w *WorkerResult) (results.CollFreqData, error) {
	var ans results.CollFreqData
	err := json.Unmarshal(w.Value, &ans)
	if err != nil {
		return ans, fmt.Errorf("failed to deserialize CollFreqData: %w", err)
	}
	return ans, nil
}
