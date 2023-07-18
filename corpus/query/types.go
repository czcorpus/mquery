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

type FreqDistribItem struct {
	Word string  `json:"word"`
	Freq int64   `json:"freq"`
	Norm int64   `json:"norm"`
	IPM  float32 `json:"ipm"`
}

type WordFormsItem struct {
	Lemma string             `json:"lemma"`
	POS   string             `json:"pos"`
	Forms []*FreqDistribItem `json:"forms"`
}
