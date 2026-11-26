// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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

package rdb

import (
	"hash/fnv"
	"path/filepath"
	"strconv"
)

// savedSubcFile is the name of a saved subcorpus data file
// (<subc. root>/<ID[:2]>/<ID>/data.subc)
const savedSubcFile = "data.subc"

// subcorpusID returns an ID of a saved subcorpus based on its path
// (or the path itself if it does not follow the expected layout)
func subcorpusID(subcPath string) string {
	if filepath.Base(subcPath) == savedSubcFile {
		return filepath.Base(filepath.Dir(subcPath))
	}
	return subcPath
}

// ShuffleSeed returns a seed for repeatable shuffling of the concordance
// lines. It depends only on what determines the lines (corpus, subcorpus,
// query, collocate filter), so all the pages of a concordance come from
// the same order, while different concordances get independent orders.
// Server paths are not used directly (only corpus and subcorpus IDs),
// so the order is the same on all installations.
func (args *ConcordanceArgs) ShuffleSeed() uint64 {
	h := fnv.New64a()
	for _, v := range []string{
		filepath.Base(args.CorpusPath),
		subcorpusID(args.SubcPath),
		args.Query,
		args.CollQuery,
		strconv.Itoa(args.CollLftCtx),
		strconv.Itoa(args.CollRgtCtx),
	} {
		h.Write([]byte(v))
		h.Write([]byte{0}) // separator, so ("ab", "c") != ("a", "bc")
	}
	return h.Sum64()
}
