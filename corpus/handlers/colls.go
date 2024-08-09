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

func (a *Actions) Collocations(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}

	measure := ctx.Request.URL.Query().Get("measure")
	if measure == "" {
		measure = DefaultCollocationFunc
	}

	srchLeft, ok := unireq.GetURLIntArgOrFail(ctx, "srchLeft", DefaultSrchLeft)
	if !ok {
		return
	}
	if srchLeft < 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid srchLeft: %d, value must be greater or equal to 0", srchLeft),
			http.StatusBadRequest,
		)
		return
	}
	srchRight, ok := unireq.GetURLIntArgOrFail(ctx, "srchRight", DefaultSrchRight)
	if !ok {
		return
	}
	if srchRight < 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid srchRight: %d, value must be greater or equal to 0", srchRight),
			http.StatusBadRequest,
		)
		return
	}

	if srchLeft == 0 && srchRight == 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("at least one of srchRight and srchLeft must be greater than 0"),
			http.StatusBadRequest,
		)
		return
	}

	minCollFreq, ok := unireq.GetURLIntArgOrFail(ctx, "minCollFreq", DefaultMinCollFreq)
	if !ok {
		return
	}
	maxItems, ok := unireq.GetURLIntArgOrFail(ctx, "maxItems", DefaultCollMaxItems)
	if !ok {
		return
	}

	corpusPath := a.conf.GetRegistryPath(queryProps.corpus)

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "collocations",
		Args: rdb.CollocationsArgs{
			CorpusPath: corpusPath,
			Query:      queryProps.query,
			Attr:       CollDefaultAttr,
			Measure:    measure,
			// Note: see the range below and note that the left context
			// is published differently (as a positive number) in contrast
			// with the "internals" where a negative number is required
			SrchRange: [2]int{-srchLeft, srchRight},
			MinFreq:   int64(minCollFreq),
			MaxItems:  maxItems,
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
