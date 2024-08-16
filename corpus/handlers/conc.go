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
	"fmt"
	"mquery/corpus"
	"mquery/corpus/transform"
	"mquery/rdb"
	"mquery/rdb/results"
	"net/http"
	"strconv"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	ConcordanceMaxWidth                = 50
	ConcordanceDefaultWidth            = 10
	termFreqContext                    = 5
	concFormatJSON          concFormat = "json"
	concFormatMarkdown      concFormat = "markdown"
)

type concFormat string

func (cf concFormat) Validate() error {
	if cf == concFormatJSON || cf == concFormatMarkdown {
		return nil
	}
	return fmt.Errorf("unknown concordance format type: %s", cf)
}

type ConcArgsBuilder func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs

type ConcArgsValidator func(args *rdb.ConcordanceArgs) error

func (a *Actions) SyntaxConcordance(ctx *gin.Context) {
	a.anyConcordance(
		ctx,
		concFormatJSON,
		func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {

			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(conf.ID),
				QueryLemma:        ctx.Query("lemma"),
				Query:             q,
				Attrs:             conf.SyntaxConcordance.ResultAttrs,
				ShowRefs:          []string{},
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        ConcordanceMaxWidth,
				ViewContextStruct: conf.ViewContextStruct,
			}
		},
		func(args *rdb.ConcordanceArgs) error {
			if args.ViewContextStruct == "" {
				return fmt.Errorf("sentence structure is not defined for the corpus")
			}
			return nil
		},
	)
}

func (a *Actions) Concordance(ctx *gin.Context) {
	format := concFormat(ctx.DefaultQuery("format", "json"))
	if err := format.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
		return
	}
	contextWidth := ConcordanceDefaultWidth
	sContextWidth := ctx.Query("contextWidth")
	if sContextWidth != "" {
		var err error
		contextWidth, err = strconv.Atoi(sContextWidth)
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				err,
				http.StatusBadRequest,
			)
			return
		}
	}
	if contextWidth > ConcordanceMaxWidth {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid contextWidth - max value is %d", ConcordanceMaxWidth),
			http.StatusBadRequest,
		)
		return
	}
	a.anyConcordance(
		ctx,
		format,
		func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {
			showStructs := []string{}
			if ctx.Query("showMarkup") == "1" {
				showStructs = conf.ConcMarkupStructures
			}
			showRefs := []string{}
			if ctx.Query("showTextProps") == "1" {
				showRefs = conf.ConcTextPropsAttrs
			}
			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(conf.ID),
				Query:             q,
				Attrs:             conf.PosAttrs.GetIDs(),
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				ShowStructs:       showStructs,
				ShowRefs:          showRefs,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        contextWidth,
				ViewContextStruct: "",
			}
		},
		func(args *rdb.ConcordanceArgs) error { return nil },
	)
}

func (a *Actions) Sentences(ctx *gin.Context) {
	format := concFormat(ctx.DefaultQuery("format", "json"))
	if err := format.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusBadRequest,
		)
		return
	}
	a.anyConcordance(
		ctx,
		format,
		func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {
			showStructs := []string{}
			if ctx.Query("showMarkup") == "1" {
				showStructs = conf.ConcMarkupStructures
			}
			showRefs := []string{}
			if ctx.Query("showTextProps") == "1" {
				showRefs = conf.ConcTextPropsAttrs
			}
			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(conf.ID),
				Query:             q,
				Attrs:             conf.PosAttrs.GetIDs(),
				ShowStructs:       showStructs,
				ShowRefs:          showRefs,
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        ConcordanceMaxWidth,
				ViewContextStruct: conf.ViewContextStruct,
			}
		},
		func(args *rdb.ConcordanceArgs) error {
			if args.ViewContextStruct == "" {
				return fmt.Errorf("sentence structure is not defined for the corpus")
			}
			return nil
		},
	)
}

func (a *Actions) anyConcordance(
	ctx *gin.Context,
	format concFormat,
	argsBuilder ConcArgsBuilder,
	validator ConcArgsValidator,

) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}
	args := argsBuilder(
		queryProps.corpusConf,
		queryProps.query,
	)
	if err := validator(&args); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "concordance",
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
	if ok := HandleWorkerError(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondError[results.Concordance](ctx, rawResult)
	if !ok {
		return
	}
	corpus.ApplyTextPropertiesMapping(result, queryProps.corpusConf.TextProperties)

	switch format {
	case concFormatJSON:
		uniresp.WriteJSONResponse(ctx.Writer, &result)
	case concFormatMarkdown:
		md := transform.ConcToMarkdown(
			&result,
			a.conf.Resources.Get(queryProps.corpus),
			len(args.ShowRefs) > 0,
		)
		ctx.Header("content-type", "text/markdown; charset=utf-8")
		ctx.Writer.WriteString(md)
	default:
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("invalid format: %s", format), http.StatusUnprocessableEntity)
	}
}

func (a *Actions) TermFrequency(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	argsBuilder := func(conf *corpus.CorpusSetup, q string) rdb.TermFrequencyArgs {
		return rdb.TermFrequencyArgs{
			CorpusPath:        a.conf.GetRegistryPath(conf.ID),
			Query:             q,
			Attrs:             conf.PosAttrs.GetIDs(),
			ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
			StartLine:         0, // TODO
			MaxItems:          1,
			MaxContext:        termFreqContext,
			ViewContextStruct: conf.ViewContextStruct,
		}
	}
	args := argsBuilder(
		queryProps.corpusConf,
		queryProps.query,
	)

	wait, err := a.radapter.PublishQuery(rdb.Query{
		Func: "termFrequency",
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
	if ok := HandleWorkerError(ctx, rawResult); !ok {
		return
	}
	result, ok := TypedOrRespondError[results.ConcSize](ctx, rawResult)
	if !ok {
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &result)
}
