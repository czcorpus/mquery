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

package results

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func bd(freq int64) binData {
	return binData{freq: freq}
}

const floatTol = 1e-9

// TestMovingZScoresSingleWindow verifies that windowSize == len(data) produces
// exactly one z-score covering the whole dataset.
func TestMovingZScoresSingleWindow(t *testing.T) {
	// data = [2, 4, 6]: mean=4, sample-stdev=2
	// z-score of first element: (2-4)/2 = -1
	data := []binData{bd(2), bd(4), bd(6)}
	result := bdMovingZScores(data, 3)

	assert.Len(t, result, 1)
	assert.InDelta(t, -1.0, result[0], floatTol)
}

// TestMovingZScoresResultLength verifies len(result) == len(data) - windowSize + 1.
func TestMovingZScoresResultLength(t *testing.T) {
	data := []binData{bd(1), bd(2), bd(3), bd(4), bd(5)}

	assert.Len(t, bdMovingZScores(data, 1), 5)
	assert.Len(t, bdMovingZScores(data, 2), 4)
	assert.Len(t, bdMovingZScores(data, 3), 3)
	assert.Len(t, bdMovingZScores(data, 4), 2)
	assert.Len(t, bdMovingZScores(data, 5), 1)
}

// TestMovingZScoresWindowSlides verifies that each window truly advances by one
// position so adjacent windows use different (but overlapping) data.
//
// data = [1, 10, 2, 10, 3], windowSize = 2
//
//	window 0: [1, 10]  → mean=5.5, stdev=sqrt(40.5)  → z = (1-5.5)/sqrt(40.5)
//	window 1: [10, 2]  → mean=6,   stdev=sqrt(32)    → z = (10-6)/sqrt(32)
//	window 2: [2, 10]  → mean=6,   stdev=sqrt(32)    → z = (2-6)/sqrt(32)
//	window 3: [10, 3]  → mean=6.5, stdev=sqrt(24.5)  → z = (10-6.5)/sqrt(24.5)
func TestMovingZScoresWindowSlides(t *testing.T) {
	data := []binData{bd(1), bd(10), bd(2), bd(10), bd(3)}
	result := bdMovingZScores(data, 2)

	assert.Len(t, result, 4)

	expected := []float64{
		(1 - 5.5) / math.Sqrt(40.5),
		(10 - 6) / math.Sqrt(32),
		(2 - 6) / math.Sqrt(32),
		(10 - 6.5) / math.Sqrt(24.5),
	}
	for i, exp := range expected {
		assert.InDelta(t, exp, result[i], floatTol, "mismatch at window %d", i)
	}
}

// TestMovingZScoresWindowSize1 verifies that a window of 1 always produces NaN
// because sample stdev of a single element is undefined (division by zero).
func TestMovingZScoresWindowSize1(t *testing.T) {
	data := []binData{bd(5), bd(10), bd(15)}
	result := bdMovingZScores(data, 1)

	assert.Len(t, result, 3)
	for i, v := range result {
		assert.True(t, math.IsNaN(v), "expected NaN at position %d, got %f", i, v)
	}
}

// TestMovingZScoresWindowTooBigPanics verifies that windowSize > len(data) panics.
func TestMovingZScoresWindowTooBigPanics(t *testing.T) {
	data := []binData{bd(1), bd(2)}
	assert.Panics(t, func() {
		bdMovingZScores(data, 3)
	})
}

// TestMovingZScoresEqualWindowsPanic verifies that windowSize == len(data) does NOT panic.
func TestMovingZScoresEqualWindowsNoPanic(t *testing.T) {
	data := []binData{bd(1), bd(2), bd(3)}
	assert.NotPanics(t, func() {
		bdMovingZScores(data, 3)
	})
}

// TestMovingZScoresSymmetricAroundMean checks that for a linear sequence the
// leading element of each window is always below-mean (negative z-score).
//
// For data = [10, 20, 30, 40, 50] with windowSize=3:
//
//	window 0: [10,20,30] mean=20, z=(10-20)/stdev < 0
//	window 1: [20,30,40] mean=30, z=(20-30)/stdev < 0
//	window 2: [30,40,50] mean=40, z=(30-40)/stdev < 0
func TestMovingZScoresLeadingElementBelowMean(t *testing.T) {
	data := []binData{bd(10), bd(20), bd(30), bd(40), bd(50)}
	result := bdMovingZScores(data, 3)

	assert.Len(t, result, 3)
	for i, v := range result {
		assert.Less(t, v, 0.0, "expected negative z-score at window %d", i)
	}
}
