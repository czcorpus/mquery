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
	"fmt"
	"math"
	"mquery/corpus"
	"mquery/corpus/scoll"
	"mquery/results"
	"net/http"
	"sort"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
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

	candidates, err := cdb.GetParentCandidates(w.V, w.PoS, "nmod")
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	result := make(results.FreqDistribItemList, len(candidates))
	for i, cand := range candidates {
		fxy := cand.Freq
		fy, err := cdb.GetFreq("", "", cand.Lemma, cand.Upos, "nmod")
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		item := &results.FreqDistribItem{
			Word:       cand.Lemma,
			Freq:       fxy,
			CollWeight: 14 + math.Log2(2*float64(fxy)/(float64(fx)+float64(fy))),
		}
		fmt.Println("CAND: ", *item)
		result[i] = item
	}
	sort.SliceStable(
		result,
		func(i, j int) bool {
			return result[j].CollWeight < result[i].CollWeight
		},
	)

	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
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
