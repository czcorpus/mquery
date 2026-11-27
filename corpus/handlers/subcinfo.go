// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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
	"github.com/czcorpus/mquery/corpus"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	subcTypeAlias = "alias"
	subcTypeSaved = "saved"
)

var errSubcNotFound = errors.New("subcorpus not found")

type subcInfoResponse struct {
	ID       string `json:"id"`
	CorpusID string `json:"corpusId"`
	// Type is either `alias` (a subcorpus defined in MQuery configuration
	// via text types) or `saved` (a subcorpus file in the saved subcorpora directory)
	Type        string `json:"type" enums:"alias,saved"`
	Description string `json:"description,omitempty"`
	// Size is the subcorpus size in tokens (saved subcorpora only)
	Size int64 `json:"size,omitempty"`
	// NumRanges is the number of position ranges forming the subcorpus (saved subcorpora only)
	NumRanges  int64 `json:"numRanges,omitempty"`
	CorpusSize int64 `json:"corpusSize"`
} // @name SubcorpusInfo

// SubcorpusInfo godoc
// @Summary      SubcorpusInfo
// @Description  Tests whether a subcorpus can be used with a corpus and shows its basic properties.
// @Description  The subcorpus can be either a configured one (`type: alias`, see `subcorpora` in /corplist)
// @Description  or a saved one identified by its ID (`type: saved`; its size is returned).
// @Description  As a saved subcorpus file contains no reference to its corpus, the server can only verify
// @Description  that it does not span beyond the end of the corpus (HTTP 422 otherwise).
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus"
// @Param        subcorpusId path string true "An ID of a subcorpus"
// @Param        lang query string false "An ISO 639-1 locale code of response" default(en)
// @Success      200 {object} subcInfoResponse
// @Router       /subcorpus/{corpusId}/{subcorpusId} [get]
func (a *Actions) SubcorpusInfo(ctx *gin.Context) {
	lang := ctx.DefaultQuery("lang", a.locales.DefaultLocale())
	if !a.locales.SupportsLocale(lang) {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("unsupported locale `%s`", lang), http.StatusUnprocessableEntity)
		return
	}
	corpusID := ctx.Param("corpusId")
	subcID := ctx.Param("subcorpusId")
	corpusConf := a.conf.GetCorp(corpusID)
	if corpusConf == nil {
		uniresp.RespondWithErrorJSON(ctx, corpus.ErrNotFound, http.StatusNotFound)
		return
	}
	if !a.canAccessCorpusConf(ctx, corpusConf) {
		uniresp.RespondWithErrorJSON(ctx, corpus.ErrAccessDenied, http.StatusForbidden)
		return
	}
	cinfo, err := a.infoProvider.LoadCorpusInfo(corpusID, lang)
	if err == corpus.ErrNotFound {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	ans := subcInfoResponse{
		ID:         subcID,
		CorpusID:   corpusID,
		CorpusSize: cinfo.Data.Size,
	}

	if alias, ok := corpusConf.Subcorpora[subcID]; ok {
		ans.Type = subcTypeAlias
		ans.Description = getTranslation(alias.Description, lang)
		uniresp.WriteJSONResponse(ctx.Writer, ans)
		return
	}

	subcPath, ok := corpus.CheckSavedSubcorpus(a.conf.SavedSubcorporaDir, corpusID, subcID)
	if !ok {
		uniresp.RespondWithErrorJSON(ctx, errSubcNotFound, http.StatusNotFound)
		return
	}
	sinfo, err := corpus.ReadSavedSubcorpusInfo(subcPath)
	if err != nil {
		// the path is logged only (it must not be exposed)
		log.Error().Err(err).Str("path", subcPath).Msg("failed to read saved subcorpus")
		uniresp.RespondWithErrorJSON(
			ctx, errors.New("failed to read the subcorpus"), http.StatusInternalServerError)
		return
	}
	if sinfo.MaxPos > cinfo.Data.Size {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf(
				"subcorpus %s cannot belong to corpus %s (it spans beyond the end of the corpus)",
				subcID, corpusID,
			),
			http.StatusUnprocessableEntity,
		)
		return
	}
	ans.Type = subcTypeSaved
	ans.Size = sinfo.Size
	ans.NumRanges = sinfo.NumRanges
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
