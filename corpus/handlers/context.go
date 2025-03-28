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
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

func (a *Actions) TokenContext(ctx *gin.Context) {
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	pos, ok := unireq.RequireURLIntArgOrFail(ctx, "idx")
	if !ok {
		return
	}
	corpConf := a.conf.Resources.Get(ctx.Param("corpusId"))
	if corpConf == nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("corpus not found"), http.StatusNotFound,
		)
		return
	}
	maxLft, maxRgt := corpConf.MaximumTokenContextWindow.LeftAndRight()
	leftCtx, ok := unireq.GetURLIntArgOrFail(ctx, "leftCtx", maxLft)
	if !ok {
		return
	}
	rightCtx, ok := unireq.GetURLIntArgOrFail(ctx, "rightCtx", maxRgt)
	if !ok {
		return
	}

	attrs := ctx.Request.URL.Query()["attr"]
	for _, attr := range attrs {
		if !corpConf.PosAttrs.Contains(attr) {
			uniresp.RespondWithErrorJSON(
				ctx, fmt.Errorf("attribute %s not found", attr), http.StatusBadRequest,
			)
			return
		}
	}
	if len(attrs) == 0 {
		attrs = append(attrs, "word", "lemma") // TODO
	}

	structs := ctx.Request.URL.Query()["struct"]
	knownStructs := corpConf.KnownStructures()
	for _, strct := range structs {
		if !collections.SliceContains(knownStructs, strct) {
			uniresp.RespondWithErrorJSON(
				ctx, fmt.Errorf("structure %s not found", strct), http.StatusBadRequest,
			)
			return
		}
	}

	corpConf.PosAttrs.GetIDs()

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "tokenContext",
		Args: rdb.TokenContextArgs{
			CorpusPath: corpusPath,
			Idx:        int64(pos),
			LeftCtx:    leftCtx,
			RightCtx:   rightCtx,
			Structs:    structs,
			Attrs:      attrs,
		},
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
	if ok := HandleWorkerError(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondError[results.TokenContext](ctx, rawResult)
	if !ok {
		return
	}

	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, result)

}
