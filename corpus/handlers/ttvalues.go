// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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
	"mquery/corpus"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

// ----

// TextTypesAvailValues godoc
// @Summary      TextTypesAvailValues
// @Description  Shows all the possible values for all the structural attributes of a corpus.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Success      200 {object} ttOverviewResponse
// @Router       /text-types-avail-values/{corpusId} [get]
func (a *Actions) TextTypesAvailValues(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	corpusConf := a.conf.GetCorp(corpusID)
	if corpusConf == nil {
		uniresp.RespondWithErrorJSON(ctx, corpus.ErrNotFound, http.StatusNotFound)
		return
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)

	wait, err := a.radapter.PublishQuery(
		rdb.Query{
			Func: "textTypesAvailValues",
			Args: rdb.TextTypesAvailValuesArgs{
				CorpusPath:       corpusPath,
				MaxValueListSize: 100,
				SkipTruncated:    true,
			},
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
	result, ok := TypedOrRespondError[results.TextTypesAvailValues](ctx, rawResult)
	if !ok {
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, result)
}
