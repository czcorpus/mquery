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
	"fmt"
	"mquery/corpus"
	"mquery/corpus/baseinfo"
	"mquery/rdb/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type subcInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type corpusCompactInfo struct {
	ID          string     `json:"id"`
	FullName    string     `json:"fullName"`
	Description string     `json:"description"`
	Flags       []string   `json:"flags"`
	Subcorpora  []subcInfo `json:"subcorpora"`
}

type corplistResponse struct {
	Corpora []corpusCompactInfo `json:"corpora"`
	Locale  string              `json:"locale"`
}

type corpusInfoResponse struct {
	Corpus *results.CorpusInfo `json:"corpus"`
	Locale string              `json:"locale"`
}

func getTranslation(data map[string]string, lang string) string {
	v, ok := data[lang]
	if ok {
		return v
	}
	return data["en"]
}

func (a *Actions) CorpusInfo(ctx *gin.Context) {
	lang := ctx.DefaultQuery("lang", a.locales.DefaultLocale())
	if !a.locales.SupportsLocale(lang) {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("unsupported locale `%s`", lang),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusID := ctx.Param("corpusId")
	info, err := a.infoProvider.LoadCorpusInfo(corpusID, lang)
	if err == corpus.ErrNotFound {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	corpusConf := a.conf.Resources.Get(corpusID)
	if corpusConf == nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("corpus not configured"),
			http.StatusNotFound,
		)
		return
	}
	info.Data.TextProperties = make([]baseinfo.TextProperty, len(corpusConf.TextProperties))
	var i int
	for prop := range corpusConf.TextProperties {
		info.Data.TextProperties[i] = prop
		i++
	}
	ans := &corpusInfoResponse{
		Locale: lang,
		Corpus: info,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) Corplist(ctx *gin.Context) {
	lang := ctx.DefaultQuery("lang", a.locales.DefaultLocale())
	if !a.locales.SupportsLocale(lang) {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("unsupported locale `%s`", lang),
			http.StatusUnprocessableEntity,
		)
		return
	}
	allCorpora := a.conf.Resources.GetAllCorpora()
	corplist := make([]corpusCompactInfo, len(allCorpora))
	for i, v := range a.conf.Resources.GetAllCorpora() {
		subcorpora := make([]subcInfo, 0, len(v.Subcorpora))
		for k, v := range v.Subcorpora {
			subcorpora = append(
				subcorpora,
				subcInfo{
					ID:          k,
					Description: getTranslation(v.Description, lang),
				},
			)
		}
		corplist[i] = corpusCompactInfo{
			ID:          v.ID,
			FullName:    getTranslation(v.FullName, lang),
			Description: getTranslation(v.Description, lang),
			Flags:       v.SrchKeywords,
			Subcorpora:  subcorpora,
		}
	}
	ans := &corplistResponse{
		Corpora: corplist,
		Locale:  lang,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
