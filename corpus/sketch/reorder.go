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

package sketch

import (
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"sort"

	"github.com/rs/zerolog/log"
)

type ReorderCalculator struct {
	corpConf   *corpus.CorporaSetup
	corpusPath string
	sketchConf CorpusSketchSetup
	radapter   *rdb.Adapter
}

func (rc *ReorderCalculator) SortByLogDiceColl(
	word string, items []*results.FreqDistribItem,
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
	items = items[:20]

	// Fy -> [deprel="nsubj" & p_upos="VERB" & p_lemma="win"]
	fyValues := make([]int64, len(items))

	// Fxy -> [lemma="team" & deprel="nsubj" & p_upos="VERB" & p_lemma="win"]
	fxyValues := make([]int64, len(items))

	wg := make(chan error)

	go func() {
		for i, item := range items {
			q := fmt.Sprintf(
				`[%s="%s" & %s="%s" & %s="%s"]`,
				rc.sketchConf.FuncAttr, rc.sketchConf.NounSubjectValue,
				rc.sketchConf.ParPosAttr, rc.sketchConf.VerbValue,
				rc.sketchConf.ParLemmaAttr, item.Word,
			)
			log.Debug().
				Int("query", i).
				Str("value", q).
				Msg("entering F(y) concSize query")
			wait, err := rc.radapter.PublishQuery(rdb.Query{
				Func: "concSize",
				Args: []any{rc.corpusPath, q},
			})
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
	}()

	// F(xy)
	go func() {
		for i, item := range items {
			q := fmt.Sprintf(
				`[%s="%s" & %s="%s" & %s="%s" & %s="%s"]`,
				rc.sketchConf.LemmaAttr, word,
				rc.sketchConf.FuncAttr, rc.sketchConf.NounSubjectValue,
				rc.sketchConf.ParPosAttr, rc.sketchConf.VerbValue,
				rc.sketchConf.ParLemmaAttr, item.Word,
			)
			log.Debug().
				Int("query", i).
				Str("value", q).
				Msg("entering F(xy) concSize query")
			wait, err := rc.radapter.PublishQuery(rdb.Query{
				Func: "concSize",
				Args: []any{rc.corpusPath, q},
			})
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
	}()

	err1 := <-wg // TODO
	err2 := <-wg // TODO
	close(wg)

	if err1 != nil {
		return items, err1
	}
	if err2 != nil {
		return items, err2
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

	return items[:10], nil
}

func NewReorderCalculator(
	corpConf *corpus.CorporaSetup,
	corpusPath string,
	sketchConf CorpusSketchSetup,
	radapter *rdb.Adapter,
) *ReorderCalculator {

	return &ReorderCalculator{
		corpConf:   corpConf,
		corpusPath: corpusPath,
		sketchConf: sketchConf,
		radapter:   radapter,
	}
}
