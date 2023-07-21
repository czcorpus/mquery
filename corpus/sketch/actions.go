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
	"mquery/corpus/query"
	"mquery/mango"
	"net/http"
	"sort"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	corpConf   *corpus.CorporaSetup
	sketchConf *SketchSetup
}

func (a *Actions) getConcordance(corpusId, query string) (*mango.GoConc, error) {
	corp, err := corpus.OpenCorpus(corpusId, a.corpConf)
	if err != nil {
		return nil, err
	}
	conc, err := mango.CreateConcordance(corp, query)
	if err != nil {
		return nil, err
	}
	return conc, nil
}

func (a *Actions) processFrequencies(freqs *mango.Freqs, corpSize int64) []*query.FreqDistribItem {
	ans := make([]*query.FreqDistribItem, len(freqs.Freqs))
	for i, _ := range ans {
		norm := freqs.Norms[i]
		if norm == 0 {
			norm = corpSize
		}
		ans[i] = &query.FreqDistribItem{
			Freq: freqs.Freqs[i],
			Norm: norm,
			IPM:  float32(freqs.Freqs[i]) / float32(norm) * 1e6,
			Word: freqs.Words[i],
		}
	}
	sort.Slice(ans, func(i, j int) bool { return ans[i].Freq > ans[j].Freq })
	return ans
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
	conc, err := a.getConcordance(corpusId, q)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	freqs, err := mango.CalcFreqDist(conc, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.ParLemmaAttr), 1)
	ans := a.processFrequencies(freqs, conc.CorpSize())
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": conc.Size(),
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
	conc, err := a.getConcordance(corpusId, q)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	freqs, err := mango.CalcFreqDist(conc, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.LemmaAttr), 1)
	ans := a.processFrequencies(freqs, conc.CorpSize())
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": conc.Size(),
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
	conc, err := a.getConcordance(corpusId, q)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	freqs, err := mango.CalcFreqDist(conc, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.ParLemmaAttr), 1)
	ans := a.processFrequencies(freqs, conc.CorpSize())
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": conc.Size(),
			"freqs":    ans,
		},
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
	conc, err := a.getConcordance(corpusId, q)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	freqs, err := mango.CalcFreqDist(conc, fmt.Sprintf("%s/e 0~0>0", sketchAttrs.ParLemmaAttr), 1)
	ans := a.processFrequencies(freqs, conc.CorpSize())
	uniresp.WriteJSONResponse(
		ctx.Writer,
		map[string]any{
			"concSize": conc.Size(),
			"freqs":    ans,
		},
	)
}

func NewActions(corpConf *corpus.CorporaSetup, sketchConf *SketchSetup) *Actions {
	return &Actions{
		corpConf:   corpConf,
		sketchConf: sketchConf,
	}
}
