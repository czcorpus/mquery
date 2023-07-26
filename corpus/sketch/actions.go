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

package sketch

import (
	"fmt"
	"mquery/corpus"
	"mquery/mango"
	"mquery/rdb"
	"mquery/worker"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	corpConf   *corpus.CorporaSetup
	sketchConf *SketchSetup
	radapter   *rdb.Adapter
}

func (a *Actions) NounsModifiedBy(ctx *gin.Context) {
	w := ctx.Request.URL.Query().Get("w")
	corpusId := ctx.Param("corpusId")
	sketchAttrs, ok := a.sketchConf.SketchAttrs[corpusId]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("Missing sketch conf for requested corpus"),
			http.StatusInternalServerError,
		)
		return
	}
	q := fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		sketchAttrs.LemmaAttr, w,
		sketchAttrs.FuncAttr, sketchAttrs.NounModifiedValue,
		sketchAttrs.ParPosAttr, sketchAttrs.NounValue,
	)
	corpusPath := a.corpConf.GetRegistryPath(corpusId)
	freqs, err := mango.CalcFreqDist(corpusPath, q, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.ParLemmaAttr), 1)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	ans := worker.MergeFreqVectors(freqs, freqs.CorpusSize)
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": freqs.ConcSize,
			"freqs":    ans,
		},
	)
}

func (a *Actions) ModifiersOf(ctx *gin.Context) {
	w := ctx.Request.URL.Query().Get("w")
	corpusId := ctx.Param("corpusId")
	sketchAttrs, ok := a.sketchConf.SketchAttrs[corpusId]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("Missing sketch conf for requested corpus"),
			http.StatusInternalServerError,
		)
		return
	}
	q := fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		sketchAttrs.ParLemmaAttr, w,
		sketchAttrs.FuncAttr, sketchAttrs.NounModifiedValue,
		sketchAttrs.PosAttr, sketchAttrs.NounValue,
	)
	corpusPath := a.corpConf.GetRegistryPath(corpusId)
	freqs, err := mango.CalcFreqDist(corpusPath, q, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.LemmaAttr), 1)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	ans := worker.MergeFreqVectors(freqs, freqs.CorpusSize)
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": freqs.ConcSize,
			"freqs":    ans,
		},
	)
}

func (a *Actions) VerbsSubject(ctx *gin.Context) {
	w := ctx.Request.URL.Query().Get("w")
	corpusId := ctx.Param("corpusId")
	sketchAttrs, ok := a.sketchConf.SketchAttrs[corpusId]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("Missing sketch conf for requested corpus"),
			http.StatusInternalServerError,
		)
		return
	}
	q := fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		sketchAttrs.LemmaAttr, w,
		sketchAttrs.FuncAttr, sketchAttrs.NounSubjectValue,
		sketchAttrs.ParPosAttr, sketchAttrs.VerbValue,
	)
	corpusPath := a.corpConf.GetRegistryPath(corpusId)
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "freqDistrib",
		Args: []any{corpusPath, q, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.ParLemmaAttr), 1},
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
	result, err := rdb.DeserializeFreqDistribResult(rawResult)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

	rc := NewReorderCalculator(
		a.corpConf,
		corpusPath,
		sketchAttrs,
		a.radapter,
	)
	ans, err := rc.SortByLogDiceColl(w, result.Freqs)

	uniresp.WriteJSONResponse(
		ctx.Writer,
		ans,
	)
}

func (a *Actions) VerbsObject(ctx *gin.Context) {
	w := ctx.Request.URL.Query().Get("w")
	corpusId := ctx.Param("corpusId")
	sketchAttrs, ok := a.sketchConf.SketchAttrs[corpusId]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("Missing sketch conf for requested corpus"),
			http.StatusInternalServerError,
		)
		return
	}
	q := fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		sketchAttrs.LemmaAttr, w,
		sketchAttrs.FuncAttr, sketchAttrs.NounObjectValue,
		sketchAttrs.ParPosAttr, sketchAttrs.NounValue,
	)

	corpusPath := a.corpConf.GetRegistryPath(corpusId)
	freqs, err := mango.CalcFreqDist(corpusPath, q, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.ParLemmaAttr), 1)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	ans := worker.MergeFreqVectors(freqs, freqs.CorpusSize)
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": freqs.ConcSize,
			"freqs":    ans,
		},
	)
}

func NewActions(corpConf *corpus.CorporaSetup, sketchConf *SketchSetup, radapter *rdb.Adapter) *Actions {
	ans := &Actions{
		corpConf:   corpConf,
		sketchConf: sketchConf,
		radapter:   radapter,
	}
	return ans
}
