// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Institute of the Czech National Corpus,
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
	"fmt"
	"mquery/mango"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	defaultExamplesPerColl = 5
)

type extendedConcLine struct {
	concordance.Line
	InteractionID string `json:"interactionId"`
}

type extendedConcLines []extendedConcLine

func (cl extendedConcLines) alwaysAsList() extendedConcLines {
	if cl == nil {
		return []extendedConcLine{}
	}
	return cl
}

type extendedCollItem struct {
	ResultIdx     int
	Word          string
	Score         float64
	Freq          int64
	InteractionID string
	Examples      extendedConcLines
	Err           error
}

func (ecItem extendedCollItem) MarshalJSON() ([]byte, error) {
	return sonic.Marshal(struct {
		Word          string            `json:"word"`
		Score         float64           `json:"score"`
		Freq          int64             `json:"freq"`
		InteractionID string            `json:"interactionId"`
		Examples      extendedConcLines `json:"examples"`
		Err           error             `json:"error,omitempty"`
	}{
		Word:          ecItem.Word,
		Score:         ecItem.Score,
		Freq:          ecItem.Freq,
		InteractionID: ecItem.InteractionID,
		Examples:      ecItem.Examples,
		Err:           ecItem.Err,
	})
}

type endpointResult struct {
	CorpusSize int64               `json:"corpusSize"`
	SubcSize   int64               `json:"subcSize,omitempty"`
	Colls      []*extendedCollItem `json:"colls"`
	ResultType rdb.ResultType      `json:"resultType"`
	Measure    string              `json:"measure"`
	SrchRange  [2]int              `json:"srchRange"`
	Error      string              `json:"error,omitempty"`
}

type wordBindConc struct {
	Lines results.ConcordanceLines
	Word  string
}

// writeStreamedData writes `res` as a server-side event.
// The function also calls flush() on an underlying ctx writer to make
// sure the data is immediately sent.
func writeStreamedData(ctx *gin.Context, collArgs *collArgs, res *endpointResult) {
	messageJSON, err := sonic.Marshal(res)
	if err != nil {
		WriteStreamingError(ctx, err)
		return
	}
	if collArgs.event != "" {
		ctx.String(http.StatusOK, "event: %s\ndata: %s\n\n", collArgs.event, messageJSON)

	} else {
		ctx.String(http.StatusOK, "data: %s\n\n", messageJSON)
	}
	ctx.Writer.Flush()
}

// CollocationsWithExamples godoc
// @Summary      CollocationsWithExamples
// @Description  Calculate a defined collocation profile of a searched expression. Values are sorted in descending order by their collocation score.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "The translated query"
// @Param        subcorpus query string false "An ID of a subcorpus"
// @Param        measure query string false "a collocation measure" enums(absFreq, logLikelihood, logDice, minSensitivity, mutualInfo, mutualInfo3, mutualInfoLogF, relFreq, tScore) default(logDice)
// @Param        srchLeft query int false "left range for candidates searching; values must be greater or equal to 1 (1 stands for words right before the searched term)" default(5)
// @Param        srchRight query int false "right range for candidates searching; values must be greater or equal to 1 (1 stands for words right after the searched term)" default(5)
// @Param        srchAttr query string false "a positional attribute considered when collocations are calculated ()" default(lemma)
// @Param        minCollFreq query int false " the minimum frequency that a collocate must have in the searched range." default(3)
// @Param        maxItems query int false "maximum number of result items" default(20)
// @Param        examplesPerColl query int false "number of concordance lines per collocation" default(5)
// @Param        event query string false "an event id used in response data stream; if omitted then just `data` line are returned"
// @Success      200 {object} results.CollocationsResponse
// @Router       /collocations-with-examples/{corpusId} [get]
func (a *Actions) CollocationsWithExamples(ctx *gin.Context) {
	collArgs, ok := a.fetchCollActionArgs(ctx)
	if !ok {
		return
	}

	defer ctx.Writer.Flush()

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	corpusPath := a.conf.GetRegistryPath(collArgs.queryProps.corpus)

	srchAttr := ctx.Request.URL.Query().Get("srchAttr")
	if srchAttr == "" {
		srchAttr = CollDefaultAttr
	}

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "collocations",
		Args: rdb.CollocationsArgs{
			CorpusPath: corpusPath,
			Query:      collArgs.queryProps.query,
			Attr:       srchAttr,
			Measure:    collArgs.measure,
			// Note: see the range below and note that the left context
			// is published differently (as a positive number) in contrast
			// with the "internals" where a negative number is required
			SrchRange: [2]int{-collArgs.srchLeft, collArgs.srchRight},
			MinFreq:   int64(collArgs.minCollFreq),
			MaxItems:  collArgs.maxItems,
		}})
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	rawResult := <-wait
	if ok := HandleWorkerErrorStreaming(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondErrorStreaming[results.Collocations](ctx, rawResult)
	if !ok {
		return
	}
	result.SrchRange[0] = -1 * result.SrchRange[0] // note: HTTP and internal API are different

	ans := endpointResult{
		CorpusSize: result.CorpusSize,
		Measure:    result.Measure,
		SrchRange:  result.SrchRange,
		ResultType: rdb.ResultTypeCOllocationsWithExamples,
	}
	ans.Colls = make([]*extendedCollItem, len(result.Colls))
	for i, v := range result.Colls {
		ans.Colls[i] = &extendedCollItem{
			Word:     v.Word,
			Score:    v.Score,
			Freq:     v.Freq,
			Examples: extendedConcLines{},
		}
	}
	// let's write colls without actual examples first
	writeStreamedData(ctx, &collArgs, &ans)

	examplesPerColl, ok := GetURLIntArgOrFailStreaming(ctx, "examplesPerColl", defaultExamplesPerColl)
	if !ok {
		return
	}
	corpusConf := a.conf.Resources.Get(collArgs.queryProps.corpus)
	resultsChan := make(chan *extendedCollItem)
	var wg sync.WaitGroup
	wg.Add(len(result.Colls))
	go func() {
		for resultIdx, coll := range result.Colls {
			go func(collItem *mango.GoCollItem) {
				defer wg.Done()
				escapedWord := strings.ReplaceAll(collItem.Word, "\"", "\\\"")
				wait, err := a.radapter.PublishQuery(rdb.Query{
					Func: "concordance",
					Args: rdb.ConcordanceArgs{
						CorpusPath: corpusPath,
						Query:      collArgs.queryProps.query,
						// note - below, we can 'simple text match ==' as the
						// inserted value is always an exact value and not a pattern
						CollQuery:         fmt.Sprintf("[%s==\"%s\"]", srchAttr, escapedWord),
						CollLftCtx:        -collArgs.srchLeft,
						CollRgtCtx:        collArgs.srchRight,
						Attrs:             corpusConf.PosAttrs.GetIDs(),
						ShowStructs:       []string{}, // TODO
						ShowRefs:          []string{},
						MaxItems:          examplesPerColl,
						RowsOffset:        0,
						ViewContextStruct: corpusConf.ViewContextStruct,
					},
				})
				if err != nil {
					resultsChan <- &extendedCollItem{
						ResultIdx: resultIdx,
						Word:      collItem.Word,
						Score:     collItem.Score,
						Freq:      collItem.Freq,
						Err:       result.Error,
					}
					return
				}
				rawResult := <-wait
				if ok := HandleWorkerError(ctx, rawResult); !ok {
					return
				}
				if result, ok := TypedOrRespondError[results.Concordance](ctx, rawResult); ok {
					interactionID := uuid.New().String()
					resultsChan <- &extendedCollItem{
						ResultIdx:     resultIdx,
						Word:          collItem.Word,
						Score:         collItem.Score,
						Freq:          collItem.Freq,
						InteractionID: interactionID,
						Examples: collections.SliceMap(
							result.Lines,
							func(cline concordance.Line, idx int) extendedConcLine {
								return extendedConcLine{
									Line:          cline,
									InteractionID: interactionID,
								}
							},
						),
						Err: result.Error,
					}
				}
				if !ok {
					log.Error().
						Str("type", reflect.TypeOf(rawResult).Name()).
						Str("coll", collItem.Word).
						Msg("CollocationsWithExamples - failed to typecast rawResult")
					return
				}
			}(coll)
		}
		wg.Wait()
		close(resultsChan)
	}()

	for item := range resultsChan {
		ans.Colls[item.ResultIdx] = item
		writeStreamedData(ctx, &collArgs, &ans)
	}
}
