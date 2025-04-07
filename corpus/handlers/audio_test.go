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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRange(t *testing.T) {
	var act Actions
	hdr := make(http.Header)
	hdr.Add("range", "bytes=13-5743")
	lft, rgt, err := act.getRange(hdr)
	assert.Equal(t, 13, lft)
	assert.Equal(t, 5743, rgt)
	assert.NoError(t, err)
}

func TestGetRangeLeftOnly(t *testing.T) {
	var act Actions
	hdr := make(http.Header)
	hdr.Add("range", "bytes=517-")
	lft, rgt, err := act.getRange(hdr)
	assert.Equal(t, 517, lft)
	assert.Equal(t, -1, rgt)
	assert.NoError(t, err)
}

func TestGetRangeCapitalB(t *testing.T) {
	var act Actions
	hdr := make(http.Header)
	hdr.Add("range", "Bytes=517-1000")
	lft, rgt, err := act.getRange(hdr)
	assert.Equal(t, 517, lft)
	assert.Equal(t, 1000, rgt)
	assert.NoError(t, err)
}

func TestGetRangeInvalidVal(t *testing.T) {
	var act Actions
	hdr := make(http.Header)
	hdr.Add("range", "bytes=517-b")
	lft, rgt, err := act.getRange(hdr)
	assert.Equal(t, -1, lft)
	assert.Equal(t, -1, rgt)
	assert.Error(t, err)
}

func TestGetRangeEmptyValue(t *testing.T) {
	var act Actions
	hdr := make(http.Header)
	hdr.Add("range", "")
	lft, rgt, err := act.getRange(hdr)
	assert.Equal(t, 0, lft)
	assert.Equal(t, -1, rgt)
	assert.NoError(t, err)
}
