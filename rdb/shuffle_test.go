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
	"testing"

	"github.com/stretchr/testify/assert"
)

func concArgs() ConcordanceArgs {
	return ConcordanceArgs{
		CorpusPath:        "/var/lib/manatee/registry/syn2020",
		SubcPath:          "/var/lib/subc/JG/JGa4AcIg/data.subc",
		Query:             `[lemma="pes"]`,
		Attrs:             []string{"word", "lemma"},
		RowsOffset:        0,
		MaxItems:          50,
		MaxContext:        20,
		ViewContextStruct: "s",
	}
}

func TestShuffleSeedSameConcordance(t *testing.T) {
	a := concArgs()
	b := concArgs()
	// paging and display options do not change the lines
	b.RowsOffset = 100
	b.MaxItems = 20
	b.MaxContext = 10
	b.Attrs = []string{"word"}
	b.ShowStructs = []string{"s", "p"}
	b.ViewContextStruct = ""
	assert.Equal(t, a.ShuffleSeed(), b.ShuffleSeed())

	// installations may differ in paths
	b.CorpusPath = "/opt/registry/syn2020"
	b.SubcPath = "/data/subcorpora/JG/JGa4AcIg/data.subc"
	assert.Equal(t, a.ShuffleSeed(), b.ShuffleSeed())
}

func TestShuffleSeedDifferentConcordances(t *testing.T) {
	base := concArgs()
	variants := []func(*ConcordanceArgs){
		func(a *ConcordanceArgs) { a.CorpusPath = "/var/lib/manatee/registry/syn2015" },
		func(a *ConcordanceArgs) { a.SubcPath = "" },
		func(a *ConcordanceArgs) { a.SubcPath = "/var/lib/subc/LC/LcwOIcoy/data.subc" },
		func(a *ConcordanceArgs) { a.Query = `[lemma="kočka"]` },
		func(a *ConcordanceArgs) { a.CollQuery = `[lemma="velký"]` },
		func(a *ConcordanceArgs) { a.CollQuery = `[lemma="velký"]`; a.CollLftCtx = -3 },
	}
	seen := map[uint64]bool{base.ShuffleSeed(): true}
	for i, change := range variants {
		a := concArgs()
		change(&a)
		seed := a.ShuffleSeed()
		assert.False(t, seen[seed], "variant %d", i)
		seen[seed] = true
	}
}

func TestShuffleSeedFieldBoundaries(t *testing.T) {
	a := concArgs()
	a.Query, a.CollQuery = "ab", "c"
	b := concArgs()
	b.Query, b.CollQuery = "a", "bc"
	assert.NotEqual(t, a.ShuffleSeed(), b.ShuffleSeed())
}

func TestSubcorpusID(t *testing.T) {
	assert.Equal(t, "JGa4AcIg", subcorpusID("/var/lib/subc/JG/JGa4AcIg/data.subc"))
	assert.Equal(t, "", subcorpusID(""))
	assert.Equal(t, "/x/foo.subc", subcorpusID("/x/foo.subc"))
}
