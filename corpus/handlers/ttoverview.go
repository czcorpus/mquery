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
	"encoding/json"
	"fmt"
	"mquery/corpus/baseinfo"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"strconv"
	"sync"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type ttOverviewResult struct {
	freqs map[string]results.FreqDistrib
	error string
}

func (tto *ttOverviewResult) set(attr baseinfo.TextProperty, v results.FreqDistrib) {
	tto.freqs[attr.String()] = v
}

func (tto *ttOverviewResult) findError() string {
	for _, v := range tto.freqs {
		if v.Error != "" {
			return v.Error
		}
	}
	return ""
}

func (tto *ttOverviewResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			Freqs      map[string]results.FreqDistrib `json:"freqs"`
			Error      string                         `json:"error,omitempty"`
			ResultType results.ResultType             `json:"resultType"`
		}{
			Freqs:      tto.freqs,
			ResultType: tto.Type(),
			Error:      tto.findError(),
		},
	)
}

func (tto *ttOverviewResult) Type() results.ResultType {
	return results.ResultTypeMultipleFreqs
}

func newTtOverviewResult() *ttOverviewResult {
	return &ttOverviewResult{
		freqs: make(map[string]results.FreqDistrib),
	}
}

// ----

func (a *Actions) TextTypesOverview(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}
	cConf := a.conf.Resources.Get(queryProps.corpus)
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
	corpusPath := a.conf.GetRegistryPath(queryProps.corpus)

	mergedFreqLock := sync.Mutex{}
	auxResult := newTtOverviewResult()
	textProps := queryProps.corpusConf.TextProperties.ListOverviewProps()
	errs := make([]error, 0, len(textProps))
	wg := sync.WaitGroup{}
	wg.Add(len(textProps))

	for _, attr := range textProps {
		freqArgs := rdb.FreqDistribArgs{
			CorpusPath:  corpusPath,
			Query:       queryProps.query,
			Crit:        fmt.Sprintf("%s 0", attr),
			IsTextTypes: true,
			FreqLimit:   flimit,
			MaxResults:  textTypesInternalMaxResults,
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
			errs = append(errs, err)
			log.Error().Err(err).Msg("failed to publish query")
			wg.Done()

		} else {
			go func(attrx baseinfo.TextProperty) {
				defer wg.Done()
				tmp := <-wait
				resultNext, err := rdb.DeserializeTextTypesResult(tmp)
				if err != nil {
					errs = append(errs, err)
					log.Error().Err(err).Msg("failed to deserialize query")
				}
				mergedFreqLock.Lock()
				auxResult.set(attrx, resultNext)
				mergedFreqLock.Unlock()
			}(attr)
		}
	}

	wg.Wait()

	result := make(map[string]results.FreqDistrib)
	for k, v := range auxResult.freqs {
		result[cConf.TextProperties.Prop(k).String()] = v
	}

	if len(errs) > 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(errs[0]), http.StatusInternalServerError)
		return
	}

	uniresp.WriteJSONResponse(ctx.Writer, &result)
}
