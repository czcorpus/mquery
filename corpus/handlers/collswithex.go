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

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

func (a *Actions) CollocationsWithExamples(ctx *gin.Context) {
	collArgs, ok := a.fetchCollActionArgs(ctx)
	if !ok {
		return
	}

	corpusPath := a.conf.GetRegistryPath(collArgs.queryProps.corpus)

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "collocations",
		Args: rdb.CollocationsArgs{
			CorpusPath: corpusPath,
			Query:      collArgs.queryProps.query,
			Attr:       CollDefaultAttr,
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
	if ok := HandleWorkerError(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondError[results.Collocations](ctx, rawResult)
	if !ok {
		return
	}
	result.SrchRange[0] = -1 * result.SrchRange[0] // note: HTTP and internal API are different

	for _, coll := range result.Colls {
		fmt.Println("COLL: ", coll)
		// TODO
	}

	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{})
}
