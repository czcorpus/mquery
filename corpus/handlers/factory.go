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

package handlers

import (
	"mquery/corpus"
	"mquery/corpus/infoload"
	"mquery/rdb"
)

func NewActions(
	conf *corpus.CorporaSetup,
	radapter *rdb.Adapter,
	infoProvider infoload.Provider,
	dfltLanguage string,
) *Actions {
	return &Actions{
		conf:         conf,
		radapter:     radapter,
		infoProvider: infoProvider,
		dfltLanguage: dfltLanguage,
	}
}
