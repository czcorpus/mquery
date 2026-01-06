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
	"fmt"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	CollDefaultAttr        = "lemma"
	DefaultSrchLeft        = 5
	DefaultSrchRight       = 5
	DefaultMinCollFreq     = 3
	DefaultCollocationFunc = "logDice"
	DefaultCollMaxItems    = 20
)

type collArgs struct {
	queryProps  queryProps
	measure     string
	srchLeft    int
	srchRight   int
	minCollFreq int
	minCorpFreq int
	maxItems    int
	event       string
}

func (a *Actions) fetchCollActionArgs(ctx *gin.Context) (collArgs, bool) {
	var ans collArgs

	ans.queryProps = DetermineQueryProps(ctx, a.conf)
	if ans.queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, ans.queryProps.err, ans.queryProps.status)
		return ans, false
	}

	ans.measure = ctx.Request.URL.Query().Get("measure")
	if ans.measure == "" {
		ans.measure = DefaultCollocationFunc
	}

	var ok bool
	ans.srchLeft, ok = unireq.GetURLIntArgOrFail(ctx, "srchLeft", DefaultSrchLeft)
	if !ok {
		return ans, false
	}
	if ans.srchLeft < 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid srchLeft: %d, value must be greater or equal to 0", ans.srchLeft),
			http.StatusBadRequest,
		)
		return ans, false
	}
	ans.srchRight, ok = unireq.GetURLIntArgOrFail(ctx, "srchRight", DefaultSrchRight)
	if !ok {
		return ans, false
	}
	if ans.srchRight < 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid srchRight: %d, value must be greater or equal to 0", ans.srchRight),
			http.StatusBadRequest,
		)
		return ans, false
	}

	if ans.srchLeft == 0 && ans.srchRight == 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("at least one of srchRight and srchLeft must be greater than 0"),
			http.StatusBadRequest,
		)
		return ans, false
	}

	ans.minCollFreq, ok = unireq.GetURLIntArgOrFail(ctx, "minCollFreq", DefaultMinCollFreq)
	if !ok {
		return ans, false
	}

	ans.minCorpFreq, ok = unireq.GetURLIntArgOrFail(ctx, "minCorpFreq", ans.minCollFreq)
	if !ok {
		return ans, false
	}

	ans.maxItems, ok = unireq.GetURLIntArgOrFail(ctx, "maxItems", DefaultCollMaxItems)
	if !ok {
		return ans, false
	}

	ans.event = ctx.Query("event")

	return ans, true
}

// Collocations godoc
// @Summary      Collocations
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
// @Success      200 {object} results.CollocationsResponse
// @Router       /collocations/{corpusId} [get]
func (a *Actions) Collocations(ctx *gin.Context) {
	collArgs, ok := a.fetchCollActionArgs(ctx)
	if !ok {
		return
	}

	corpusPath := a.conf.GetRegistryPath(collArgs.queryProps.corpus)

	srchAttr := ctx.Request.URL.Query().Get("srchAttr")
	if srchAttr == "" {
		srchAttr = CollDefaultAttr
	}

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "collocations",
		Args: rdb.CollocationsArgs{
			CorpusPath: corpusPath,
			SubcPath:   collArgs.queryProps.savedSubcorpus,
			Query:      collArgs.queryProps.query,
			Attr:       srchAttr,
			Measure:    collArgs.measure,
			// Note: see the range below and note that the left context
			// is published differently (as a positive number) in contrast
			// with the "internals" where a negative number is required
			SrchRange:   [2]int{-collArgs.srchLeft, collArgs.srchRight},
			MinFreq:     int64(collArgs.minCollFreq),
			MinCorpFreq: int64(collArgs.minCorpFreq),
			MaxItems:    collArgs.maxItems,
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
	if ok := HandleWorkerError(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondError[results.Collocations](ctx, rawResult)
	if !ok {
		return
	}
	result.SrchRange[0] = -1 * result.SrchRange[0] // note: HTTP and internal API are different
	uniresp.WriteJSONResponse(
		ctx.Writer,
		&result,
	)
}
