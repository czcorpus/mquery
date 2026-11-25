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
	"encoding/binary"
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

func writeSubcFile(t *testing.T, ranges ...[2]int64) string {
	path := filepath.Join(t.TempDir(), "data.subc")
	data := make([]byte, 0, len(ranges)*16)
	for _, r := range ranges {
		data = binary.LittleEndian.AppendUint64(data, uint64(r[0]))
		data = binary.LittleEndian.AppendUint64(data, uint64(r[1]))
	}
	assert.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func TestReadSavedSubcorpusInfo(t *testing.T) {
	path := writeSubcFile(t, [2]int64{0, 10}, [2]int64{20, 25}, [2]int64{100, 101})
	info, err := ReadSavedSubcorpusInfo(path)
	assert.NoError(t, err)
	assert.Equal(t, SavedSubcInfo{Size: 16, NumRanges: 3, MaxPos: 101}, info)
}

func TestReadSavedSubcorpusInfoEmpty(t *testing.T) {
	info, err := ReadSavedSubcorpusInfo(writeSubcFile(t))
	assert.NoError(t, err)
	assert.Equal(t, SavedSubcInfo{}, info)
}

func TestReadSavedSubcorpusInfoInvalid(t *testing.T) {
	_, err := ReadSavedSubcorpusInfo(writeSubcFile(t, [2]int64{10, 5}))
	assert.Error(t, err)

	path := filepath.Join(t.TempDir(), "data.subc")
	assert.NoError(t, os.WriteFile(path, make([]byte, 20), 0644))
	_, err = ReadSavedSubcorpusInfo(path)
	assert.Error(t, err)

	_, err = ReadSavedSubcorpusInfo(filepath.Join(t.TempDir(), "missing.subc"))
	assert.Error(t, err)
}
