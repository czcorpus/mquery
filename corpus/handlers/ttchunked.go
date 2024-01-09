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
	"encoding/json"
	"fmt"
	"math/rand"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type StreamData struct {
	Entries results.FreqDistrib `json:"entries"`

	// ChunkNum identifies the chunk. Values starts with 1.
	ChunkNum int `json:"chunkNum"`

	Total int `json:"totalChunks"`

	Error string `json:"error"`
}

type streamedFreqsBaseArgs struct {
	Q        string
	Attr     string
	Fcrit    string
	Flimit   int
	MaxItems int
}

type streamingError struct {
	Error string `json:"error"`
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
				Entries:  ans,
				ChunkNum: counter,
				Total:    30,
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

// filterByYearRange creates a new stream of `StreamData` with freqs not matching
// the provided year range (`fromYear` ... `toYear`) excluded. To leave a year
// limit empty, use 0.
func (a *Actions) filterByYearRange(inStream chan StreamData, fromYear, toYear int) chan StreamData {
	if fromYear == 0 && toYear == 0 {
		return inStream
	}
	ans := make(chan StreamData)
	go func() {
		for item := range inStream {
			item.Entries.Freqs = collections.SliceFilter(
				item.Entries.Freqs,
				func(v *results.FreqDistribItem, i int) bool {
					year, err := strconv.Atoi(v.Word)
					if err != nil {
						return false
					}
					if toYear == 0 {
						return year >= fromYear
					}
					return year >= fromYear && year <= toYear
				},
			)
			ans <- item
		}
		close(ans)
	}()
	return ans
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
			go func(chIdx int, subcx string) {
				defer wg.Done()
				args, err := json.Marshal(rdb.FreqDistribArgs{
					CorpusPath:  corpusPath,
					SubcPath:    subcx,
					Query:       query,
					Crit:        fmt.Sprintf("%s 0", attr),
					IsTextTypes: true,
					FreqLimit:   flimit,
					MaxResults:  maxItems,
				})
				if err != nil {
					messageChannel <- StreamData{
						ChunkNum: chunkIdx + 1,
						Total:    len(sc.Subcorpora),
						Error:    err.Error(),
					}
					return
				}

				wait, err := a.radapter.PublishQuery(rdb.Query{
					Func: "freqDistrib",
					Args: args,
				})
				if err != nil {
					messageChannel <- StreamData{
						ChunkNum: chunkIdx + 1,
						Total:    len(sc.Subcorpora),
						Error:    err.Error(),
					}
					return

				} else {
					tmp := <-wait
					resultNext, err := rdb.DeserializeTextTypesResult(tmp)
					if err != nil {
						messageChannel <- StreamData{
							ChunkNum: chIdx + 1,
							Total:    len(sc.Subcorpora),
							Error:    err.Error(),
						}
						return
					}
					if err := resultNext.Err(); err != nil {
						messageChannel <- StreamData{
							ChunkNum: chIdx + 1,
							Total:    len(sc.Subcorpora),
							Error:    err.Error(),
						}
						return
					}
					mergedFreqLock.Lock()
					result.MergeWith(&resultNext)
					mergedFreqLock.Unlock()
					messageChannel <- StreamData{
						Entries:  *result,
						ChunkNum: chIdx + 1,
						Total:    len(sc.Subcorpora),
						Error:    resultNext.Error,
					}
				}
			}(chunkIdx, subc)
		}
		wg.Wait()
		close(messageChannel)
	}()

	return messageChannel, nil
}

func (a *Actions) writeStreamingError(ctx *gin.Context, err error) {
	messageJSON, err2 := json.Marshal(streamingError{err.Error()})
	if err2 != nil {
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	// We use status 200 here deliberately as we don't want to trigger
	// the error handler.
	ctx.String(http.StatusOK, fmt.Sprintf("data: %s\n\n", messageJSON))
}

// ttStreamedBase performs common actions for both
// general streamed text types and "by year" freqs (which is
// in fact also based on text types)
// In case of an error, the function writes proper error response
// and returns false so the caller knows it should not continue
// with execution of its additional actions.
func (a *Actions) ttStreamedBase(ctx *gin.Context) (streamedFreqsBaseArgs, bool) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	var args streamedFreqsBaseArgs

	args.Q = ctx.Request.URL.Query().Get("q")
	args.Attr = ctx.Request.URL.Query().Get("attr")
	args.Fcrit = ctx.Request.URL.Query().Get("fcrit")
	if args.Attr != "" && args.Fcrit != "" {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("parameters `attr` and `fcrit` cannot be used at the same time"),
			http.StatusBadRequest,
		)
		return args, false
	}
	if args.Fcrit != "" {
		tmp := strings.Split(args.Fcrit, " ")
		if len(tmp) != 2 {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionError("invalid `fcrit` value"),
				http.StatusUnprocessableEntity,
			)
			return args, false
		}
		if tmp[1] != "0" {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionError("only kwic position is supported (`attr 0`)"),
				http.StatusUnprocessableEntity,
			)
			return args, false
		}
		args.Attr = tmp[0]
	}
	var ok bool
	args.Flimit, ok = unireq.GetURLIntArgOrFail(ctx, "flimit", 1)
	if !ok {
		return args, false
	}
	args.MaxItems, ok = unireq.GetURLIntArgOrFail(ctx, "maxItems", 0)
	if !ok {
		return args, false
	}
	return args, true
}

// TextTypesStreamed provides parallel calculation
// of text types frequencies with an output based
// on "server-sent events".
// The endpoint allows either `attr` or `fcrit`
// arguments in URL but in case of '
func (a *Actions) TextTypesStreamed(ctx *gin.Context) {
	defer ctx.Writer.Flush()

	args, ok := a.ttStreamedBase(ctx)
	if !ok {
		return
	}

	calc, err := a.streamCalc(args.Q, args.Attr, ctx.Param("corpusId"), args.Flimit, args.MaxItems)
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

func (a *Actions) FreqsByYears(ctx *gin.Context) {
	defer ctx.Writer.Flush()

	args, ok := a.ttStreamedBase(ctx)
	if !ok {
		return
	}

	fromYear, ok := unireq.GetURLIntArgOrFail(ctx, "fromYear", 0)
	if !ok {
		return
	}
	toYear, ok := unireq.GetURLIntArgOrFail(ctx, "toYear", 0)
	if !ok {
		return
	}

	calc, err := a.streamCalc(args.Q, args.Attr, ctx.Param("corpusId"), args.Flimit, args.MaxItems)
	if err != nil {
		a.writeStreamingError(ctx, err)
		return
	}
	calc = a.filterByYearRange(calc, fromYear, toYear)

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
