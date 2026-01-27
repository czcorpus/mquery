// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
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
	"strconv"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	textTypesInternalMaxResults = 20
)

// TextTypes godoc
// @Summary      TextTypes
// @Description  Calculates frequencies of all the values of a requested structural attribute found in structures matching required query (e.g. all the authors found in &lt;doc author=\"...\"&gt;)
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "The translated query"
// @Param        subcorpus query string false "An ID of a subcorpus"
// @Param        attr query string false "a structural attribute the frequencies will be calculated for (e.g. `doc.pubyear`, `text.author`,...)"
// @Param        maxItems query int 20 "maximum result size"
// @Param        flimit query int 1 "minimum accepted frequency"
// @Success      200 {object} results.FreqDistribResponse
// @Router       /text-types/{corpusId} [get]
func (a *Actions) TextTypes(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}

	attr, ok := a.DecodeTextTypeAttrOrFail(ctx, queryProps.corpus)
	if !ok {
		return
	}
	flimit := DefaultFreqLimit
	if ctx.Request.URL.Query().Has("flimit") {
		var err error
		flimit, err = strconv.Atoi(ctx.Request.URL.Query().Get("flimit"))
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	maxResults, ok := unireq.GetURLIntArgOrFail(ctx, "maxItems", textTypesInternalMaxResults)
	if !ok {
		return
	}

	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	freqArgs := rdb.FreqDistribArgs{
		CorpusPath:  corpusPath,
		SubcPath:    queryProps.savedSubcorpus,
		Query:       queryProps.query,
		Crit:        fmt.Sprintf("%s 0", attr),
		IsTextTypes: true,
		FreqLimit:   flimit,
		MaxItems:    maxResults,
	}
	// TODO this probably needs some work
	if ctx.Request.URL.Query().Has("subc") {
		freqArgs.SubcPath = ctx.Request.URL.Query().Get("subc")
	}

	wait, err := a.radapter.PublishQuery(
		rdb.Query{
			Func: "freqDistrib",
			Args: freqArgs,
		},
		GetCTXStoredTimeout(ctx),
	)
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
	result, ok := TypedOrRespondError[results.FreqDistrib](ctx, rawResult)
	if !ok {
		return
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		&result,
	)
}
