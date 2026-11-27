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
	"github.com/czcorpus/mquery/corpus"
	"github.com/czcorpus/mquery/rdb/results"
	"maps"
	"net/http"
	"slices"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/gin-gonic/gin"
)

type subcInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
} // @name SubcInfo

type corpusCompactInfo struct {
	ID          string `json:"id"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	// Language is the corpus language as specified in the corpus
	// registry file (a language name in English, e.g. "Czech")
	Language   string     `json:"language,omitempty"`
	Flags      []string   `json:"flags"`
	Subcorpora []subcInfo `json:"subcorpora"`
} // @name CorpusCompactInfo

type corplistResponse struct {
	Corpora []corpusCompactInfo `json:"corpora"`
	Locale  string              `json:"locale"`
} // @name Corplist

// corpusBriefInfo is a minimal corpus info (see the `compact` argument
// of the corplist action) - e.g. for LLM-based clients where response
// size matters
type corpusBriefInfo struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	// Language is the corpus language as specified in the corpus
	// registry file (a language name in English, e.g. "Czech")
	Language string   `json:"language,omitempty"`
	Flags    []string `json:"flags,omitempty"`
	// Subcorpora contains just subcorpora IDs (sorted)
	Subcorpora []string `json:"subcorpora,omitempty"`
} // @name CorpusBriefInfo

type corplistBriefResponse struct {
	Corpora []corpusBriefInfo `json:"corpora"`
	Locale  string            `json:"locale"`
} // @name CorplistBrief

func newCorpusBriefInfo(conf *corpus.MQCorpusSetup, lang string) corpusBriefInfo {
	return corpusBriefInfo{
		ID:         conf.ID,
		FullName:   getTranslation(conf.FullName, lang),
		Language:   conf.Language(),
		Flags:      conf.SrchKeywords,
		Subcorpora: slices.Sorted(maps.Keys(conf.Subcorpora)),
	}
}

type corpusInfoResponse struct {
	Corpus *results.CorpusInfo   `json:"corpus"`
	Locale string                `json:"locale"`
	Conf   *corpus.MQCorpusSetup `json:"conf,omitempty"`
} // @name CorpusInfo

func getTranslation(data map[string]string, lang string) string {
	v, ok := data[lang]
	if ok {
		return v
	}
	return data["en"]
}

// CorpusInfo godoc
// @Summary      CorpusInfo
// @Description  Shows a detailed corpus information, including size in tokens, available positional and structural attributes.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to get info about"
// @Param        locale query string false "An ISO 639-1 locale code of response." default(en)
// @Param		 attachConf query int 0 "If 1, then attach MQuery configuration of the corpus"
// @Success      200 {object} corpusInfoResponse
// @Router       /info/{corpusId} [get]
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
	corpusConf := a.conf.GetCorp(corpusID)
	if corpusConf == nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			corpus.ErrNotFound,
			http.StatusNotFound,
		)
		return
	}
	if !a.canAccessCorpusOrFail(ctx, corpusID) {
		return
	}

	cinfo, err := a.infoProvider.LoadCorpusInfo(corpusID, lang)
	if err == corpus.ErrNotFound {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	cinfo.Data.TextProperties = make([]corp.TextProperty, len(corpusConf.TextProperties))
	var i int
	for prop := range corpusConf.TextProperties {
		cinfo.Data.TextProperties[i] = prop
		i++
	}
	ans := &corpusInfoResponse{
		Locale: lang,
		Corpus: cinfo,
	}
	if ctx.Query("attachConf") == "1" {
		ans.Conf = corpusConf
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// Corplist godoc
// @Summary      Corplist
// @Description  Shows a list of available corpora with their basic properties.
// @Description  With `compact=1`, each corpus contains only `id`, `fullName`, `language`, `flags` and
// @Description  `subcorpora` (a sorted list of IDs); empty values are omitted.
// @Produce      json
// @Param        lang query string false "An ISO 639-1 locale code of response" default(en)
// @Param        compact query int false "If 1, then a much smaller response with essential properties only is returned" default(0)
// @Success      200 {object} corplistResponse
// @Router       /corplist [get]
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
	allCorpora := a.conf.GetAllCorpora("") // TODO
	if ctx.Query("compact") == "1" {
		brief := make([]corpusBriefInfo, 0, len(allCorpora))
		for _, v := range allCorpora {
			if a.canAccessCorpusConf(ctx, v) {
				brief = append(brief, newCorpusBriefInfo(v, lang))
			}
		}
		uniresp.WriteJSONResponse(ctx.Writer, &corplistBriefResponse{Corpora: brief, Locale: lang})
		return
	}
	corplist := make([]corpusCompactInfo, 0, len(allCorpora))
	for _, v := range allCorpora {
		if !a.canAccessCorpusConf(ctx, v) {
			continue
		}
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
		corplist = append(
			corplist,
			corpusCompactInfo{
				ID:          v.ID,
				FullName:    getTranslation(v.FullName, lang),
				Description: getTranslation(v.Description, lang),
				Language:    v.Language(),
				Flags:       v.SrchKeywords,
				Subcorpora:  subcorpora,
			},
		)
	}
	ans := &corplistResponse{
		Corpora: corplist,
		Locale:  lang,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
