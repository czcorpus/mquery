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
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"net/http"
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
	conf     *corpus.CorporaSetup
	radapter *rdb.Adapter
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

	corp, err := splitCorpus(a.conf.SplitCorporaDir, corpPath, a.conf.MultiprocChunkSize)
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
			CorpusPath: corpPath,
			SubcPath:   subc,
			Attrs:      []string{"word", "lemma"},
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
			if resp.Err() != nil {
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
			CorpusPath: corpPath,
			SubcPath:   subc,
			Attrs:      []string{"word", "lemma"},
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
			wg.Done()
			ans := <-wait
			resp, err := rdb.DeserializeCollFreqDataResult(ans)
			if err != nil {
				errs = append(errs, err)
				log.Error().Err(err).Msg("failed to execute action calcCollFreqData")
			}
			if resp.Err() != nil {
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
					CorpusPath: corpPath,
					SubcPath:   subc,
					Attrs:      []string{attr},
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
					if resp.Err() != nil {
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

func NewActions(conf *corpus.CorporaSetup, radapter *rdb.Adapter) *Actions {
	return &Actions{
		conf:     conf,
		radapter: radapter,
	}
}
