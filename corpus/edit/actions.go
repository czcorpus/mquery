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
	"database/sql"
	"encoding/json"
	"fmt"
	"mquery/corpus"
	"mquery/engine"
	"mquery/mango"
	"mquery/rdb"
	"net/http"
	"strings"
	"sync"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	DfltNumSamples                         = 30
	SplitCorpus        corpusStructVariant = "split"
	MultisampledCorpus corpusStructVariant = "multisampled"
)

type corpusStructVariant string

func (variant corpusStructVariant) Validate() bool {
	return variant == SplitCorpus || variant == MultisampledCorpus
}

type multiSubcCorpus interface {
	GetSubcorpora() []string
}

type Actions struct {
	conf        *corpus.CorporaSetup
	radapter    *rdb.Adapter
	db          *sql.DB
	language    string
	corpusTable string
}

func (a *Actions) DeleteSplit(ctx *gin.Context) {
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	exists, err := SplitCorpusExists(a.conf.SplitCorporaDir, corpPath)
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
	err = DeleteSplit(a.conf.SplitCorporaDir, corpPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"ok": true})

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

	chunkSize, ok := unireq.GetURLIntArgOrFail(ctx, "chunkSize", int(a.conf.MultiprocChunkSize))
	if !ok {
		return
	}

	// note: `splitCorpus` is very fast so there is no need to delegate it to a worker
	corp, err := splitCorpus(a.conf.SplitCorporaDir, corpPath, int64(chunkSize))
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

func (a *Actions) MultiSample(ctx *gin.Context) {
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	exists, err := MultisampleCorpusExists(a.conf.MultisampledCorporaDir, corpPath)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusConflict)
		return
	}
	if exists {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("multisampled corpus already exists"), http.StatusConflict)
		return
	}
	numSamples, ok := unireq.GetURLIntArgOrFail(ctx, "numSamples", DfltNumSamples)
	if !ok {
		return
	}
	corp, err := MultisampleCorpus(
		a.conf.MultisampledCorporaDir, corpPath, a.conf.MultisampledSubcSize, numSamples)
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
			Attrs:          []string{"word", "lemma"}, // TODO hardcoded stuff
			Structs:        []string{"doc"},           // TODO hardcoded stuff
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

// CollFreqData
// TODO add support for token coverage
func (a *Actions) CollFreqData(ctx *gin.Context) {
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	variant := corpusStructVariant(ctx.Param("variant"))
	if !variant.Validate() {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(
				fmt.Sprintf("invalid corpus composition variant: %s", variant),
			),
			http.StatusUnprocessableEntity,
		)
		return
	}
	var multicorp multiSubcCorpus
	var err error
	if variant == SplitCorpus {
		multicorp, err = corpus.OpenSplitCorpus(a.conf.SplitCorporaDir, corpPath)

	} else if variant == MultisampledCorpus {
		multicorp, err = corpus.OpenMultisampledCorpus(a.conf.MultisampledCorporaDir, corpPath)

	} else {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("invalid corpus structure type specified: %s", variant),
			http.StatusUnprocessableEntity,
		)
		return
	}
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusConflict)
		return
	}
	wg := sync.WaitGroup{}
	errs := make([]error, 0, len(multicorp.GetSubcorpora()))
	for _, subc := range multicorp.GetSubcorpora() {
		for _, attr := range []string{"word", "lemma"} {
			exists, err := CollFreqDataExists(subc, attr)
			if err != nil {
				errs = append(errs, err)
				log.Error().Err(err).Msg("failed to determine freq file existence")
				continue

			} else if !exists {
				wg.Add(1)
				args, err := json.Marshal(rdb.CalcCollFreqDataArgs{
					CorpusPath:     corpPath,
					SubcPath:       subc,
					Attrs:          []string{attr},
					MktokencovPath: a.conf.MktokencovPath,
				})
				if err != nil {
					errs = append(errs, err)
					log.Error().Err(err).Msg("failed to publish task")
					wg.Done()
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
		}
	}
	wg.Wait()
	if len(errs) > 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(errs[0]), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, multicorp)
}

func (a *Actions) CorpusInfo(ctx *gin.Context) {
	kdb := engine.NewKontextDatabase(a.db, a.corpusTable, a.language)
	info, err := kdb.LoadCorpusInfo(ctx.Param("corpusId"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	corpPath := a.conf.GetRegistryPath(ctx.Param("corpusId"))
	attrs, err := mango.GetCorpusConf(corpPath, "ATTRLIST")
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	for _, v := range strings.Split(attrs, ",") {
		info.AttrList = append(info.AttrList, engine.Item{
			Name: v,
			Size: 0,
		})
	}
	structs, err := mango.GetCorpusConf(corpPath, "STRUCTLIST")
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	for _, v := range strings.Split(structs, ",") {
		info.StructList = append(info.StructList, engine.Item{
			Name: v,
			Size: 0,
		})
	}

	uniresp.WriteJSONResponse(ctx.Writer, info)
}

func NewActions(conf *corpus.CorporaSetup, radapter *rdb.Adapter, sqlDB *sql.DB, corpusTable string, language string) *Actions {
	return &Actions{
		conf:        conf,
		radapter:    radapter,
		db:          sqlDB,
		corpusTable: corpusTable,
		language:    language,
	}
}
