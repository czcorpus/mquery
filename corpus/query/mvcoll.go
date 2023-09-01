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

package query

import (
	"fmt"
	"math"
	"math/rand"
	"mquery/mango"
	"sort"
)

const (
	MinSampleSize = 20
)

type CollEstim struct {
	Word      string  `json:"word"`
	Mean      float64 `json:"mean"`
	Left      float64 `json:"leftCi"`
	Right     float64 `json:"rightCi"`
	NumChunks int     `json:"numChunks"`
}

type MultivalueColls struct {
	Values map[string][]*mango.GoCollItem
}

func (mvc *MultivalueColls) Add(values []*mango.GoCollItem) {
	for _, v := range values {
		_, ok := mvc.Values[v.Word]
		if !ok {
			mvc.Values[v.Word] = make([]*mango.GoCollItem, 0, 20)
		}
		mvc.Values[v.Word] = append(mvc.Values[v.Word], v)
	}
}

func (mvc *MultivalueColls) ForEach(fn func(word string, values []*mango.GoCollItem)) {
	for k, v := range mvc.Values {
		fn(k, v)
	}
}

func (mvc *MultivalueColls) evaluateScores(scores []float64) CollEstim {
	var mn float64
	var stdev float64
	for _, v := range scores {
		mn += v
	}
	mn /= float64(len(scores))
	for _, v := range scores {
		stdev += (v - mn) * (v - mn)
	}
	stdev = math.Sqrt(stdev / float64(len(scores)))
	return CollEstim{
		Mean:  mn,
		Left:  mn - 1.96*(stdev/math.Sqrt(float64(len(scores)))),
		Right: mn + 1.96*(stdev/math.Sqrt(float64(len(scores)))),
	}
}

func (mvc *MultivalueColls) bootstrapWord(w string, sampleLen int) (float64, error) {
	items, ok := mvc.Values[w]
	if !ok {
		return 0, fmt.Errorf("word not found: %s", w)
	}
	var mean float64
	for i := 0; i < sampleLen; i++ {
		xi := rand.Intn(len(items))
		mean += items[xi].Score
	}
	return mean / float64(sampleLen), nil
}

func (mvc *MultivalueColls) SortedByBootstrappedScore(sampleSize, numSamples int) ([]*CollEstim, error) {
	ans := make([]*CollEstim, 0, len(mvc.Values))
	for word, items := range mvc.Values {
		tmp := make([]float64, 0, sampleSize) // TODO parallelize
		for i := 0; i < numSamples; i++ {
			mn, err := mvc.bootstrapWord(word, sampleSize)
			if err != nil {
				return []*CollEstim{}, err
			}
			tmp = append(tmp, mn)
		}
		estim := mvc.evaluateScores(tmp)
		estim.Word = word
		estim.NumChunks = len(items)
		ans = append(ans, &estim)
	}

	sort.SliceStable(
		ans,
		func(i, j int) bool {
			return ans[i].Mean > ans[j].Mean
		},
	)
	return ans, nil
}

func (mvc *MultivalueColls) SortedByAvgScore() []*mango.GoCollItem {
	ans := make([]*mango.GoCollItem, len(mvc.Values))
	var i int
	for _, vals := range mvc.Values {
		var avg float64
		for _, x := range vals {
			avg += x.Score
		}
		avg /= float64(len(vals))
		ans[i] = &mango.GoCollItem{
			Word:  vals[0].Word,
			Freq:  0, // TODO
			Score: avg,
		}
		i++
	}
	sort.SliceStable(
		ans,
		func(i, j int) bool {
			return ans[i].Score > ans[j].Score
		},
	)
	return ans
}

// TODO bootstrap
// 95%:
// ci = mean +/- (1.96 * (stdev/ sqrt(n))), where n = num of bootstrap samples
