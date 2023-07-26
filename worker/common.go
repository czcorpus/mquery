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

package worker

import (
	"mquery/mango"
	"mquery/results"
	"sort"
)

func MergeFreqVectors(freqs *mango.Freqs, corpSize int64) []*results.FreqDistribItem {
	ans := make([]*results.FreqDistribItem, len(freqs.Freqs))
	for i, _ := range ans {
		norm := freqs.Norms[i]
		if norm == 0 {
			norm = corpSize
		}
		ans[i] = &results.FreqDistribItem{
			Freq: freqs.Freqs[i],
			Norm: norm,
			IPM:  float32(freqs.Freqs[i]) / float32(norm) * 1e6,
			Word: freqs.Words[i],
		}
	}
	sort.Slice(ans, func(i, j int) bool { return ans[i].Freq > ans[j].Freq })
	return ans
}
