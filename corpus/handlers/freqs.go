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
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	defaultFreqCritTpl = "%s/%s 0~0>0"
	DefaultFreqLimit   = 1
	DefaultFreqAttr    = "lemma"
	DefaultFreqCrit    = "lemma/e 0~0>0"
	MaxFreqResultItems = 20
)

// FreqDistrib godoc
// @Summary      FreqDistrib
// @Description  Calculate a frequency distribution for a searched term (KWIC).
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "The translated query"
// @Param        subcorpus query string false "An ID of a subcorpus"
// @Param        attr query string false "a positional attribute (e.g. `word`, `lemma`, `tag`) the frequency will be calculated on" default(lemma)
// @Param        matchCase query int false " " enums(0, 1)
// @Param        maxItems query int false "maximum number of result items" default(20)
// @Param        flimit query int false "minimum frequency of result items to be included in the result set" minimum(0) default(1)
// @Success      200 {object} results.FreqDistribResponse
// @Router       /freqs/{corpusId} [get]
func (a *Actions) FreqDistrib(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}
	flimit := DefaultFreqLimit
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
	attr := ctx.Request.URL.Query().Get("attr")
	if attr == "" {
		attr = DefaultFreqAttr
	}
	matchCase := ctx.Request.URL.Query().Get("matchCase")
	var ic string
	if matchCase == "1" {
		ic = "e"

	} else {
		ic = "i"
	}
	fcrit := fmt.Sprintf(defaultFreqCritTpl, attr, ic)

	maxItems, ok := unireq.GetURLIntArgOrFail(ctx, "maxItems", MaxFreqResultItems)
	if !ok {
		return
	}

	corpusPath := a.conf.GetRegistryPath(queryProps.corpus)
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "freqDistrib",
		Args: rdb.FreqDistribArgs{
			CorpusPath: corpusPath,
			Query:      queryProps.query,
			Crit:       fcrit,
			FreqLimit:  flimit,
			MaxResults: maxItems,
		},
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
	if ok := HandleWorkerError(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondError[results.FreqDistrib](ctx, rawResult)
	if !ok {
		return
	}
	uniresp.WriteJSONResponse(
		ctx.Writer,
		&result,
	)
}

func (a *Actions) FreqDistribParallel(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}

	corpusPath := a.conf.GetRegistryPath(queryProps.corpus)
	sc, err := corpus.OpenSplitCorpus(a.conf.SplitCorporaDir, corpusPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

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

	maxItems := MaxFreqResultItems
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

	within := ""
	q := queryProps.query
	if ctx.Request.URL.Query().Has("within") { // TODO - here we have double within! (one from configured subcorpora)
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
		fcrit = DefaultFreqCrit
	}
	for _, subc := range sc.Subcorpora {
		wait, err := a.radapter.PublishQuery(rdb.Query{
			Func: "freqDistrib",
			Args: rdb.FreqDistribArgs{
				CorpusPath: corpusPath,
				SubcPath:   subc,
				Query:      q,
				Crit:       fcrit,
				FreqLimit:  flimit,
				MaxResults: maxItems,
			},
		})
		if err != nil {
			// TODO
			log.Error().Err(err).Msg("failed to publish query")

		} else {
			go func() {
				defer wg.Done()
				tmp := <-wait
				if err := tmp.Value.Err(); err != nil {
					// TODO
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				resultNext, ok := tmp.Value.(results.FreqDistrib)
				if !ok {
					// TODO
					err := fmt.Errorf("invalid type for FreqDistrib")
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				mergedFreqLock.Lock()
				result.MergeWith(&resultNext)
				mergedFreqLock.Unlock()
			}()
		}
	}
	wg.Wait()
	// TODO: no need to sort here (already sorted on worker)
	sort.SliceStable(
		result.Freqs,
		func(i, j int) bool {
			return result.Freqs[i].Freq > result.Freqs[j].Freq
		},
	)
	cut := maxItems
	if maxItems == 0 {
		cut = MaxFreqResultItems
	}
	result.Freqs = result.Freqs.Cut(cut)
	uniresp.WriteJSONResponse(ctx.Writer, result)
}
