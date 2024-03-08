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
	"mquery/corpus"
	"mquery/rdb"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	dfltMaxContext = 50
)

type ConcArgsBuilder func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs

func (a *Actions) SyntaxConcordance(ctx *gin.Context) {
	a.anyConcordance(
		ctx,
		func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {
			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(conf.ID),
				QueryLemma:        ctx.Query("lemma"),
				Query:             q,
				Attrs:             conf.SyntaxConcordance.ResultAttrs,
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        dfltMaxContext,
				ViewContextStruct: conf.ViewContextStruct,
			}
		},
	)
}

func (a *Actions) Concordance(ctx *gin.Context) {
	a.anyConcordance(
		ctx,
		func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {
			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(conf.ID),
				Query:             q,
				Attrs:             conf.PosAttrs.GetIDs(),
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        dfltMaxContext,
				ViewContextStruct: conf.ViewContextStruct,
			}
		},
	)
}

func (a *Actions) anyConcordance(ctx *gin.Context, argsBuilder ConcArgsBuilder) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}

	args, err := json.Marshal(argsBuilder(
		queryProps.corpusConf,
		queryProps.query,
	))
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
