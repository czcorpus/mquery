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
	"fmt"
	"mquery/mango"
	"mquery/results"
	"sort"
	"strings"
)

// CompileFreqResult merges three vectors holding words, freqs and norms
// (as provided by Manatee), sorts the values and returns at most
// maxItems.
// Please note that the function sorts the frequency results in RAM so it
// may be quite demanding based on corpus size and underlying concordance.
func CompileFreqResult(
	freqs *mango.Freqs,
	corpSize int64,
	maxItems int,
	norms map[string]int64,
) ([]*results.FreqDistribItem, error) {
	ans := make([]*results.FreqDistribItem, 0, len(freqs.Freqs))
	isTT := len(norms) > 0
	for i := range freqs.Freqs {
		var norm int64
		if isTT {
			var ok bool
			norm, ok = norms[freqs.Words[i]]
			if !ok {
				return ans, fmt.Errorf("cannot find norm for `%s`", freqs.Words[i])
			}

		} else {
			norm = corpSize
		}
		if norm > 0 {
			ans = append(
				ans,
				&results.FreqDistribItem{
					Freq:  freqs.Freqs[i],
					Base:  norm,
					IPM:   float32(freqs.Freqs[i]) / float32(norm) * 1e6,
					Value: freqs.Words[i],
				},
			)
		}
	}
	lenLimit := len(ans)
	if maxItems < lenLimit {
		lenLimit = maxItems
	}
	sort.Slice(ans, func(i, j int) bool { return ans[i].Freq > ans[j].Freq })
	return ans[:lenLimit], nil
}

func extractAttrFromTTCrit(crit string) string {
	tmp := strings.Split(crit, " ")
	return tmp[0]
}
