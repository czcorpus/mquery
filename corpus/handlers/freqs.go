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
	"mquery/results"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	defaultFreqCrit = "lemma/e 0~0>0"
)

func (a *Actions) FreqDistrib(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
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
	fcrit := ctx.Request.URL.Query().Get("fcrit")
	if fcrit == "" {
		fcrit = defaultFreqCrit
	}
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	args, err := json.Marshal(rdb.FreqDistribArgs{
		CorpusPath: corpusPath,
		Query:      q,
		Crit:       fcrit,
		FreqLimit:  flimit,
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
		Func: "freqDistrib",
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
	result, err := rdb.DeserializeFreqDistribResult(rawResult)
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
	uniresp.WriteJSONResponse(
		ctx.Writer,
		result,
	)
}

func (a *Actions) FreqDistribParallel(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	flimit := 1
	maxItems := 0
	within := ""
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	sc, err := corpus.OpenSplitCorpus(a.conf.SplitCorporaDir, corpusPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

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

	if ctx.Request.URL.Query().Has("maxItems") {
		var err error
		maxItems, err = strconv.Atoi(ctx.Request.URL.Query().Get("maxItems"))
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionErrorFrom(err),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	if ctx.Request.URL.Query().Has("within") {
		within = ctx.Request.URL.Query().Get("within")
		if within == "" {
			uniresp.RespondWithErrorJSON(
				ctx,
				errors.New("empty `within` argument"),
				http.StatusBadRequest,
			)
			return
		}
		tmp := strings.SplitN(within, "=", 2)
		if len(tmp) != 2 {
			uniresp.RespondWithErrorJSON(
				ctx,
				errors.New("invalid `within` expression"),
				http.StatusBadRequest,
			)
			return
		}
		kv := strings.Split(tmp[0], ".")
		if len(kv) != 2 {
			uniresp.RespondWithErrorJSON(
				ctx,
				errors.New("invalid `within` expression"),
				http.StatusBadRequest,
			)
			return
		}
		q = fmt.Sprintf("%s within <%s %s=\"%s\" />", q, kv[0], kv[1], tmp[1])
	}
	mergedFreqLock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(sc.Subcorpora))
	result := new(results.FreqDistrib)
	result.Freqs = make([]*results.FreqDistribItem, 0)
	fcrit := ctx.Request.URL.Query().Get("fcrit")
	if fcrit == "" {
		fcrit = defaultFreqCrit
	}
	for _, subc := range sc.Subcorpora {
		args, err := json.Marshal(rdb.FreqDistribArgs{
			CorpusPath: corpusPath,
			SubcPath:   subc,
			Query:      q,
			Crit:       fcrit,
			FreqLimit:  flimit,
			MaxResults: maxItems,
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
			Func: "freqDistrib",
			Args: args,
		})
		if err != nil {
			// TODO
			log.Error().Err(err).Msg("failed to publish query")

		} else {
			go func() {
				defer wg.Done()
				tmp := <-wait
				resultNext, err := rdb.DeserializeFreqDistribResult(tmp)
				if err != nil {
					// TODO
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				if err := resultNext.Err(); err != nil {
					// TODO
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				mergedFreqLock.Lock()
				result.MergeWith(&resultNext)
				mergedFreqLock.Unlock()
			}()
		}
	}
	wg.Wait()
	sort.SliceStable(
		result.Freqs,
		func(i, j int) bool {
			return result.Freqs[i].Freq > result.Freqs[j].Freq
		},
	)
	cut := maxItems
	if maxItems == 0 {
		cut = 100 // TODO !!! (configured on worker, cannot import here)
	}
	result.Freqs = result.Freqs.Cut(cut)
	uniresp.WriteJSONResponse(ctx.Writer, result)
}
