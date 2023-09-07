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
	"fmt"
	"math/rand"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"sync"
	"time"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/gin-gonic/gin"
)

type StreamData struct {
	Entries results.FreqDistrib `json:"entries"`
	Count   int                 `json:"count"`
	Total   int                 `json:"total"`
	Error   string              `json:"error"`
}

// mockFreqCalculation is a testing replacement for
// `streamCalc`
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
			messageChannel <- StreamData{
				Entries: ans,
				Count:   counter,
				Total:   30,
			}
			time.Sleep(1 * time.Second)
			if counter >= 30 {
				close(messageChannel)
				break
			}
		}
	}()
	return messageChannel
}

func (a *Actions) streamCalc(query, attr, corpusID string, flimit, maxItems int) (chan StreamData, error) {
	messageChannel := make(chan StreamData, 10)
	corpusPath := a.conf.GetRegistryPath(corpusID)
	sc, err := corpus.OpenSplitCorpus(a.conf.SplitCorporaDir, corpusPath)
	if err != nil {
		close(messageChannel)
		return messageChannel, err
	}

	result := new(results.FreqDistrib)
	result.Freqs = make([]*results.FreqDistribItem, 0)
	mergedFreqLock := sync.Mutex{}

	go func() {
		wg := sync.WaitGroup{}
		wg.Add(len(sc.Subcorpora))

		for chunkIdx, subc := range sc.Subcorpora {
			go func(chIdx int) {
				defer wg.Done()
				args, err := json.Marshal(rdb.FreqDistribArgs{
					CorpusPath: corpusPath,
					SubcPath:   subc,
					Query:      query,
					Crit:       fmt.Sprintf("%s 0", attr),
					FreqLimit:  flimit,
					MaxResults: maxItems,
				})
				if err != nil {
					messageChannel <- StreamData{
						Count: chunkIdx,
						Total: len(sc.Subcorpora),
						Error: err.Error(),
					}
					return
				}

				wait, err := a.radapter.PublishQuery(rdb.Query{
					Func: "freqDistrib",
					Args: args,
				})
				if err != nil {
					messageChannel <- StreamData{
						Count: chunkIdx,
						Total: len(sc.Subcorpora),
						Error: err.Error(),
					}
					return

				} else {
					tmp := <-wait
					resultNext, err := rdb.DeserializeTextTypesResult(tmp)
					if err != nil {
						messageChannel <- StreamData{
							Count: chIdx,
							Total: len(sc.Subcorpora),
							Error: err.Error(),
						}
						return
					}
					mergedFreqLock.Lock()
					result.MergeWith(&resultNext)
					mergedFreqLock.Unlock()
					messageChannel <- StreamData{
						Entries: *result,
						Count:   chIdx,
						Total:   len(sc.Subcorpora),
						Error:   resultNext.Error,
					}
				}
			}(chunkIdx)
		}
		wg.Wait()
		close(messageChannel)
	}()

	return messageChannel, nil
}

type streamingError struct {
	Error string `json:"error"`
}

func (a *Actions) writeStreamingError(ctx *gin.Context, err error) {
	messageJSON, err2 := json.Marshal(streamingError{err.Error()})
	if err2 != nil {
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	// We use status 200 here deliberately as we don't want to trigger
	// the error handler.
	ctx.String(http.StatusOK, string(messageJSON))
}

func (a *Actions) TextTypesStreamed(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	defer ctx.Writer.Flush()

	q := ctx.Request.URL.Query().Get("q")
	attr := ctx.Request.URL.Query().Get("attr")
	flimit, ok := unireq.GetURLIntArgOrFail(ctx, "flimit", 1)
	if !ok {
		return
	}
	maxItems, ok := unireq.GetURLIntArgOrFail(ctx, "maxItems", 0)
	if !ok {
		return
	}

	calc, err := a.streamCalc(q, attr, ctx.Param("corpusId"), flimit, maxItems)
	if err != nil {
		a.writeStreamingError(ctx, err)
		return
	}
	for message := range calc {
		messageJSON, err := json.Marshal(message)
		if err == nil {
			ctx.String(http.StatusOK, "data: %s\n\n", messageJSON)

		} else {
			a.writeStreamingError(ctx, err)
			return
		}
		ctx.Writer.Flush()
	}
}
