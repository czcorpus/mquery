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
	"fmt"
	"math"
)

type binData struct {
	dateBucket int
	zScore     float64
	freq       int64
	base       int64
	label      string
	numGrouped int
}

func (bd *binData) String() string {
	return fmt.Sprintf("binData{freq: %d, label: %s}", bd.freq, bd.label)
}

func bdStdev(data []binData) float64 {
	var total, avg, stdev float64
	for _, item := range data {
		total += float64(item.freq)
	}
	avg = total / float64(len(data))
	for _, item := range data {
		stdev += (float64(item.freq) - avg) * (float64(item.freq) - avg) / float64(len(data)-1)
	}
	return math.Sqrt(stdev)
}

func bdMean(data []binData) float64 {
	var ans float64
	for _, item := range data {
		ans += float64(item.freq)
	}
	return ans / float64(len(data))
}

func bdMovingZScores(data []binData, windowSize int) []float64 {
	if windowSize > len(data) {
		panic("movingZScores - window size is bigger than data length")
	}
	ans := make([]float64, len(data)-windowSize+1)
	for i := range ans {
		window := data[i : i+windowSize]
		stdev := bdStdev(window)
		mean := bdMean(window)
		ans[i] = (float64(data[i].freq) - mean) / stdev
	}
	return ans
}

func bdNumZeroFreq(data []binData) int {
	var ans int
	for _, v := range data {
		if v.freq == 0 {
			ans++
		}
	}
	return ans
}
