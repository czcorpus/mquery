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
	"errors"
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	MaxWordFormResultItems = 50
)

type lemmaItem struct {
	Lemma    string `json:"lemma"`
	Sublemma string `json:"sublemma,omitempty"`
	POS      string `json:"pos"`
}

func (a *Actions) findLemmas(corpusID string, word, pos string, exportSublemmas bool, workerTimeout time.Duration) ([]*lemmaItem, error) {
	q := "word=\"" + word + "\""
	if len(pos) > 0 {
		q += " & pos=\"" + pos + "\""
	}
	crit := "lemma 0~0>0 pos 0~0>0"
	if exportSublemmas {
		crit = crit + " sublemma 0~0>0"
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)
	wait, err := a.radapter.PublishQuery(
		rdb.Query{
			Func: "freqDistrib",
			Args: rdb.FreqDistribArgs{
				CorpusPath: corpusPath,
				Query:      "[" + q + "]",
				Crit:       crit,
				FreqLimit:  1,
				MaxItems:   500,
			},
		},
		workerTimeout,
	)
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	freqs, ok := rawResult.Value.(results.FreqDistrib)
	if !ok {
		return nil, fmt.Errorf("invalid type for FreqDistrib")
	}
	if err := freqs.Err(); err != nil {
		return nil, err
	}

	ans := make([]*lemmaItem, len(freqs.Freqs))
	for i, freq := range freqs.Freqs {
		wordSplit := strings.Split(freq.Word, " ")
		// this presumes only single word queries
		ans[i] = &lemmaItem{
			Lemma: wordSplit[0],
			POS:   wordSplit[1],
		}
		if exportSublemmas && len(wordSplit) > 2 {
			ans[i].Sublemma = wordSplit[2]
		}
	}
	return ans, nil
}

func (a *Actions) findWordForms(corpusID string, lemma *lemmaItem, caseSensitive bool, workerTimeout time.Duration) (*results.WordFormsItem, error) {
	q := "lemma=\"" + lemma.Lemma + "\"" // TODO hardcoded `lemma`
	if lemma.POS != "" {
		q += " & pos=\"" + lemma.POS + "\"" // TODO hardcoded `pos`
	}
	if lemma.Sublemma != "" {
		q += " & sublemma=\"" + lemma.Sublemma + "\""
	}
	crit := "word 0~0>0"
	if !caseSensitive {
		crit = "word/i 0~0>0"
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)
	wait, err := a.radapter.PublishQuery(
		rdb.Query{
			Func: "freqDistrib",
			Args: rdb.FreqDistribArgs{
				CorpusPath: corpusPath,
				Query:      "[" + q + "]",
				Crit:       crit,
				FreqLimit:  1,
				MaxItems:   MaxWordFormResultItems,
			},
		},
		workerTimeout,
	)
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	if rawResult.Value.Err() != nil {
		return nil, fmt.Errorf("failed to find word forms: %w", rawResult.Value.Err())
	}
	freqs, ok := rawResult.Value.(results.FreqDistrib)
	if !ok {
		return nil, fmt.Errorf("failed to find word forms: invalid type for FreqDistrib")
	}

	ans := &results.WordFormsItem{
		Lemma:    lemma.Lemma,
		Sublemma: lemma.Sublemma,
		POS:      lemma.POS,
		Forms:    freqs.Freqs.AlwaysAsList(),
	}
	return ans, nil
}

// OtherForms godoc
// @Summary      WordForms
// @Description  Based of a provided word form, find all the other forms beloning to form's one or more lemmas/sublemmas
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param		 anyForm path string true "A lemma to search forms for"
// @Success      200 {array} results.WordFormsItem
// @Router       /other-forms/{corpusId}/{anyForm} [get]
func (a *Actions) OtherForms(ctx *gin.Context) {
	word := ctx.Param("wordForm")
	pos := ctx.Query("pos")
	corpusID := ctx.Param("corpusId")
	corpInfo := a.conf.GetCorp(corpusID)
	if corpInfo == nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			corpus.ErrNotFound,
			http.StatusNotFound,
		)
		return
	}

	var ans []*results.WordFormsItem
	hasSublemma := corpInfo.PosAttrs.Contains("sublemma")

	lemmas, err := a.findLemmas(corpusID, word, pos, hasSublemma, GetCTXStoredTimeout(ctx))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

	groupedFreqs := collections.SliceGroupBy(
		lemmas,
		func(item *lemmaItem) string {
			return item.Sublemma
		},
	)

	for _, v := range groupedFreqs {
		// as we group by sublemmas, to get sublemma, we can
		// just take the first item of the group (see v[0] below)
		wordForms, err := a.findWordForms(ctx.Param("corpusId"), v[0], true, GetCTXStoredTimeout(ctx))
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
			return
		}
		ans = append(ans, wordForms)
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// WordForms godoc
// @Summary      WordForms
// @Description  Get word forms of a lemma (plus optionally a sublemma and/or PoS).
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param		 lemma path string true "A lemma to search forms for"
// @Param        sublemma query string false "A sublemma to search - it must match the lemma argument, otherwise, 404 is returned"
// @Param        pos query string false "A Part of Speech to search - it must match the lemma (and sublemma), otherwise, 404 is returned"
// @Success      200 {array} results.WordFormsItem
// @Router       /word-forms/{corpusId}/{lemma} [get]
func (a *Actions) WordForms(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	lemma := ctx.Param("lemma")
	sublemma := ctx.Query("sublemma")

	var ans []*results.WordFormsItem

	pos := ctx.Query("pos")
	if ctx.Request.URL.Query().Has("subcorpus") {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("subcorpora not supported (yet) in word forms"),
			http.StatusBadRequest,
		)
		return
	}
	if lemma == "" {
		uniresp.RespondWithErrorJSON(
			ctx,
			errors.New("No lemma specified"),
			http.StatusBadRequest,
		)
		return
	}
	wordForms, err := a.findWordForms(
		corpusID,
		&lemmaItem{Lemma: lemma, Sublemma: sublemma, POS: pos},
		true,
		GetCTXStoredTimeout(ctx),
	)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusInternalServerError,
		)
		return
	}
	if wordForms == nil || len(wordForms.Forms) == 0 {
		uniresp.RespondWithErrorJSON(
			ctx,
			errors.New("Lemma not found"),
			http.StatusNotFound,
		)
		return
	}

	ans = append(ans, wordForms)

	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
