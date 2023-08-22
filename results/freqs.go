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

package results

import (
	"errors"
	"mquery/mango"
)

const (
	ResultTypeFx           = "Fx"
	ResultTypeFy           = "Fy"
	ResultTypeFxy          = "Fxy"
	ResultTypeCollocations = "Collocations"
	ResultTypeError        = "Error"
)

type ResultType string

func (rt ResultType) IsValid() bool {
	return rt == ResultTypeFx || rt == ResultTypeFy || rt == ResultTypeFxy
}

func (rt ResultType) String() string {
	return string(rt)
}

type WordFormsItem struct {
	Lemma string             `json:"lemma"`
	POS   string             `json:"pos"`
	Forms []*FreqDistribItem `json:"forms"`
}

type FreqDistribItem struct {
	Word       string  `json:"word"`
	Freq       int64   `json:"freq"`
	Norm       int64   `json:"norm"`
	IPM        float32 `json:"ipm"`
	CollWeight float64 `json:"collWeight"`
}

type SerializableResult interface {
	Type() ResultType
	Err() error
}

// ----

type FreqDistrib struct {
	ConcSize   int64              `json:"concSize"`
	CorpusSize int64              `json:"corpusSize"`
	Freqs      []*FreqDistribItem `json:"freqs"`

	// ExamplesQueryTpl provides a (CQL) query template
	// for obtaining examples matching words from the `Freqs`
	// atribute (one by one).
	ExamplesQueryTpl string     `json:"examplesQueryTpl"`
	ResultType       ResultType `json:"resultType"`
	Error            string     `json:"error"`
}

func (res *FreqDistrib) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *FreqDistrib) Type() ResultType {
	return res.ResultType
}

// ----

type ConcSize struct {
	ConcSize   int64      `json:"concSize"`
	CorpusSize int64      `json:"corpusSize"`
	ResultType ResultType `json:"resultType"`
	Error      string     `json:"error"`
}

func (res *ConcSize) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *ConcSize) Type() ResultType {
	return res.ResultType
}

// ----

type Collocations struct {
	ConcSize   int64               `json:"concSize"`
	CorpusSize int64               `json:"corpusSize"`
	Colls      []*mango.GoCollItem `json:"colls"`
	Error      string              `json:"error"`
}

func (res *Collocations) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *Collocations) Type() ResultType {
	return ResultTypeCollocations
}
