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

package corpus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubcFreqDataPath(t *testing.T) {
	assert.Equal(t, "/a/b/data.lemma.frq", SubcFreqDataPath("/a/b/data.subc", "lemma"))
}

func TestSubcFreqDataExists(t *testing.T) {
	dir := t.TempDir()
	subcPath := filepath.Join(dir, "data.subc")

	exists, err := SubcFreqDataExists(subcPath, "lemma")
	assert.NoError(t, err)
	assert.False(t, exists)

	assert.NoError(t, os.WriteFile(filepath.Join(dir, "data.lemma.frq64"), []byte{}, 0644))
	exists, err = SubcFreqDataExists(subcPath, "lemma")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestCheckSavedSubcorpusInvalidID(t *testing.T) {
	dir := t.TempDir()
	for _, subcID := range []string{"", "a", "../../etc", "ab/cd"} {
		_, ok := CheckSavedSubcorpus(dir, "syn2020", subcID)
		assert.False(t, ok, subcID)
	}
}
