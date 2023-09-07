// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
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
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (a *Actions) TextTypes(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	attr := ctx.Request.URL.Query().Get("attr")
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
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	freqArgs := rdb.FreqDistribArgs{
		CorpusPath: corpusPath,
		Query:      q,
		Crit:       fmt.Sprintf("%s 0", attr),
		FreqLimit:  flimit,
	}

	// TODO this probably needs some work
	if ctx.Request.URL.Query().Has("subc") {
		freqArgs.SubcPath = ctx.Request.URL.Query().Get("subc")
	}

	args, err := json.Marshal(freqArgs)
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
	result, err := rdb.DeserializeTextTypesResult(rawResult)
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

func (a *Actions) TextTypesParallel(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")
	attr := ctx.Request.URL.Query().Get("attr")
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

	flimit, ok := unireq.GetURLIntArgOrFail(ctx, "flimit", 1)
	if !ok {
		return
	}
	maxItems, ok := unireq.GetURLIntArgOrFail(ctx, "maxItems", 0)
	if !ok {
		return
	}

	mergedFreqLock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(sc.Subcorpora))
	result := new(results.FreqDistrib)
	result.Freqs = make([]*results.FreqDistribItem, 0)
	errs := make([]error, 0, len(sc.Subcorpora))
	for _, subc := range sc.Subcorpora {
		args, err := json.Marshal(rdb.FreqDistribArgs{
			CorpusPath: corpusPath,
			SubcPath:   subc,
			Query:      q,
			Crit:       fmt.Sprintf("%s 0", attr),
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
			errs = append(errs, err)
			log.Error().Err(err).Msg("failed to publish query")
			wg.Done()

		} else {
			go func() {
				defer wg.Done()
				tmp := <-wait
				resultNext, err := rdb.DeserializeTextTypesResult(tmp)
				if err != nil {
					errs = append(errs, err)
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				mergedFreqLock.Lock()
				result.MergeWith(&resultNext)
				mergedFreqLock.Unlock()
			}()
		}
	}
	wg.Wait()

	if len(errs) > 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(errs[0]), http.StatusInternalServerError)
		return
	}

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
