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
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

// TokenContext godoc
// @Summary      TokenContext
// @Description  This endpoint provides a text window around a specified token number indexed by token position within a corresponding corpus.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        idx query int true "A token number"
// @Param        leftCtx query int false "Left window size in tokens. Default value is corpus-dependent, but typically around 25"
// @Param        rightCtx query int false "Right window size in tokens. Default value is corpus-dependent, but typically around 25"
// @Param        attr query string false "Positional attributes to be returned, multiple values are supported. Default is corpus-dependend, but typically: word, lemma, tag"
// @Param 		 struct query string false "Structure (or structure with an attribute) to be returned. E.g. 'p', 'p.id', 'doc.pubyear'"
// @Success      200 {object} results.TokenContext
// @Router       /token-context/{corpusId} [get]
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
		rawStrct := strings.Split(strct, ".")[0] // we allow structattrs (e.g. p.id, but check just structures)
		if !collections.SliceContains(knownStructs, rawStrct) {
			uniresp.RespondWithErrorJSON(
				ctx, fmt.Errorf("structure %s not found", rawStrct), http.StatusBadRequest,
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
