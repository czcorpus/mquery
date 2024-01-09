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
	"math/rand"
	"mquery/corpus"
	"mquery/corpus/query"
	"mquery/mango"
	"mquery/rdb"
	"net/http"
	"sync"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	CollDefaultAttr       = "lemma"
	defaultNumSubcSamples = 30
)

func (a *Actions) Collocations(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")

	collFnArg := ctx.Request.URL.Query().Get("fn")
	collFn, ok := collFunc[collFnArg]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("unknown collocations function %s", collFnArg),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))

	args, err := json.Marshal(rdb.CollocationsArgs{
		CorpusPath: corpusPath,
		Query:      q,
		Attr:       CollDefaultAttr,
		CollFn:     collFn,
		MinFreq:    20,
		MaxItems:   20,
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
		Func: "collocations",
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
	result, err := rdb.DeserializeCollocationsResult(rawResult)
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

func (a *Actions) CollocationsParallel(ctx *gin.Context) {
	q := ctx.Request.URL.Query().Get("q")

	collFnArg := ctx.Request.URL.Query().Get("fn")
	collFn, ok := collFunc[collFnArg]
	if !ok {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("unknown collocations function %s", collFnArg),
			http.StatusUnprocessableEntity,
		)
		return
	}
	corpusPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))

	sc, err := corpus.OpenMultisampledCorpus(a.conf.MultisampledCorporaDir, corpusPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

	mergedFreqLock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(defaultNumSubcSamples)
	result := new(query.MultivalueColls)
	result.Values = make(map[string][]*mango.GoCollItem)

	for i := 0; i < defaultNumSubcSamples; i++ {
		subc := sc.Subcorpora[rand.Intn(len(sc.Subcorpora))]
		args, err := json.Marshal(rdb.CollocationsArgs{
			CorpusPath: corpusPath,
			SubcPath:   subc,
			Query:      q,
			Attr:       CollDefaultAttr,
			CollFn:     collFn,
			MinFreq:    2,
			MaxItems:   20,
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
			Func: "collocations",
			Args: args,
		})

		if err != nil {
			// TODO
			log.Error().Err(err).Msg("failed to publish query")
			wg.Done()

		} else {
			go func() {
				defer wg.Done()
				tmp := <-wait
				resultNext, err := rdb.DeserializeCollocationsResult(tmp)
				if err != nil {
					// TODO
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				if err := resultNext.Err(); err != nil {
					// TODO
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				mergedFreqLock.Lock()
				result.Add(resultNext.Colls)
				mergedFreqLock.Unlock()
			}()
		}
	}
	wg.Wait()
	resp := result.SortedByAvgScore()
	uniresp.WriteJSONResponse(
		ctx.Writer,
		resp,
	)
}
