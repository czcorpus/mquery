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
	"mquery/corpus"
	"mquery/corpus/edit"
	"mquery/corpus/infoload"
	"mquery/rdb"
	"net/http"
	"sync"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	DfltNumSamples                     = 30
	SplitCorpus    corpusStructVariant = "split"
)

type corpusStructVariant string

func (variant corpusStructVariant) Validate() bool {
	return variant == SplitCorpus
}

type multiSubcCorpus interface {
	GetSubcorpora() []string
}

type Actions struct {
	conf         *corpus.CorporaSetup
	radapter     *rdb.Adapter
	infoProvider infoload.Provider
	dfltLanguage string
}

func (a *Actions) DeleteSplit(ctx *gin.Context) {
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	exists, err := edit.SplitCorpusExists(a.conf.SplitCorporaDir, corpPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusConflict)
		return
	}
	if !exists {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("split does not exist"), http.StatusNotFound)
		return
	}
	err = edit.DeleteSplit(a.conf.SplitCorporaDir, corpPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"ok": true})

}

func (a *Actions) SplitCorpus(ctx *gin.Context) {
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	exists, err := edit.SplitCorpusExists(a.conf.SplitCorporaDir, corpPath)
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

	chunkSize, ok := unireq.GetURLIntArgOrFail(ctx, "chunkSize", int(a.conf.MultiprocChunkSize))
	if !ok {
		return
	}

	// note: `splitCorpus` is very fast so there is no need to delegate it to a worker
	corp, err := edit.SplitCorpus(a.conf.SplitCorporaDir, corpPath, int64(chunkSize))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusConflict)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(len(corp.Subcorpora))
	errs := make([]error, 0, len(corp.Subcorpora))
	for _, subc := range corp.Subcorpora {
		args, err := json.Marshal(rdb.CalcCollFreqDataArgs{
			CorpusPath:     corpPath,
			SubcPath:       subc,
			Attrs:          []string{"word", "lemma"}, // TODO this should not be hardcoded
			Structs:        []string{"doc"},
			MktokencovPath: a.conf.MktokencovPath,
		})
		if err != nil {
			wg.Done()
			log.Error().Err(err).Msg("failed to publish task")
			errs = append(errs, err)
			continue
		}
		wait, err := a.radapter.PublishQuery(rdb.Query{
			Func: "calcCollFreqData",
			Args: args,
		})
		go func() {
			defer wg.Done()
			ans := <-wait
			resp, err := rdb.DeserializeCollFreqDataResult(ans)
			if err != nil {
				errs = append(errs, err)
				log.Error().Err(err).Msg("failed to execute action calcCollFreqData")
			}
			if err := resp.Err(); err != nil {
				errs = append(errs, err)
				log.Error().Err(err).Msg("failed to execute action calcCollFreqData")
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(errs[0]), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, corp)
}

func (a *Actions) CorpusInfo(ctx *gin.Context) {
	info, err := a.infoProvider.LoadCorpusInfo(ctx.Param("corpusId"), ctx.DefaultQuery("lang", a.dfltLanguage))
	if err == corpus.ErrNotFound {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, info)
}
