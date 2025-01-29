// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
// This file is part of MQUERY.
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

import "fmt"

type NormsCache struct {
	data map[string]map[string]int64
}

func (nc *NormsCache) mkKey(corp, sattr string) string {
	return fmt.Sprintf("%s#%s", corp, sattr)
}

func (nc *NormsCache) Get(corp, sattr string) (map[string]int64, bool) {
	v, ok := nc.data[nc.mkKey(corp, sattr)]
	if !ok {
		return map[string]int64{}, false
	}
	return v, true
}

func (nc *NormsCache) Set(corp, sattr string, values map[string]int64) {
	_, ok := nc.data[nc.mkKey(corp, sattr)]
	if !ok && values == nil {
		nc.data[nc.mkKey(corp, sattr)] = make(map[string]int64)

	} else {
		nc.data[nc.mkKey(corp, sattr)] = values
	}
}

func NewNormsCache() *NormsCache {
	return &NormsCache{
		data: make(map[string]map[string]int64),
	}
}
