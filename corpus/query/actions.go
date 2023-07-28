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

package query

import (
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

var (
	collFunc = map[string]string{
		"absoluteFreq":  "f",
		"LLH":           "l",
		"logDice":       "d",
		"minSens":       "s",
		"mutualInf":     "m",
		"mutualInf3":    "3",
		"mutualInfLogF": "p",
		"relativeFreq":  "r",
		"tScore":        "t",
	}
)

type Actions struct {
	conf     *corpus.CorporaSetup
	radapter *rdb.Adapter
}

func (a *Actions) FreqDistrib(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	log.Debug().
		Str("query", q).
		Msg("processing Mango query")
	flimit := 1
	if ctx.Request.URL.Query().Has("flimit") {
		var err error
		flimit, err = strconv.Atoi(ctx.Request.URL.Query().Get("flimit"))
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "freqDistrib",
		Args: []any{ctx.Param("corpusId"), q, "lemma/e 0~0>0", flimit},
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
	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func (a *Actions) Collocations(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	log.Debug().
		Str("query", q).
		Msg("processing Mango query")

	collFnArg := ctx.Request.URL.Query().Get("fn")
	collFn, ok := collFunc[collFnArg]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("unknown collocations function %s", collFnArg),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "collocations",
		Args: []any{corpusPath, q, "word", collFn, 20, 20},
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
	result, err := rdb.DeserializeCollocationsResult(rawResult)
	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func (a *Actions) findLemmas(corpusID string, word string, pos string) (*results.WordFormsItem, error) {
	q := "word=\"" + word + "\""
	if len(pos) > 0 {
		q += " & pos=\"" + pos + "\""
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "freqDistrib",
		Args: []any{corpusPath, "[" + q + "]", "lemma 0~0>0 pos 0~0>0", 1},
	})
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	freqs, err := rdb.DeserializeFreqDistribResult(rawResult)
	if err != nil {
		return nil, err
	}

	ans := &results.WordFormsItem{
		Lemma: "-- TODO ---", // TODO !!
		POS:   pos,
		Forms: make([]*results.FreqDistribItem, len(freqs.Freqs)),
	}
	return ans, nil
}

func (a *Actions) findWordForms(corpusID string, lemma string, pos string) (*results.WordFormsItem, error) {
	q := "lemma=\"" + lemma + "\""
	if len(pos) > 0 {
		q += " & pos=\"" + pos + "\""
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)
	wait, err := a.radapter.CacheResult(
		a.radapter.PublishQuery,
		rdb.Query{
			Func: "freqDistrib",
			Args: []any{corpusPath, "[" + q + "]", "word/i 0~0>0", 1},
		},
	)
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	freqs, err := rdb.DeserializeFreqDistribResult(rawResult)
	if err != nil {
		return nil, err
	}

	ans := &results.WordFormsItem{
		Lemma: lemma,
		POS:   pos,
		Forms: freqs.Freqs,
	}
	return ans, nil
}

func (a *Actions) WordForms(ctx *gin.Context) {
	var ans []*results.WordFormsItem
	lemma := ctx.Request.URL.Query().Get("lemma")
	word := ctx.Request.URL.Query().Get("word")
	pos := ctx.Request.URL.Query().Get("pos")
	if lemma != "" {
		log.Debug().
			Str("lemma", lemma).
			Str("pos", pos).
			Msg("processing Mango query")
		wordForms, err := a.findWordForms(ctx.Param("corpusId"), lemma, pos)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
			return
		}
		ans = append(ans, wordForms)

	} else if len(word) > 0 {
		log.Debug().
			Str("word", word).
			Str("pos", pos).
			Msg("processing Mango query")
		lemmas, err := a.findLemmas(ctx.Param("corpusId"), word, pos)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
			return
		}

		for _, lemmaPos := range lemmas.Forms {
			lemmaPosSplit := strings.Split(lemmaPos.Word, " ")
			pos := lemmaPosSplit[len(lemmaPosSplit)-1]
			lemma := strings.Join(lemmaPosSplit[:len(lemmaPosSplit)-1], " ")
			wordForms, err := a.findWordForms(ctx.Param("corpusId"), lemma, pos)
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

	} else {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("Required parameters are `lemma` or `word`"),
			http.StatusBadRequest,
		)
		return
	}

	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewActions(conf *corpus.CorporaSetup, radapter *rdb.Adapter) *Actions {
	return &Actions{
		conf:     conf,
		radapter: radapter,
	}
}
