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

package scoll

import (
	"math"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"sort"

	"github.com/rs/zerolog/log"
)

type ReorderCalculator struct {
	corpConf   *corpus.CorporaSetup
	corpusPath string
	qGen       QueryGenerator
	radapter   *rdb.Adapter
	executor   *QueryExecutor
}

func (rc *ReorderCalculator) calcFy(
	items []*results.FreqDistribItem,
	wg chan<- error,
	fyValues []int64,
	fromIdx,
	toIdx int,
) {
	for i := fromIdx; i < toIdx; i++ {
		item := items[i]
		wait, err := rc.executor.FyQuery(rc.qGen, rc.corpusPath, item.Word)
		if err != nil {
			wg <- err
			return
		}
		ans := <-wait
		result, err := rdb.DeserializeConcSizeResult(ans)
		if err != nil {
			wg <- err
			return
		}
		fyValues[i] = result.ConcSize
		log.Debug().
			Int("query", i).
			Int64("concSize", result.ConcSize).
			Msg("finished conc size query")
	}
	wg <- nil
}

func (rc *ReorderCalculator) calcFxy(
	items []*results.FreqDistribItem,
	wg chan<- error,
	fxyValues []int64,
	word Word,
	fromIdx,
	toIdx int,
) {
	for i := fromIdx; i < toIdx; i++ {
		item := items[i]
		wait, err := rc.executor.FxyQuery(
			rc.qGen, rc.corpusPath, word, item.Word)
		if err != nil {
			wg <- err
			return
		}
		ans := <-wait
		result, err := rdb.DeserializeConcSizeResult(ans)
		if err != nil {
			wg <- err
			return
		}
		fxyValues[i] = result.ConcSize
		log.Debug().
			Int("query", i).
			Int64("concSize", result.ConcSize).
			Msg("finished conc size query")
	}
	wg <- nil
}

// SortByLogDiceColl calculates best freq. candidates by
// cutting provided slice to a reasonable size specified
// in the `conf.CollPreliminarySel.Size`, then calculating
// collocation score for all the items and returning a slice
// of size `conf.CollResultSize`.
// The method creates multiple goroutines (configurable via
// `conf.NumParallelChunks`) where each goroutine handles
// a communication with a worker process.
func (rc *ReorderCalculator) SortByLogDiceColl(
	word Word,
	items []*results.FreqDistribItem,
	conf *SketchSetup,
) ([]*results.FreqDistribItem, error) {

	sort.SliceStable(
		items,
		func(i, j int) bool {
			return items[i].IPM > items[j].IPM
		},
	)
	// we take more than we need so there
	// is a chance that some lower freq items
	// with higher collocation value will
	// promote higher
	chunkSize := len(items)
	if chunkSize > conf.CollPreliminarySelSize {
		chunkSize = conf.CollPreliminarySelSize
	}
	items = items[:chunkSize]

	// Fy -> [deprel="nsubj" & p_upos="VERB" & p_lemma="win"]
	fyValues := make([]int64, len(items))

	// Fxy -> [lemma="team" & deprel="nsubj" & p_upos="VERB" & p_lemma="win"]
	fxyValues := make([]int64, len(items))

	wg := make(chan error)
	defer close(wg)

	runFx := func(from, to int) {
		rc.calcFy(items, wg, fyValues, from, to)
	}
	runFxy := func(from, to int) {
		rc.calcFxy(items, wg, fxyValues, word, from, to)
	}

	min := func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}

	procChunkSize := int(math.Ceil(float64(chunkSize) / float64(conf.NumParallelChunks)))
	var numRoutines int
	for i := 0; i < chunkSize; i += procChunkSize {
		to := min(i+procChunkSize, chunkSize)
		go runFx(i, to)
		go runFxy(i, to)
		numRoutines += 2
	}

	for i := 0; i < numRoutines; i++ {
		err := <-wg
		if err != nil {
			return []*results.FreqDistribItem{}, err
		}
	}

	for i, item := range items {
		item.CollWeight = 14 + math.Log2(2*float64(fxyValues[i])/(float64(item.Freq)+float64(fyValues[i])))
	}

	sort.SliceStable(
		items,
		func(i, j int) bool {
			return items[i].CollWeight > items[j].CollWeight
		},
	)
	resultChunkSize := conf.CollResultSize
	if len(items) < resultChunkSize {
		resultChunkSize = len(items)
	}
	return items[:resultChunkSize], nil
}
