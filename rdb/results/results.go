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
	"math"
	"mquery/mango"
	"mquery/rdb"
	"slices"

	"github.com/czcorpus/cnc-gokit/util"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/rs/zerolog/log"
)

type FreqDistribItemList []*FreqDistribItem

// Cut makes the list at most maxItems long (i.e. in case
// the list is shorter, no error is triggered)
func (flist FreqDistribItemList) Cut(maxItems int) FreqDistribItemList {
	if len(flist) > maxItems {
		return flist[:maxItems]
	}
	return flist
}

// BinAsDataSeries groups time-series items based on the provided toInt function converting
// values (e.g. calendar dates) to ints so we can sort them.
func (flist FreqDistribItemList) BinAsDataSeries(toInt func(string) (int, error), analysisWindowSize int) FreqDistribItemList {
	if len(flist) < 5 {
		return flist
	}
	if analysisWindowSize <= 1 {
		panic("BinAsDataSeries - analysisWindowSize must be > 1")
	}

	dates := make([]binData, len(flist))
	for i, item := range flist {
		idx, err := toInt(item.Word)
		if err != nil {
			log.Error().
				Err(err).
				Str("value", item.Word).
				Msg("failed to process data item during data series binning")
			return flist
		}
		dates[i] = binData{
			dateBucket: idx,
			freq:       item.Freq,
			base:       item.Base,
			label:      item.Word,
		}
	}
	slices.SortFunc(
		dates,
		func(d1, d2 binData) int {
			return d1.dateBucket - d2.dateBucket
		},
	)

	movingZScores := bdMovingZScores(dates, analysisWindowSize)
	var maxZScore float64

	for _, item := range movingZScores {
		maxZScore = max(maxZScore, math.Abs(item))
	}

	// here is a heuristic for automatic binning:
	//
	// too many zero values => less detailed view
	numBins := len(flist)
	numZero := bdNumZeroFreq(dates)
	if numBins > 10 {
		if maxZScore < 2.5 {
			if numZero > len(dates)/10 {
				numBins = int(math.Ceil(float64(len(flist)) / 10))

			} else {
				numBins = int(math.Ceil(float64(len(flist)) / 5))
			}

		} else if maxZScore < 3.5 {
			numBins = int(math.Ceil(float64(len(flist)) / 3))
		}
	}
	itemsPerBin := int(math.RoundToEven(float64(len(flist)) / float64(numBins)))

	log.Debug().
		Int("totalItems", len(flist)).
		Int("numBins", numBins).
		Int("numZero", numZero).
		Int("itemsPerBin", itemsPerBin).
		Float64("maxZScore", maxZScore).
		Msg("determined num of bins for BinAsDataSeries")

	result := make(FreqDistribItemList, 0, len(dates))
	var currBin binData
	var totalFreq float64
	for i, item := range dates {
		totalFreq += float64(item.freq)
		currBin.base = item.base
		currBin.freq += item.freq
		currBin.numGrouped++
		if currBin.label == "" {
			currBin.label = item.label
		}
		if (i+1)%itemsPerBin == 0 {
			result = append(result, &FreqDistribItem{
				Word: currBin.label,
				Freq: currBin.freq,
				IPM:  float32(currBin.freq) / float32(currBin.base) * 1e6,
				Base: currBin.base,
			})
			currBin = binData{}
		}
	}
	if currBin.freq > 0 {
		result = append(result, &FreqDistribItem{
			Word: currBin.label,
			Freq: currBin.freq,
			IPM:  float32(currBin.freq) / float32(currBin.base) * 1e6,
			Base: currBin.base,
		})
	}
	return result
}

// AlwaysAsList returns an empty list in case the original
// value is nil.
func (flist FreqDistribItemList) AlwaysAsList() []*FreqDistribItem {
	if flist != nil {
		return flist
	}
	return []*FreqDistribItem{}
}

type FreqDistribItem struct {
	Word string  `json:"word"`
	Freq int64   `json:"freq"`
	Base int64   `json:"base"`
	IPM  float32 `json:"ipm"`
}

type WordFormsItem struct {
	Lemma    string              `json:"lemma"`
	Sublemma string              `json:"sublemma,omitempty"`
	POS      string              `json:"pos"`
	Forms    FreqDistribItemList `json:"forms"`
}

// ----

type FreqDistribResponse struct {
	ConcSize         int64               `json:"concSize"`
	CorpusSize       int64               `json:"corpusSize"`
	SubcSize         int64               `json:"subcSize,omitempty"`
	Freqs            FreqDistribItemList `json:"freqs"`
	Fcrit            string              `json:"fcrit"`
	ExamplesQueryTpl string              `json:"examplesQueryTpl,omitempty"`
	ResultType       rdb.ResultType      `json:"resultType"`
	Error            error               `json:"error,omitempty"`
} // @name Freq

type FreqDistrib struct {

	// ConcSize represents number of matching concordance rows
	ConcSize int64 `json:"concSize"`

	// CorpusSize is always equal to the whole corpus size
	// (even if we work with a subcorpus)
	CorpusSize int64 `json:"corpusSize"`

	// SubcSize shows a subcorpus size in case a subcorpus
	// is involved
	SubcSize int64 `json:"subcSize,omitempty"`

	Freqs FreqDistribItemList `json:"freqs"`

	// Fcrit a Manatee-encoded freq. criterion used with
	// this result. This is mostly useful (as an info for
	// a client) in case a default criterion is applied.
	Fcrit string `json:"fcrit"`

	// ExamplesQueryTpl provides a (CQL) query template
	// for obtaining examples matching words from the `Freqs`
	// atribute (one by one).
	ExamplesQueryTpl string `json:"examplesQueryTpl,omitempty"`

	Error error `json:"error,omitempty"`
}

func (res FreqDistrib) Err() error {
	return res.Error
}

func (res FreqDistrib) Type() rdb.ResultType {
	return rdb.ResultTypeFreqs
}

func (res *FreqDistrib) MarshalJSON() ([]byte, error) {
	return json.Marshal(FreqDistribResponse{
		ConcSize:         res.ConcSize,
		CorpusSize:       res.CorpusSize,
		SubcSize:         res.SubcSize,
		Freqs:            res.Freqs.AlwaysAsList(),
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
			v1.IPM = float32(v1.Freq) / float32(v1.Base) * 1e6

		} else {
			// orig IPM should be OK for the first item so no need to set it here
			res.Freqs = append(res.Freqs, v2)
		}
	}
}

// ----

type ConcSizeResponse struct {
	Total      int64          `json:"total"`
	ARF        float64        `json:"arf"`
	IPM        float64        `json:"ipm"`
	CorpusSize int64          `json:"corpusSize"`
	ResultType rdb.ResultType `json:"resultType"`
	Error      error          `json:"error,omitempty"`
} // @name ConcSize

type ConcSize struct {
	Total      int64   `json:"total"`
	ARF        float64 `json:"arf"`
	CorpusSize int64   `json:"corpusSize"`
	Error      error   `json:"error,omitempty"`
}

func (res ConcSize) Err() error {
	return res.Error
}

func (res ConcSize) Type() rdb.ResultType {
	return rdb.ResultTypeConcSize
}

func (res *ConcSize) MarshalJSON() ([]byte, error) {
	var ipm float64
	if res.CorpusSize > 0 {
		ipm = float64(res.Total) / float64(res.CorpusSize) * 1000000
	}
	return json.Marshal(
		ConcSizeResponse{
			Total:      res.Total,
			ARF:        rdb.NormRound(res.ARF),
			IPM:        rdb.NormRound(ipm),
			CorpusSize: res.CorpusSize,
			ResultType: res.Type(),
			Error:      res.Error,
		},
	)
}

// ----

type CollocationsResponse struct {
	CorpusSize int64               `json:"corpusSize"`
	ConcSize   int64               `json:"concSize"`
	SubcSize   int64               `json:"subcSize,omitempty"`
	Colls      []*mango.GoCollItem `json:"colls"`
	ResultType rdb.ResultType      `json:"resultType"`
	Measure    string              `json:"measure"`
	SrchRange  [2]int              `json:"srchRange"`
	Error      error               `json:"error,omitempty"`
} // @name Collocations

type Collocations struct {
	ConcSize   int64
	CorpusSize int64
	SubcSize   int64
	Colls      []*mango.GoCollItem
	Measure    string
	SrchRange  [2]int
	Error      error
}

func (res Collocations) Err() error {
	return res.Error
}

func (res Collocations) Type() rdb.ResultType {
	return rdb.ResultTypeCollocations
}

func (res *Collocations) MarshalJSON() ([]byte, error) {
	colls := res.Colls
	if colls == nil {
		colls = []*mango.GoCollItem{}
	}
	return json.Marshal(
		CollocationsResponse{
			CorpusSize: res.CorpusSize,
			SubcSize:   res.SubcSize,
			Colls:      colls,
			ResultType: res.Type(),
			Measure:    res.Measure,
			SrchRange:  res.SrchRange,
			Error:      res.Error,
		},
	)
}

// ----

type CollFreqData struct {
	Error error `json:"error,omitempty"`
}

func (res CollFreqData) Err() error {
	return res.Error
}

func (res CollFreqData) Type() rdb.ResultType {
	return rdb.ResultTypeCollFreqData
}

// ----

type ConcordanceResponse struct {
	Lines      []concordance.Line `json:"lines"`
	ConcSize   int                `json:"concSize"`
	CorpusSize int                `json:"corpusSize"`
	IPM        float64            `json:"ipm"`
	ResultType rdb.ResultType     `json:"resultType"`
	Error      error              `json:"error,omitempty"`
}

type ConcordanceLines []concordance.Line

func (cl ConcordanceLines) alwaysAsList() ConcordanceLines {
	if cl == nil {
		return []concordance.Line{}
	}
	return cl
}

// @name Concordance
type Concordance struct {
	Lines      ConcordanceLines
	ConcSize   int
	CorpusSize int
	IPM        float64
	Error      error
}

func (res Concordance) Err() error {
	return res.Error
}

func (res Concordance) Type() rdb.ResultType {
	return rdb.ResultTypeConcordance
}

func (res Concordance) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		ConcordanceResponse{
			Lines:      res.Lines.alwaysAsList(),
			ConcSize:   res.ConcSize,
			CorpusSize: res.CorpusSize,
			IPM:        util.Ternary(res.CorpusSize > 0, float64(res.ConcSize)/float64(res.CorpusSize)*1e6, 0),
			ResultType: res.Type(),
			Error:      res.Error,
		},
	)
}

// --------

type CorpusInfo struct {
	Data  corp.Overview
	Error error
}

func (res CorpusInfo) Err() error {
	return res.Error
}

func (res CorpusInfo) Type() rdb.ResultType {
	return rdb.ResultTypeCorpusInfo

}

func (res CorpusInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Data       corp.Overview  `json:"data"`
		ResultType rdb.ResultType `json:"resultType"`
		Error      error          `json:"error,omitempty"`
	}{
		Data:       res.Data,
		ResultType: res.Type(),
		Error:      res.Error,
	})
}

// -----------------

type TextTypeNorms struct {
	Sizes map[string]int64 `json:"sizes"`
	Error error            `json:"error,omitempty"`
}

func (res TextTypeNorms) Err() error {
	return res.Error
}

func (res TextTypeNorms) Type() rdb.ResultType {
	return rdb.ResultTypeTextTypeNorms
}

func (res TextTypeNorms) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sizes      map[string]int64 `json:"sizes"`
		ResultType rdb.ResultType   `json:"resultType"`
		Error      error            `json:"error,omitempty"`
	}{
		Sizes:      res.Sizes,
		ResultType: res.Type(),
		Error:      res.Error,
	})
}

// ----------------------------

type TokenContext struct {
	Context concordance.Line `json:"context"`
	Error   error            `json:"error,omitempty"`
}

func (res TokenContext) Err() error {
	return res.Error
}

func (res TokenContext) Type() rdb.ResultType {
	return rdb.ResultTypeTokenContext
}

func (res TokenContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Context    concordance.Line `json:"context"`
		ResultType rdb.ResultType   `json:"resultType"`
		Error      error            `json:"error,omitempty"`
	}{
		Context:    res.Context,
		ResultType: res.Type(),
		Error:      res.Error,
	})
}
