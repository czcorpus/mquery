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

package edit

import (
	"encoding/json"
	"mquery/corpus"
	"mquery/rdb"
	"net/http"
	"sync"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type Actions struct {
	conf     *corpus.CorporaSetup
	radapter *rdb.Adapter
}

func (a *Actions) SplitCorpus(ctx *gin.Context) {
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	exists, err := SplitCorpusExists(a.conf.SplitCorporaDir, corpPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusConflict)
		return
	}
	if exists {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("split corpus already exists"), http.StatusConflict)
		return
	}

	corp, err := SplitCorpus(a.conf.SplitCorporaDir, corpPath, a.conf.MultiprocChunkSize)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusConflict)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(len(corp.Subcorpora))
	for _, subc := range corp.Subcorpora {
		args, err := json.Marshal(rdb.CalcCollFreqDataArgs{
			CorpusPath: corpPath,
			SubcPath:   subc,
			Attrs:      []string{"word", "lemma"},
		})
		if err != nil {
			// TODO
			log.Error().Err(err).Msg("failed to publish task")
		}
		wait, err := a.radapter.PublishQuery(rdb.Query{
			Func: "calcCollFreqData",
			Args: args,
		})
		go func() {
			ans := <-wait
			resp, err := rdb.DeserializeCollFreqDataResult(ans)
			if err != nil {
				// TODO
				log.Error().Err(err).Msg("failed to execute action calcCollFreqData")
			}
			if resp.Err() != nil {
				// TODO
				log.Error().Err(err).Msg("failed to execute action calcCollFreqData")
			}
			wg.Done()
		}()
	}
	wg.Wait()
	uniresp.WriteJSONResponse(ctx.Writer, corp)
}

func NewActions(conf *corpus.CorporaSetup, radapter *rdb.Adapter) *Actions {
	return &Actions{
		conf:     conf,
		radapter: radapter,
	}
}
