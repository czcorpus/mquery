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
	"mquery/mango"
	"sort"

	"github.com/czcorpus/cnc-gokit/maths"
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

func (mvc *MultivalueColls) calcCI(value *mango.GoCollItem, numSamples int) {
	lft, rgt, err := maths.TDistribConfInterval(
		value.Score, value.Stdev, numSamples, maths.Significance_0_05)
	if err == maths.ErrValueNotAvailable {
		return

	} else if err != nil {
		panic(err.Error())
	}
	value.ScoreLCI = lft
	value.ScoreRCI = rgt
}

func (mvc *MultivalueColls) SortedByAvgScore() []*mango.GoCollItem {
	ans := make([]*mango.GoCollItem, 0, len(mvc.Values))
	for _, vals := range mvc.Values {
		var mn maths.OnlineMean
		for _, v := range vals {
			mn = mn.Add(v.Score)
		}
		tmp := &mango.GoCollItem{
			Word:  vals[0].Word,
			Freq:  0, // TODO
			Score: mn.Mean(),
			Stdev: mn.Stdev(),
		}
		mvc.calcCI(tmp, len(vals))
		if tmp.ScoreLCI == 0 || tmp.ScoreRCI == 0 {
			continue
		}
		ans = append(ans, tmp)
	}
	sort.SliceStable(
		ans,
		func(i, j int) bool {
			return ans[i].Score > ans[j].Score
		},
	)
	return ans
}
