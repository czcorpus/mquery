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

import (
	"encoding/json"
	"math/rand"
	"mquery/results"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type StreamData struct {
	Entries results.FreqDistrib `json:"entries"`
	Count   int                 `json:"count"`
	Total   int                 `json:"total"`
}

// mockFreqCalculation is a testing replacement for (not yet available)
// chunked text types freq. calculation
func mockFreqCalculation() chan StreamData {
	messageChannel := make(chan StreamData, 10)
	go func() {
		counter := 0
		values := []string{
			"1990", "1991", "1992", "1993", "1994", "1995", "1996", "1997", "1998",
			"1999", "2000", "2001", "2002", "2003", "2004", "2005", "2006", "2007",
			"2008", "2009", "2010", "2011", "2012", "2013", "2014", "2015", "2016",
			"2017", "2018", "2019", "2020"}

		randItem := func() *results.FreqDistrib {
			ans := &results.FreqDistrib{
				CorpusSize: 120000000,
				Freqs:      make([]*results.FreqDistribItem, 0, 10),
			}
			for i := 0; i < 1+rand.Intn(5); i++ {
				ans.Freqs = append(ans.Freqs, &results.FreqDistribItem{
					Word: values[rand.Intn(len(values))],
					Freq: int64(10 + rand.Intn(10000)),
				})
				for _, v := range ans.Freqs {
					ans.ConcSize += v.Freq
				}
			}
			return ans
		}

		ans := results.FreqDistrib{}
		for {
			counter++
			newFreq := randItem()
			ans.MergeWith(newFreq)
			messageChannel <- StreamData{ans, counter, 30}
			time.Sleep(1 * time.Second)
			if counter >= 30 {
				close(messageChannel)
				break
			}
		}
	}()
	return messageChannel
}

type streamingError struct {
	Error string `json:"error"`
}

func (a *Actions) TextTypesChunked(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	defer ctx.Writer.Flush()

	calc := mockFreqCalculation()
	for message := range calc {
		messageJSON, err := json.Marshal(message)
		if err == nil {
			ctx.String(http.StatusOK, "data: %s\n\n", messageJSON)

		} else {
			// We use status 200 here deliberately as we don't want to trigger
			// the error handler.
			messageJSON, err2 := json.Marshal(streamingError{err.Error()})
			if err2 != nil {
				ctx.String(http.StatusInternalServerError, "Internal Server Error")
				return
			}
			ctx.String(http.StatusOK, string(messageJSON))
			return
		}
		ctx.Writer.Flush()
	}
}
