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
	"mquery/rdb"
	"net/http"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	CollDefaultAttr        = "lemma"
	defaultNumSubcSamples  = 30
	defaultSrchLeft        = -5
	defaultSrchRight       = 5
	defaultMinCollFreq     = 3
	defaultCollocationFunc = "logDice"
	defaultCollMaxItems    = 20
)

func (a *Actions) Collocations(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}

	measure := ctx.Request.URL.Query().Get("measure")
	if measure == "" {
		measure = defaultCollocationFunc
	}

	srchLeft, ok := unireq.GetURLIntArgOrFail(ctx, "srchLeft", defaultSrchLeft)
	if !ok {
		return
	}
	srchRight, ok := unireq.GetURLIntArgOrFail(ctx, "srchRight", defaultSrchRight)
	if !ok {
		return
	}
	minCollFreq, ok := unireq.GetURLIntArgOrFail(ctx, "minCollFreq", defaultMinCollFreq)
	if !ok {
		return
	}
	maxItems, ok := unireq.GetURLIntArgOrFail(ctx, "maxItems", defaultCollMaxItems)
	if !ok {
		return
	}

	corpusPath := a.conf.GetRegistryPath(queryProps.corpus)

	args, err := json.Marshal(rdb.CollocationsArgs{
		CorpusPath: corpusPath,
		Query:      queryProps.query,
		Attr:       CollDefaultAttr,
		Measure:    measure,
		SrchRange:  [2]int{srchLeft, srchRight},
		MinFreq:    int64(minCollFreq),
		MaxItems:   maxItems,
	})
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "collocations",
		Args: args,
	})
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	rawResult := <-wait
	result, err := rdb.DeserializeCollocationsResult(rawResult)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	if err := result.Err(); err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		&result,
	)
}
