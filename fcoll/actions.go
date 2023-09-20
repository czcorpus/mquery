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

package fcoll

import (
	"database/sql"
	"math"
	"mquery/corpus"
	"mquery/corpus/scoll"
	"mquery/results"
	"net/http"
	"sort"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	fyBatchSize = 100
)

type Actions struct {
	corpConf   *corpus.CorporaSetup
	sketchConf *scoll.SketchSetup
	db         *sql.DB
}

func (a *Actions) initScollAttrsOrWriteErr(ctx *gin.Context, corpusID string) *scoll.CorpusSketchSetup {
	sketchAttrs, ok := a.sketchConf.SketchAttrs[corpusID]
	if !ok {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionError("Missing sketch conf for requested corpus"),
			http.StatusInternalServerError,
		)
		return nil
	}
	return sketchAttrs
}

func (a *Actions) NounsModifiedBy(ctx *gin.Context) {
	w := scoll.Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")

	// [lemma="team" & deprel="nmod" & p_upos="NOUN"]
	cdb := NewCollDatabase(a.db, corpusID)

	fx, err := cdb.GetFreq(w.V, w.PoS, "", "NOUN", "nmod")
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	candidates, err := cdb.GetParentCandidates(w.V, w.PoS, "nmod", candidatesFreqLimit)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	result := make(results.FreqDistribItemList, 0, len(candidates))
	argsBatch := make([]batchFreqArgs, 0, fyBatchSize)

	processBatch := func(ab []batchFreqArgs) bool {
		mapping := make(map[string]*batchFreqArgs)
		for _, a := range ab {
			mapping[a.Lemma] = &a
		}
		fyList, err := cdb.GetFreqBatch(ab)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return false
		}
		for _, fy := range fyList {
			res, ok := mapping[fy.Lemma]
			if !ok {
				log.Warn().Str("value", fy.Lemma).Msg("value mapping ambiguity")
				continue
			}
			result = append(
				result,
				&results.FreqDistribItem{
					Word:       fy.Lemma,
					Freq:       res.fxy,
					CollWeight: 14 + math.Log2(2*float64(res.fxy)/(float64(fx)+float64(fy.Freq))),
				},
			)
		}
		argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		return true
	}

	for _, cand := range candidates {
		argsBatch = append(argsBatch, batchFreqArgs{
			fxy:          cand.Freq,
			isParentSrch: true,
			Lemma:        cand.Lemma,
			Upos:         cand.Upos,
			Deprel:       "nmod",
		})
		if len(argsBatch) == fyBatchSize {
			ok := processBatch(argsBatch)
			if !ok {
				return
			}
			argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		}
	}
	if len(argsBatch) > 0 {
		ok := processBatch(argsBatch)
		if !ok {
			return
		}
	}

	sort.SliceStable(
		result,
		func(i, j int) bool {
			return result[j].CollWeight < result[i].CollWeight
		},
	)
	sketchAttrs := a.initScollAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := scoll.NewQueryGenerator(corpusID, scoll.QueryNounsModifiedBy, sketchAttrs)
	resp := results.FreqDistrib{
		Freqs:            result,
		ExamplesQueryTpl: queryGen.FxyQuery(w, "%s"),
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		resp,
	)
}

func (a *Actions) ModifiersOf(ctx *gin.Context) {
	w := scoll.Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")

	// [p_lemma="team" & deprel="nmod" & upos="NOUN"]
	cdb := NewCollDatabase(a.db, corpusID)

	fx, err := cdb.GetFreq("", "NOUN", w.V, w.PoS, "nmod")

	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	candidates, err := cdb.GetChildCandidates(w.V, w.PoS, "nmod", candidatesFreqLimit)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	result := make(results.FreqDistribItemList, 0, len(candidates))
	argsBatch := make([]batchFreqArgs, 0, fyBatchSize)

	processBatch := func(ab []batchFreqArgs) bool {
		mapping := make(map[string]*batchFreqArgs)
		for _, a := range ab {
			mapping[a.Lemma] = &a
		}
		fyList, err := cdb.GetFreqBatch(ab)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return false
		}
		for _, fy := range fyList {
			res, ok := mapping[fy.Lemma]
			if !ok {
				log.Warn().Str("value", fy.Lemma).Msg("value mapping ambiguity")
				continue
			}
			result = append(
				result,
				&results.FreqDistribItem{
					Word:       fy.Lemma,
					Freq:       res.fxy,
					CollWeight: 14 + math.Log2(2*float64(res.fxy)/(float64(fx)+float64(fy.Freq))),
				},
			)
		}
		argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		return true
	}

	for _, cand := range candidates {
		argsBatch = append(argsBatch, batchFreqArgs{
			fxy:          cand.Freq,
			isParentSrch: false,
			Lemma:        cand.Lemma,
			Upos:         cand.Upos,
			Deprel:       "nmod",
		})
		if len(argsBatch) == fyBatchSize {
			ok := processBatch(argsBatch)
			if !ok {
				return
			}
			argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		}
	}
	if len(argsBatch) > 0 {
		ok := processBatch(argsBatch)
		if !ok {
			return
		}
	}

	sort.SliceStable(
		result,
		func(i, j int) bool {
			return result[j].CollWeight < result[i].CollWeight
		},
	)
	sketchAttrs := a.initScollAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := scoll.NewQueryGenerator(corpusID, scoll.QueryModifiersOf, sketchAttrs)
	resp := results.FreqDistrib{
		Freqs:            result,
		ExamplesQueryTpl: queryGen.FxyQuery(w, "%s"),
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		resp,
	)
}

func (a *Actions) VerbsSubject(ctx *gin.Context) {
	w := scoll.Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	// [lemma="team" & deprel="nsubj" & p_upos="VERB"]
	cdb := NewCollDatabase(a.db, corpusID)

	fx, err := cdb.GetFreq(w.V, w.PoS, "", "VERB", "nsubj")
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	candidates, err := cdb.GetParentCandidates(w.V, w.PoS, "nsubj", candidatesFreqLimit)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	result := make(results.FreqDistribItemList, 0, len(candidates))
	argsBatch := make([]batchFreqArgs, 0, fyBatchSize)

	processBatch := func(ab []batchFreqArgs) bool {
		mapping := make(map[string]*batchFreqArgs)
		for _, a := range ab {
			mapping[a.Lemma] = &a
		}
		fyList, err := cdb.GetFreqBatch(ab)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return false
		}
		for _, fy := range fyList {
			res, ok := mapping[fy.Lemma]
			if !ok {
				log.Warn().Str("value", fy.Lemma).Msg("value mapping ambiguity")
				continue
			}
			result = append(
				result,
				&results.FreqDistribItem{
					Word:       fy.Lemma,
					Freq:       res.fxy,
					CollWeight: 14 + math.Log2(2*float64(res.fxy)/(float64(fx)+float64(fy.Freq))),
				},
			)
		}
		argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		return true
	}

	for _, cand := range candidates {
		argsBatch = append(argsBatch, batchFreqArgs{
			fxy:          cand.Freq,
			isParentSrch: true,
			Lemma:        cand.Lemma,
			Upos:         cand.Upos,
			Deprel:       "nsubj",
		})
		if len(argsBatch) == fyBatchSize {
			ok := processBatch(argsBatch)
			if !ok {
				return
			}
			argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		}
	}
	if len(argsBatch) > 0 {
		ok := processBatch(argsBatch)
		if !ok {
			return
		}
	}

	sort.SliceStable(
		result,
		func(i, j int) bool {
			return result[j].CollWeight < result[i].CollWeight
		},
	)
	sketchAttrs := a.initScollAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := scoll.NewQueryGenerator(corpusID, scoll.QueryVerbsSubject, sketchAttrs)
	resp := results.FreqDistrib{
		Freqs:            result,
		ExamplesQueryTpl: queryGen.FxyQuery(w, "%s"),
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		resp,
	)
}

func (a *Actions) VerbsObject(ctx *gin.Context) {
	w := scoll.Word{V: ctx.Request.URL.Query().Get("w"), PoS: ctx.Request.URL.Query().Get("pos")}
	if !w.IsValid() {
		uniresp.RespondWithErrorJSON(
			ctx,
			uniresp.NewActionError("invalid word value"),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	// [lemma="team" & deprel="obj|iobj" & p_upos="VERB"]
	cdb := NewCollDatabase(a.db, corpusID)

	fx, err := cdb.GetFreq(w.V, w.PoS, "", "VERB", "obj|iobj")
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	candidates, err := cdb.GetParentCandidates(w.V, w.PoS, "obj|iobj", candidatesFreqLimit)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	result := make(results.FreqDistribItemList, 0, len(candidates))
	argsBatch := make([]batchFreqArgs, 0, fyBatchSize)

	processBatch := func(ab []batchFreqArgs) bool {
		mapping := make(map[string]*batchFreqArgs)
		for _, a := range ab {
			mapping[a.Lemma] = &a
		}
		fyList, err := cdb.GetFreqBatch(ab)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return false
		}
		for _, fy := range fyList {
			res, ok := mapping[fy.Lemma]
			if !ok {
				log.Warn().Str("value", fy.Lemma).Msg("value mapping ambiguity")
				continue
			}
			result = append(
				result,
				&results.FreqDistribItem{
					Word:       fy.Lemma,
					Freq:       res.fxy,
					CollWeight: 14 + math.Log2(2*float64(res.fxy)/(float64(fx)+float64(fy.Freq))),
				},
			)
		}
		argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		return true
	}

	for _, cand := range candidates {
		argsBatch = append(argsBatch, batchFreqArgs{
			fxy:          cand.Freq,
			isParentSrch: true,
			Lemma:        cand.Lemma,
			Upos:         cand.Upos,
			Deprel:       "obj|iobj",
		})
		if len(argsBatch) == fyBatchSize {
			ok := processBatch(argsBatch)
			if !ok {
				return
			}
			argsBatch = make([]batchFreqArgs, 0, fyBatchSize)
		}
	}
	if len(argsBatch) > 0 {
		ok := processBatch(argsBatch)
		if !ok {
			return
		}
	}

	sort.SliceStable(
		result,
		func(i, j int) bool {
			return result[j].CollWeight < result[i].CollWeight
		},
	)
	sketchAttrs := a.initScollAttrsOrWriteErr(ctx, corpusID)
	if sketchAttrs == nil {
		return
	}
	queryGen := scoll.NewQueryGenerator(corpusID, scoll.QueryVerbsObject, sketchAttrs)
	resp := results.FreqDistrib{
		Freqs:            result,
		ExamplesQueryTpl: queryGen.FxyQuery(w, "%s"),
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		resp,
	)
}

func NewActions(
	corpConf *corpus.CorporaSetup,
	sketchConf *scoll.SketchSetup,
	db *sql.DB,
) *Actions {
	return &Actions{
		corpConf:   corpConf,
		sketchConf: sketchConf,
		db:         db,
	}
}
