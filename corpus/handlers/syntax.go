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
	"mquery/rdb"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	dfltMaxContext = 50
)

func (a *Actions) SyntaxConcordance(ctx *gin.Context) {
	corpusName := ctx.Param("corpusId")
	corpusConf, ok := a.conf.Resources[corpusName]
	if !ok {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("corpus %s not found", corpusName),
			http.StatusNotFound,
		)
	}
	args, err := json.Marshal(rdb.ConcordanceArgs{
		CorpusPath:        a.conf.GetRegistryPath(corpusName),
		QueryLemma:        ctx.Query("lemma"),
		Query:             ctx.Query("q"),
		Attrs:             corpusConf.SyntaxConcordance.ResultAttrs,
		ParentIdxAttr:     corpusConf.SyntaxConcordance.ParentAttr,
		StartLine:         0, // TODO
		MaxItems:          corpusConf.MaximumRecords,
		MaxContext:        dfltMaxContext,
		ViewContextStruct: corpusConf.ViewContextStruct,
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
		Func: "concordance",
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
	result, err := rdb.DeserializeConcordanceResult(rawResult)
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
	uniresp.WriteJSONResponse(ctx.Writer, result)
}
