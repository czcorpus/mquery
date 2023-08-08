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

package qgen

import (
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
	word string,
	fromIdx,
	toIdx int,
) {
	for i := fromIdx; i < toIdx; i++ {
		item := items[i]
		wait, err := rc.executor.FxyQuery(rc.qGen, rc.corpusPath, word, item.Word)
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

// SortByLogDiceColl calculates log dice collocation
// for at most `preliminarySelSize` most frequent items and
// returns at most `resultSize` those items sorted by
// the collocation score.
// In case the `preliminarySelSize` is greater than
// the `resultSize`, the method call panics.
func (rc *ReorderCalculator) SortByLogDiceColl(
	word string,
	items []*results.FreqDistribItem,
	preliminarySelSize int,
	resultSize int,
) ([]*results.FreqDistribItem, error) {

	if preliminarySelSize < resultSize {
		panic("preliminarySelSize must be >= resultSize")
	}
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
	if chunkSize > 20 {
		chunkSize = 20
	}
	chunkHalf := chunkSize / 2
	items = items[:chunkSize]

	// Fy -> [deprel="nsubj" & p_upos="VERB" & p_lemma="win"]
	fyValues := make([]int64, len(items))

	// Fxy -> [lemma="team" & deprel="nsubj" & p_upos="VERB" & p_lemma="win"]
	fxyValues := make([]int64, len(items))

	wg := make(chan error)
	defer close(wg)

	// F(y)
	go func() {
		rc.calcFy(items, wg, fyValues, 0, chunkHalf)
	}()
	go func() {
		rc.calcFy(items, wg, fyValues, chunkHalf, chunkSize)
	}()

	// F(xy)
	go func() {
		rc.calcFxy(items, wg, fxyValues, word, 0, chunkHalf)
	}()
	go func() {
		rc.calcFxy(items, wg, fxyValues, word, chunkHalf, chunkSize)
	}()

	for i := 0; i < 4; i++ {
		err := <-wg
		if err != nil {
			return []*results.FreqDistribItem{}, err
		}
	}

	for i, item := range items {
		item.CollWeight = float64(fxyValues[i]) / (float64(item.Freq) + float64(fyValues[i]))
	}

	sort.SliceStable(
		items,
		func(i, j int) bool {
			return items[i].CollWeight > items[j].CollWeight
		},
	)
	resultChunkSize := 10
	if len(items) < resultChunkSize {
		resultChunkSize = len(items)
	}
	return items[:resultChunkSize], nil
}
