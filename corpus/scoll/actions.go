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

package scoll

import (
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	corpConf   *corpus.CorporaSetup
	sketchConf *SketchSetup
	radapter   *rdb.Adapter
	qExecutor  *QueryExecutor
}

func (a *Actions) initSkechAttrsOrWriteErr(ctx *gin.Context, corpusID string) *CorpusSketchSetup {
	sketchAttrs, ok := a.sketchConf.SketchAttrs[corpusID]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("Missing sketch conf for requested corpus"),
			http.StatusInternalServerError,
		)
		return nil
	}
	return sketchAttrs
}

func (a *Actions) handleResultOrWriteErr(
	ctx *gin.Context,
	res results.SerializableResult,
	deserializeErr error,
) bool {
	if deserializeErr != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(deserializeErr),
			http.StatusInternalServerError,
		)
		return true
	}
	if res.Err() != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(res.Err()),
			http.StatusInternalServerError,
		)
		return true
	}
	return false
}

func (a *Actions) NounsModifiedBy(ctx *gin.Context) {
	w := Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	sketchAttrs := a.initSkechAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := NewQueryGenerator(QueryNounsModifiedBy, sketchAttrs)
	corpusPath := a.corpConf.GetRegistryPath(corpusID)
	wait, err := a.qExecutor.FxQuery(queryGen, corpusPath, w)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	rawResult := <-wait
	result, err := rdb.DeserializeFreqDistribResult(rawResult)
	if failed := a.handleResultOrWriteErr(ctx, &result, err); failed {
		return
	}

	rc := a.qExecutor.NewReorderCalculator(
		a.corpConf,
		corpusPath,
		queryGen,
	)
	ans, err := rc.SortByLogDiceColl(w, result.Freqs, a.sketchConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	result.Freqs = ans
	result.ExamplesQueryTpl = queryGen.FxyQuery(w, "%s")

	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func (a *Actions) ModifiersOf(ctx *gin.Context) {
	w := Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	sketchAttrs := a.initSkechAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := NewQueryGenerator(QueryModifiersOf, sketchAttrs)
	corpusPath := a.corpConf.GetRegistryPath(corpusID)
	wait, err := a.qExecutor.FxQuery(queryGen, corpusPath, w)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	rawResult := <-wait
	result, err := rdb.DeserializeFreqDistribResult(rawResult)
	if failed := a.handleResultOrWriteErr(ctx, &result, err); failed {
		return
	}
	rc := a.qExecutor.NewReorderCalculator(
		a.corpConf,
		corpusPath,
		queryGen,
	)
	ans, err := rc.SortByLogDiceColl(w, result.Freqs, a.sketchConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	result.Freqs = ans
	result.ExamplesQueryTpl = queryGen.FxyQuery(w, "%s")
	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func (a *Actions) VerbsSubject(ctx *gin.Context) {
	w := Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	sketchAttrs := a.initSkechAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := NewQueryGenerator(QueryVerbsSubject, sketchAttrs)
	corpusPath := a.corpConf.GetRegistryPath(corpusID)
	wait, err := a.qExecutor.FxQuery(queryGen, corpusPath, w)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	rawResult := <-wait
	result, err := rdb.DeserializeFreqDistribResult(rawResult)
	if failed := a.handleResultOrWriteErr(ctx, &result, err); failed {
		return
	}

	rc := a.qExecutor.NewReorderCalculator(
		a.corpConf,
		corpusPath,
		queryGen,
	)
	ans, err := rc.SortByLogDiceColl(w, result.Freqs, a.sketchConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	result.Freqs = ans
	result.ExamplesQueryTpl = queryGen.FxyQuery(w, "%s")
	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func (a *Actions) VerbsObject(ctx *gin.Context) {
	w := Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	sketchAttrs := a.initSkechAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := NewQueryGenerator(QueryVerbsObject, sketchAttrs)
	corpusPath := a.corpConf.GetRegistryPath(corpusID)
	wait, err := a.qExecutor.FxQuery(queryGen, corpusPath, w)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	rawResult := <-wait
	result, err := rdb.DeserializeFreqDistribResult(rawResult)
	if failed := a.handleResultOrWriteErr(ctx, &result, err); failed {
		return
	}

	rc := a.qExecutor.NewReorderCalculator(
		a.corpConf,
		corpusPath,
		queryGen,
	)
	ans, err := rc.SortByLogDiceColl(w, result.Freqs, a.sketchConf)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	result.Freqs = ans
	result.ExamplesQueryTpl = queryGen.FxyQuery(w, "%s")
	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func NewActions(
	corpConf *corpus.CorporaSetup,
	sketchConf *SketchSetup,
	radapter *rdb.Adapter,
	qExecutor *QueryExecutor,
) *Actions {
	ans := &Actions{
		corpConf:   corpConf,
		sketchConf: sketchConf,
		radapter:   radapter,
		qExecutor:  qExecutor,
	}
	return ans
}