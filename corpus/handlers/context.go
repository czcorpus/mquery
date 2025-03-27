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
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

func (a *Actions) TokenContext(ctx *gin.Context) {
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	pos, ok := unireq.GetURLIntArgOrFail(ctx, "idx", -1)
	if !ok {
		return
	}

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "tokenContext",
		Args: rdb.TokenContextArgs{
			CorpusPath: corpusPath,
			Idx:        int64(pos),
			LeftCtx:    5, // TODO
			RightCtx:   5, // TODO
			Structs:    []string{"doc"},
			Attrs:      []string{"word", "lemma", "tag"},
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
