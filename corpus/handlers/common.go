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
	"encoding/json"
	"errors"
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	TimeoutCtxKey = "workerTimeout"
)

type queryProps struct {
	corpus         string
	savedSubcorpus string
	query          string
	err            error
	corpusConf     *corpus.MQCorpusSetup
	status         int
}

func (qp queryProps) hasError() bool {
	return qp.err != nil
}

// DetermineQueryProps searches for common arguments
// required for most query+operation actions (freqs, colls, concordance)
// Those are:
// * `q` for Manatee CQL query
// * `subcorpus` for a named ad-hoc subcorpus
func DetermineQueryProps(ctx *gin.Context, cConf *corpus.CorporaSetup) queryProps {
	var ans queryProps
	ans.corpus = ctx.Param("corpusId")
	corpusConf := cConf.Resources.Get(ans.corpus)
	if corpusConf == nil {
		ans.err = fmt.Errorf("corpus %s not found", ans.corpus)
		ans.status = http.StatusNotFound
		return ans
	}
	ans.corpusConf = corpusConf

	var ttCQL string
	userQuery := ctx.Query("q")
	if userQuery == "" {
		ans.err = errors.New("missing `q` argument")
		ans.status = http.StatusBadRequest
		return ans
	}
	subc := ctx.Query("subcorpus")
	if subc != "" {
		ttCQL = corpus.SubcorpusToCQL(corpusConf.Subcorpora[subc].TextTypes)
		if ttCQL == "" {
			savedSubcPath, ok := corpus.CheckSavedSubcorpus(cConf.SavedSubcorporaDir, ans.corpus, subc)
			if ok {
				ans.savedSubcorpus = savedSubcPath

			} else {
				ans.err = fmt.Errorf("invalid subcorpus specification: %s", savedSubcPath)
				ans.status = http.StatusUnprocessableEntity
				return ans
			}
		}
	}
	ans.query = userQuery + ttCQL
	return ans
}

func (a *Actions) DecodeTextTypeAttrOrFail(
	ctx *gin.Context,
	corpusID string,
) (string, bool) {
	corpus := ctx.Param("corpusId")
	attr := corp.TextProperty(ctx.Query("attr"))
	tProp := ctx.Query("textProperty")
	if attr != "" && tProp != "" {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("cannot use attr and textProperty at the same time"),
			http.StatusBadRequest,
		)
		return "", false
	}
	if attr != "" {
		return attr.String(), true
	}
	if tProp != "" {
		corpConf := a.conf.Resources.Get(corpus)
		if corpConf == nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("unknown corpus"),
				http.StatusNotFound,
			)
			return "", false
		}
		tp, ok := corpConf.TextProperties[corp.TextProperty(tProp)]
		if !ok {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("unknown text property"),
				http.StatusUnprocessableEntity,
			)
			return "", false
		}
		return tp.Name, true
	}
	return "", true
}

func TypedOrRespondError[T any](ctx *gin.Context, w rdb.WorkerResult) (T, bool) {
	if w.Value == nil {
		var ans T
		return ans, false
	}
	vt, ok := w.Value.(T)
	if !ok {
		var n T
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf(
				"unexpected type for %s: %s",
				reflect.TypeOf(n), reflect.TypeOf(w.Value)),
			http.StatusInternalServerError,
		)
		return n, false
	}
	return vt, true
}

func HandleWorkerError(ctx *gin.Context, result rdb.WorkerResult) bool {
	if err := result.Value.Err(); err != nil {
		if result.HasUserError {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusBadRequest,
			)
			return false

		} else {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusInternalServerError,
			)
		}
		return false
	}
	return true
}

func WriteStreamingError(ctx *gin.Context, err error) {
	messageJSON, err2 := json.Marshal(streamingError{err.Error()})
	if err2 != nil {
		messageJSON = []byte(fmt.Sprintf(`{"error": "failed to encode error: %s"}`, err2))
	}
	// We use status 200 here deliberately as we don't want to trigger
	// the error handler.
	ctx.String(http.StatusOK, fmt.Sprintf("data: %s\n\n", messageJSON))
}

func HandleWorkerErrorStreaming(ctx *gin.Context, result rdb.WorkerResult) bool {
	if err := result.Value.Err(); err != nil {
		WriteStreamingError(ctx, err)
		return false
	}
	return true
}

func TypedOrRespondErrorStreaming[T any](ctx *gin.Context, w rdb.WorkerResult) (T, bool) {
	if w.Value == nil {
		var ans T
		return ans, false
	}
	vt, ok := w.Value.(T)
	if !ok {
		var n T
		WriteStreamingError(
			ctx,
			fmt.Errorf(
				"unexpected type for %s: %s",
				reflect.TypeOf(n), reflect.TypeOf(w.Value),
			),
		)
		return n, false
	}
	return vt, true
}

func GetURLIntArgOrFailStreaming(ctx *gin.Context, name string, dflt int) (int, bool) {
	if !ctx.Request.URL.Query().Has(name) {
		return dflt, true
	}
	tmp := ctx.Request.URL.Query().Get(name)
	value, err := strconv.Atoi(tmp)
	if err != nil {
		WriteStreamingError(ctx, err)
		return 0, false
	}
	return value, true
}

func GetCTXStoredTimeout(ctx *gin.Context) time.Duration {
	tmp, ok := ctx.Get(TimeoutCtxKey)
	if !ok {
		return 0
	}
	v, ok := tmp.(time.Duration)
	if !ok {
		log.Error().Msgf("ctx-stored timeout has invalid data type")
		return 0
	}
	return v
}
