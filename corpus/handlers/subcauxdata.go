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
	"mquery/corpus"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type SubcAuxDataStatus string

const (
	SubcAuxDataStatusExisting SubcAuxDataStatus = "existing"
	SubcAuxDataStatusCreated  SubcAuxDataStatus = "created"
)

type SubcAuxDataResponse struct {
	Corpus    string                       `json:"corpus"`
	Subcorpus string                       `json:"subcorpus"`
	Attrs     map[string]SubcAuxDataStatus `json:"attrs"`
}

// SubcAuxData godoc
// @Summary      Prepare auxiliary frequency data for a subcorpus
// @Description  Calculate auxiliary frequency data for positional attributes of a saved subcorpus. Such data are required by Manatee for some operations on subcorpora (e.g. collocations). The action is idempotent - in case the data already exist, nothing is calculated.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus"
// @Param        subcorpus query string true "An ID of a saved subcorpus"
// @Param        attr query []string false "a positional attribute to calculate the data for (can be repeated)" default(lemma)
// @Success      200 {object} SubcAuxDataResponse
// @Router       /subc-aux-data/{corpusId} [post]
func (a *Actions) SubcAuxData(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	corpusConf := a.conf.GetCorp(corpusID)
	if corpusConf == nil {
		uniresp.RespondWithErrorJSON(ctx, corpus.ErrNotFound, http.StatusNotFound)
		return
	}
	if !a.canAccessCorpusConf(ctx, corpusConf) {
		uniresp.RespondWithErrorJSON(ctx, corpus.ErrAccessDenied, http.StatusForbidden)
		return
	}

	subc := ctx.Query("subcorpus")
	if subc == "" {
		uniresp.RespondWithErrorJSON(
			ctx, errors.New("missing `subcorpus` argument"), http.StatusBadRequest)
		return
	}
	if _, ok := corpusConf.Subcorpora[subc]; ok {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("subcorpus %s is defined by text types and does not require auxiliary data", subc),
			http.StatusUnprocessableEntity,
		)
		return
	}
	subcPath, ok := corpus.CheckSavedSubcorpus(a.conf.SavedSubcorporaDir, corpusID, subc)
	if !ok {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("subcorpus %s not found", subc), http.StatusNotFound)
		return
	}

	attrs := ctx.QueryArray("attr")
	if len(attrs) == 0 {
		attrs = []string{CollDefaultAttr}
	}
	ans := SubcAuxDataResponse{
		Corpus:    corpusID,
		Subcorpus: subc,
		Attrs:     make(map[string]SubcAuxDataStatus),
	}
	missing := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		if corpusConf.GetPosAttr(attr).Name == "" {
			uniresp.RespondWithErrorJSON(
				ctx, fmt.Errorf("unknown positional attribute %s", attr), http.StatusBadRequest)
			return
		}
		exists, err := corpus.SubcFreqDataExists(subcPath, attr)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		if exists {
			ans.Attrs[attr] = SubcAuxDataStatusExisting

		} else if _, ok := ans.Attrs[attr]; !ok {
			ans.Attrs[attr] = SubcAuxDataStatusCreated
			missing = append(missing, attr)
		}
	}

	if len(missing) > 0 {
		wait, err := a.radapter.PublishQuery(
			rdb.Query{
				Func: "calcCollFreqData",
				Args: rdb.CalcCollFreqDataArgs{
					CorpusPath: a.conf.GetRegistryPath(corpusID),
					SubcPath:   subcPath,
					Attrs:      missing,
				},
			},
			GetCTXStoredTimeout(ctx),
		)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		rawResult := <-wait
		if ok := HandleWorkerError(ctx, rawResult); !ok {
			return
		}
		if _, ok := TypedOrRespondError[results.CollFreqData](ctx, rawResult); !ok {
			return
		}
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
