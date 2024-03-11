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
	"encoding/json"
	"errors"
	"mquery/corpus/baseinfo"
	"mquery/mango"

	"github.com/czcorpus/mquery-common/concordance"
)

const (
	ResultTypeConcordance   = "conc"
	ResultTypeConcSize      = "concSize"
	ResultTypeCollocations  = "coll"
	ResultTypeCollFreqData  = "collFreqData"
	ResultTypeFreqs         = "freqs"
	ResultTypeMultipleFreqs = "multipleFreqs"
	ResultTypeCorpusInfo    = "corpusInfo"
	ResultTypeError         = "error"
)

type ResultType string

func (rt ResultType) String() string {
	return string(rt)
}

type LemmaItem struct {
	Lemma string `json:"lemma"`
	POS   string `json:"pos"`
}

type FreqDistribItemList []*FreqDistribItem

func (flist FreqDistribItemList) Cut(maxItems int) FreqDistribItemList {
	if len(flist) > maxItems {
		return flist[:maxItems]
	}
	return flist
}

type FreqDistribItem struct {
	Word string  `json:"word"`
	Freq int64   `json:"freq"`
	Norm int64   `json:"norm"`
	IPM  float32 `json:"ipm"`
}

type WordFormsItem struct {
	Lemma string              `json:"lemma"`
	POS   string              `json:"pos"`
	Forms FreqDistribItemList `json:"forms"`
}

type SerializableResult interface {
	Type() ResultType
	Err() error
}

// ----

type FreqDistrib struct {

	// ConcSize represents number of matching concordance rows
	ConcSize int64

	// CorpusSize is always equal to the whole corpus size
	// (even if we work with a subcorpus)
	CorpusSize int64

	// SearchSize is either equal to `CorpusSize` (in case
	// no subcorpus is involved) or equal to a respective
	// subcorpus size
	SearchSize int64

	Freqs FreqDistribItemList

	// Fcrit a Manatee-encoded freq. criterion used with
	// this result. This is mostly useful (as an info for
	// a client) in case a default criterion is applied.
	Fcrit string

	// ExamplesQueryTpl provides a (CQL) query template
	// for obtaining examples matching words from the `Freqs`
	// atribute (one by one).
	ExamplesQueryTpl string

	Error string
}

func (res *FreqDistrib) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *FreqDistrib) Type() ResultType {
	return ResultTypeFreqs
}

func (res *FreqDistrib) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ConcSize         int64               `json:"concSize"`
		CorpusSize       int64               `json:"corpusSize"`
		SearchSize       int64               `json:"searchSize"`
		Freqs            FreqDistribItemList `json:"freqs"`
		Fcrit            string              `json:"fcrit"`
		ExamplesQueryTpl string              `json:"examplesQueryTpl,omitempty"`
		ResultType       ResultType          `json:"resultType"`
		Error            string              `json:"error,omitempty"`
	}{
		ConcSize:         res.ConcSize,
		CorpusSize:       res.CorpusSize,
		SearchSize:       res.SearchSize,
		Freqs:            res.Freqs,
		Fcrit:            res.Fcrit,
		ExamplesQueryTpl: res.ExamplesQueryTpl,
		ResultType:       res.Type(),
		Error:            res.Error,
	})
}

func (res *FreqDistrib) FindItem(w string) *FreqDistribItem {
	for _, v := range res.Freqs {
		if v.Word == w {
			return v
		}
	}
	return nil
}

func (res *FreqDistrib) MergeWith(other *FreqDistrib) {
	res.ConcSize += other.ConcSize
	res.CorpusSize = other.CorpusSize // always the same value but to resolve possible initial 0
	res.ExamplesQueryTpl = ""         // we cannot merge two CQL queries so we remove it
	for _, v2 := range other.Freqs {
		v1 := res.FindItem(v2.Word)
		if v1 != nil {
			v1.Freq += v2.Freq
			v1.IPM = float32(v1.Freq) / float32(v1.Norm) * 1e6

		} else {
			// orig IPM should be OK for the first item so no need to set it here
			res.Freqs = append(res.Freqs, v2)
		}
	}
}

// ----

type ConcSize struct {
	ConcSize   int64
	CorpusSize int64
	Error      string
}

func (res *ConcSize) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *ConcSize) Type() ResultType {
	return ResultTypeConcSize
}

func (res *ConcSize) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			ConcSize   int64      `json:"concSize"`
			CorpusSize int64      `json:"corpusSize"`
			ResultType ResultType `json:"resultType"`
			Error      string     `json:"error,omitempty"`
		}{
			ConcSize:   res.ConcSize,
			CorpusSize: res.CorpusSize,
			ResultType: res.Type(),
			Error:      res.Error,
		},
	)
}

// ----

type Collocations struct {
	ConcSize   int64               `json:"concSize"`
	CorpusSize int64               `json:"corpusSize"`
	Colls      []*mango.GoCollItem `json:"colls"`
	Error      string              `json:"error,omitempty"`
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

func (res *Collocations) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			CorpusSize int64               `json:"corpusSize"`
			Colls      []*mango.GoCollItem `json:"colls"`
			ResultType ResultType          `json:"resultType"`
			Error      string              `json:"error,omitempty"`
		}{
			CorpusSize: res.CorpusSize,
			Colls:      res.Colls,
			ResultType: res.Type(),
			Error:      res.Error,
		},
	)

}

// ----

type CollFreqData struct {
	Error string `json:"error,omitempty"`
}

func (res *CollFreqData) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *CollFreqData) Type() ResultType {
	return ResultTypeCollFreqData
}

// ----

type Concordance struct {
	Lines    []concordance.Line
	ConcSize int
	Error    string
}

func (res *Concordance) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *Concordance) Type() ResultType {
	return ResultTypeConcordance
}

func (res Concordance) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			Lines      []concordance.Line `json:"lines"`
			ConcSize   int                `json:"concSize"`
			ResultType ResultType         `json:"resultType"`
			Error      string             `json:"error,omitempty"`
		}{
			Lines:      res.Lines,
			ConcSize:   res.ConcSize,
			ResultType: res.Type(),
			Error:      res.Error,
		},
	)
}

// --------

type CorpusInfo struct {
	Data  baseinfo.Corpus
	Error string
}

func (res *CorpusInfo) Err() error {
	if res.Error != "" {
		return errors.New(res.Error)
	}
	return nil
}

func (res *CorpusInfo) Type() ResultType {
	return ResultTypeCorpusInfo

}

func (res CorpusInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Data       baseinfo.Corpus `json:"data"`
		ResultType ResultType      `json:"resultType"`
		Error      string          `json:"error,omitempty"`
	}{
		Data:       res.Data,
		ResultType: res.Type(),
		Error:      res.Error,
	})
}
