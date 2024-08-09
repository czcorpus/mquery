// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
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

package handlers

import (
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"
	"sort"
	"sync"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

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
		wait, err := a.radapter.PublishQuery(rdb.Query{
			Func: "freqDistrib",
			Args: rdb.FreqDistribArgs{
				CorpusPath:  corpusPath,
				SubcPath:    subc,
				Query:       q,
				Crit:        fmt.Sprintf("%s 0", attr),
				IsTextTypes: true,
				FreqLimit:   flimit,
				MaxResults:  maxItems,
			},
		})
		if err != nil {
			errs = append(errs, err)
			log.Error().Err(err).Msg("failed to publish query")
			wg.Done()

		} else {
			go func() {
				defer wg.Done()
				tmp := <-wait
				if err := tmp.Value.Err(); err != nil {
					errs = append(errs, err)
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				resultNext, ok := tmp.Value.(results.FreqDistrib)
				if !ok {
					err := fmt.Errorf("invalid type for FreqDistrib")
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
