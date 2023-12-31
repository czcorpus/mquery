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
	"encoding/json"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
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

func (a *Actions) findLemmas(corpusID string, word string, pos string) ([]*results.LemmaItem, error) {
	q := "word=\"" + word + "\""
	if len(pos) > 0 {
		q += " & pos=\"" + pos + "\""
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)
	args, err := json.Marshal(rdb.FreqDistribArgs{
		CorpusPath: corpusPath,
		Query:      "[" + q + "]",
		Crit:       "lemma 0~0>0 pos 0~0>0",
		FreqLimit:  1,
	})
	if err != nil {
		return nil, err
	}
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "freqDistrib",
		Args: args,
	})
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	freqs, err := rdb.DeserializeFreqDistribResult(rawResult)
	if err != nil {
		return nil, err
	}
	if err := freqs.Err(); err != nil {
		return nil, err
	}

	ans := make([]*results.LemmaItem, len(freqs.Freqs))
	for i, freq := range freqs.Freqs {
		wordSplit := strings.Split(freq.Word, " ")
		// this presumes only single word queries
		ans[i] = &results.LemmaItem{
			Lemma: wordSplit[0],
			POS:   wordSplit[1],
		}
	}
	return ans, nil
}

func (a *Actions) findWordForms(corpusID string, lemma string, pos string) (*results.WordFormsItem, error) {
	q := "lemma=\"" + lemma + "\""
	if len(pos) > 0 {
		q += " & pos=\"" + pos + "\""
	}
	corpusPath := a.conf.GetRegistryPath(corpusID)
	args, err := json.Marshal(rdb.FreqDistribArgs{
		CorpusPath: corpusPath,
		Query:      "[" + q + "]",
		Crit:       "word/i 0~0>0",
		FreqLimit:  1,
	})
	if err != nil {
		return nil, err
	}
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "freqDistrib",
		Args: args,
	})
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	freqs, err := rdb.DeserializeFreqDistribResult(rawResult)
	if err != nil {
		return nil, err
	}
	if err := freqs.Err(); err != nil {
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
		lemmas, err := a.findLemmas(ctx.Param("corpusId"), word, pos)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
			return
		}

		for _, v := range lemmas {
			wordForms, err := a.findWordForms(ctx.Param("corpusId"), v.Lemma, v.POS)
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

func (a *Actions) ConcExample(ctx *gin.Context) {
	attrs := []string{"word", "lemma", "p_lemma", "parent"} // TODO configurable
	corpusName := ctx.Param("corpusId")
	args, err := json.Marshal(rdb.ConcExampleArgs{
		CorpusPath:    a.conf.GetRegistryPath(corpusName),
		QueryLemma:    ctx.Query("lemma"),
		Query:         ctx.Query("query"),
		Attrs:         attrs,
		MaxItems:      10,
		ParentIdxAttr: a.conf.Resources[corpusName].SyntaxParentAttr.Name,
	})
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "concExample",
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
	result, err := rdb.DeserializeConcExampleResult(rawResult)
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

func NewActions(
	conf *corpus.CorporaSetup,
	radapter *rdb.Adapter,
) *Actions {
	return &Actions{
		conf:     conf,
		radapter: radapter,
	}
}
