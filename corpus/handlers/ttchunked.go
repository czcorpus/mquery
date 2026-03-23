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
	"mquery/rdb/results"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

var (
	yearMatchRegexp = regexp.MustCompile(`^\d\d\d\d?$`)
)

type StreamData struct {
	Entries results.FreqDistrib `json:"entries"`

	// ChunkNum identifies the chunk. Values starts with 1.
	ChunkNum int `json:"chunkNum"`

	Total int `json:"totalChunks"`

	Error error `json:"error,omitempty"`
}

type streamedFreqsBaseArgs struct {
	Q        string
	Attr     string
	Fcrit    string
	Flimit   int
	MaxItems int

	// Event is an optional argument specifying that
	// a client wants data to be returned with a specific
	// event name (EventSource API). This is mostly used
	// in case the streamed freqs are part of multiple data
	// stream (e.g. in WaG). Otherwise, MQuery will expect
	// the client to have an exclusive event stream opened
	// for the data (it returns just the `data` label).
	Event string
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
func (a *Actions) filterByYearRange(inStream chan StreamData, dateFormat, fromDateStr, toDateStr string, autobin bool) chan StreamData {

	datetimeToInt := func(v string) (int, error) {
		t, err := time.Parse(dateFormat, v)
		if err != nil {
			return 0, fmt.Errorf("failed to parse value as %s: %w", dateFormat, err)
		}
		return int(t.Unix()), nil
	}

	ans := make(chan StreamData)

	var fromDate, toDate time.Time
	var parseErr error
	if fromDateStr != "" {
		fromDate, parseErr = time.Parse(dateFormat, fromDateStr)
		if parseErr != nil {
			go func() {
				for range inStream {
				}
				ans <- StreamData{
					Error: fmt.Errorf("failed to parse fromDateStr %s using template %s", fromDateStr, dateFormat),
				}
				close(ans)
			}()
			return ans
		}
	}
	if toDateStr != "" {
		toDate, parseErr = time.Parse(dateFormat, toDateStr)
		if parseErr != nil {
			go func() {
				for range inStream {
				}
				ans <- StreamData{
					Error: fmt.Errorf("failed to parse toDateStr %s using template %s", toDateStr, dateFormat),
				}
				close(ans)
			}()
			return ans
		}
	}

	go func() {
		for item := range inStream {
			item.Entries.Freqs = collections.SliceFilter(
				item.Entries.Freqs,
				func(v *results.FreqDistribItem, i int) bool {
					docDate, err := time.Parse(dateFormat, v.Word)
					if err != nil {
						log.Error().Str("dateLayout", dateFormat).Str("value", v.Word).Msg("failed to parse supposedly date attribute during filtering, skippping item")
						return false
					}
					if fromDate.IsZero() && toDate.IsZero() {
						return true
					}
					if toDate.IsZero() {
						return docDate.After(fromDate)
					}
					return docDate.After(fromDate) && docDate.Before(toDate)
				},
			)
			if autobin {
				// here is a heuristic evaluation of window size for calculating
				// moving z-score which determines number of data bins.
				// We assume here that typically there are at most hundreds or small thousands
				// of items (e.g. a monitoring corpus with daily updates across at most a few years)
				windowSize := 10
				if len(item.Entries.Freqs) > 1000 {
					windowSize = 100
				} else if len(item.Entries.Freqs) > 300 {
					windowSize = 30

				} else if len(item.Entries.Freqs) > 100 {
					windowSize = 20

				} else if len(item.Entries.Freqs) > 50 {
					windowSize = 15
				}
				log.Debug().Int("windowSize", windowSize).Msg("determining window size for data binning")
				item.Entries.Freqs = item.Entries.Freqs.BinAsDataSeries(datetimeToInt, windowSize)
			}
			ans <- item
		}
		close(ans)
	}()
	return ans
}

func (a *Actions) streamCalc(query, attr, corpusID string, flimit, maxItems int, workerTimeout time.Duration) (chan StreamData, error) {
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
				wait, err := a.radapter.PublishQuery(
					rdb.Query{
						Func: "freqDistrib",
						Args: rdb.FreqDistribArgs{
							CorpusPath:  corpusPath,
							SubcPath:    subcx,
							Query:       query,
							Crit:        fmt.Sprintf("%s 0", attr),
							IsTextTypes: true,
							FreqLimit:   flimit,
							MaxItems:    maxItems,
						},
					},
					workerTimeout,
				)
				if err != nil {
					messageChannel <- StreamData{
						ChunkNum: chunkIdx + 1,
						Total:    len(sc.Subcorpora),
						Error:    err,
					}
					return

				} else {
					tmp := <-wait
					if err := tmp.Value.Err(); err != nil {
						messageChannel <- StreamData{
							ChunkNum: chIdx + 1,
							Total:    len(sc.Subcorpora),
							Error:    err,
						}
						return
					}
					resultNext, ok := tmp.Value.(results.FreqDistrib)
					if !ok {
						messageChannel <- StreamData{
							ChunkNum: chIdx + 1,
							Total:    len(sc.Subcorpora),
							Error:    fmt.Errorf("invalid type for FreqDistrib"),
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
	args.Event = ctx.Request.URL.Query().Get("event")
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

// TextTypesStreamed godoc
// @Summary      TextTypesStreamed
// @Description  provides parallel text types frequencies based across multiple subcorpora
// @Description returned SSE stream produces gradually growing data in results.FreqDistrib
// (i.e. it is not necessary to accumulate the result)
// @Produce json
// @Produce text/event-stream
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "A search query"
// @Param        attr query string false "An attribute used for freq. calculation (mutually exclusive with `fcrit`)"
// @Param        fcrit query string false "A freq. criterium in Manatee-open format (mutually exclusive with `attr`)"
// @Param		 autobin query int 0 "If 1 then data will be grouped into a suitable number of bins for readability"
// @Success      200 {object} results.FreqDistrib
// @Router       /text-types-streamed/{corpusId} [get]
func (a *Actions) TextTypesStreamed(ctx *gin.Context) {
	defer ctx.Writer.Flush()

	args, ok := a.ttStreamedBase(ctx)
	if !ok {
		return
	}

	calc, err := a.streamCalc(args.Q, args.Attr, ctx.Param("corpusId"), args.Flimit, args.MaxItems, GetCTXStoredTimeout(ctx))
	if err != nil {
		WriteStreamingError(ctx, err)
		return
	}
	for message := range calc {
		messageJSON, err := json.Marshal(message)
		if err == nil {
			if args.Event != "" {
				ctx.String(http.StatusOK, "event: %s\ndata: %s\n\n", args.Event, messageJSON)

			} else {
				ctx.String(http.StatusOK, "data: %s\n\n", messageJSON)
			}

		} else {
			WriteStreamingError(ctx, err)
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
	corpusID := ctx.Param("corpusId")
	fromDate := ctx.Query("fromDate")
	toDate := ctx.Query("toDate")

	if fromDate == "" && toDate == "" {
		// deprecated
		fromDate = ctx.Query("fromYear")
		toDate = ctx.Query("toYear")
	}

	autobin := ctx.Query("autobin") == "1"

	cinfo := a.conf.GetCorp(corpusID)
	tprop, ok := cinfo.TextProperties[corp.TextProperty(args.Attr)]
	if !ok {
		for _, attr := range cinfo.TextProperties {
			if attr.Name == args.Attr {
				tprop = attr
				break
			}
		}
	}
	if tprop.DateFormat == "" {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("attribute %s not of a date type", args.Attr),
			http.StatusUnprocessableEntity,
		)
		return
	}

	calc, err := a.streamCalc(args.Q, args.Attr, corpusID, args.Flimit, args.MaxItems, GetCTXStoredTimeout(ctx))
	if err != nil {
		WriteStreamingError(ctx, err)
		return
	}

	calc = a.filterByYearRange(calc, tprop.DateFormat, fromDate, toDate, autobin)

	for message := range calc {
		messageJSON, err := json.Marshal(message)
		if err == nil {
			if args.Event != "" {
				ctx.String(http.StatusOK, "event: %s\ndata: %s\n\n", args.Event, messageJSON)

			} else {
				ctx.String(http.StatusOK, "data: %s\n\n", messageJSON)
			}

		} else {
			WriteStreamingError(ctx, err)
			return
		}
		ctx.Writer.Flush()
	}
}
