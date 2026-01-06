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
	"strings"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/cnc-gokit/util"
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

type ConcArgsBuilder func(queryProps queryProps) rdb.ConcordanceArgs

type ConcArgsValidator func(args *rdb.ConcordanceArgs) error

func (a *Actions) SyntaxConcordance(ctx *gin.Context) {
	a.anyConcordance(
		ctx,
		concFormatJSON,
		func(queryProps queryProps) rdb.ConcordanceArgs {

			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(queryProps.corpusConf.ID),
				QueryLemma:        ctx.Query("lemma"),
				Query:             queryProps.query,
				Attrs:             queryProps.corpusConf.SyntaxConcordance.ResultAttrs,
				ShowRefs:          []string{},
				ParentIdxAttr:     queryProps.corpusConf.SyntaxConcordance.ParentAttr,
				RowsOffset:        0, // TODO
				MaxItems:          queryProps.corpusConf.MaximumRecords,
				MaxContext:        ConcordanceMaxWidth,
				ViewContextStruct: queryProps.corpusConf.ViewContextStruct,
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

// Concordance godoc
// @Summary      Concordance
// @Description  Search in a corpus for concordances
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "The translated query"
// @Param        subcorpus query string false "An ID of a subcorpus"
// @Param        format query string false "For a concordance formatted in Markdown, `markdown` value can be passed" Enums(json,markdown) default(json)
// @Param        showMarkup query int false "if 1, then markup specifying formatting and structure of text will be displayed along with tokens" enums(0,1) default(0)
// @Param        showTextProps query int false "if 1, then text metadata (e.g. author, publication year) will be attached to each line" enums(0,1) default(0)
// @Param        contextWidth query int false "Defines number of tokens around KWIC. For a value K, the left context is floor(K / 2) and for the right context, it is ceil(K / 2)." minimum(0) maximum(50) default(10)
// @Param        contextStruct query string false "By default, tokens are used for specifying context window. Setting this value will change the units to structs (typically a sentence) "
// @Param        rowsOffset query int false "Take results starting from this row number (first row = 0)"
// @Param        maxRows query int false "Max. number of concordance lines to return. Default is corpus-dependent but mostly around 50"
// @Param        coll query string false "Optional collocate query (CQL)"
// @Param        collRange query string false "Specifies where to search the collocate. I.e. this only applies if the `coll` is filled. Format: left,right where negative numbers are on the left side of the KWIC."
// @Success      200 {object} results.ConcordanceResponse
// @Success      200 {string} text/markdown
// @Router       /concordance/{corpusId} [get]
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

	contextWidth, ok := unireq.GetURLIntArgOrFail(ctx, "contextWidth", ConcordanceDefaultWidth)
	if !ok {
		return
	}
	if contextWidth > ConcordanceMaxWidth {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid contextWidth - max value is %d", ConcordanceMaxWidth),
			http.StatusBadRequest,
		)
		return
	}

	maxRows, ok := unireq.GetURLIntArgOrFail(ctx, "maxRows", 0) // default will be added later below
	if !ok {
		return
	}

	rowsOffset, ok := unireq.GetURLIntArgOrFail(ctx, "rowsOffset", 0)
	if !ok {
		return
	}

	var collLftCtx, collRgtCtx int
	collQuery := ctx.Request.URL.Query().Get("coll")
	rng := ctx.Request.URL.Query().Get("collRange")
	if collQuery != "" && rng != "" {
		var err error
		rngItems := strings.Split(rng, ",")
		if len(rngItems) != 2 {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("invalid collocate range format (should be 'left,right')"),
				http.StatusBadRequest,
			)
			return
		}
		collLftCtx, err = strconv.Atoi(rngItems[0])
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("invalid collocate left range value %s: %w", rngItems[0], err),
				http.StatusBadRequest,
			)
			return
		}
		collRgtCtx, err = strconv.Atoi(rngItems[1])
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("invalid collocate right range value %s: %w", rngItems[1], err),
				http.StatusBadRequest,
			)
			return
		}
	}

	a.anyConcordance(
		ctx,
		format,
		func(queryProps queryProps) rdb.ConcordanceArgs {
			showStructs := []string{}
			if ctx.Query("showMarkup") == "1" {
				showStructs = queryProps.corpusConf.ConcMarkupStructures
			}
			showRefs := []string{}
			if ctx.Query("showTextProps") == "1" {
				showRefs = queryProps.corpusConf.ConcTextPropsAttrs
			}
			contextStruct := ctx.DefaultQuery("contextStruct", queryProps.corpusConf.ViewContextStruct)

			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(queryProps.corpusConf.ID),
				SubcPath:          queryProps.savedSubcorpus,
				Query:             queryProps.query,
				CollQuery:         collQuery,
				CollLftCtx:        collLftCtx,
				CollRgtCtx:        collRgtCtx,
				Attrs:             queryProps.corpusConf.PosAttrs.GetIDs(),
				ParentIdxAttr:     queryProps.corpusConf.SyntaxConcordance.ParentAttr,
				ShowStructs:       showStructs,
				ShowRefs:          showRefs,
				MaxItems:          util.Ternary(maxRows > 0, maxRows, queryProps.corpusConf.MaximumRecords),
				RowsOffset:        rowsOffset,
				MaxContext:        contextWidth,
				ViewContextStruct: contextStruct,
			}
		},
		func(args *rdb.ConcordanceArgs) error { return nil },
	)
}

// Sentences godoc
// @Summary      Sentences
// @Description  Search in a corpus for matching sentences. This is an alternative to the /concordance/{corpusId} endpoint.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "The translated query"
// @Param        subcorpus query string false "An ID of a subcorpus"
// @Param        format query string false "For a concordance formatted in Markdown, `markdown` value can be passed" enums(json,markdown) default(json)
// @Param        showMarkup query int false "if 1, then markup specifying formatting and structure of text will be displayed along with tokens" enums(0,1) default(0)
// @Param        showTextProps query int false "if 1, then text metadata (e.g. author, publication year) will be attached to each line" enums(0,1) default(0)
// @Success      200 {object} results.ConcordanceResponse
// @Success      200 {string} text/markdown
// @Router       /sentences/{corpusId} [get]
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
		func(queryProps queryProps) rdb.ConcordanceArgs {
			showStructs := []string{}
			if ctx.Query("showMarkup") == "1" {
				showStructs = queryProps.corpusConf.ConcMarkupStructures
			}
			showRefs := []string{}
			if ctx.Query("showTextProps") == "1" {
				showRefs = queryProps.corpusConf.ConcTextPropsAttrs
			}
			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(queryProps.corpusConf.ID),
				SubcPath:          queryProps.savedSubcorpus,
				Query:             queryProps.query,
				Attrs:             queryProps.corpusConf.PosAttrs.GetIDs(),
				ShowStructs:       showStructs,
				ShowRefs:          showRefs,
				ParentIdxAttr:     queryProps.corpusConf.SyntaxConcordance.ParentAttr,
				RowsOffset:        0, // TODO
				MaxItems:          queryProps.corpusConf.MaximumRecords,
				MaxContext:        ConcordanceMaxWidth,
				ViewContextStruct: queryProps.corpusConf.ViewContextStruct,
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
	args := argsBuilder(queryProps)
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

// TermFrequency godoc
// @Summary      TermFrequency
// @Description  This endpoint retrieves the frequency, instances per million (IPM), and Average Reduced Frequency (ARF) of a searched term within a corpus. It provides a concise aggregated frequency overview for a given query, regardless of the number of concrete words (n-grams) it covers.
// @Produce      json
// @Param        corpusId path string true "An ID of a corpus to search in"
// @Param        q query string true "The translated query"
// @Param        subcorpus query string false "An ID of a subcorpus"
// @Success      200 {object} results.ConcSizeResponse
// @Router       /term-frequency/{corpusId} [get]
func (a *Actions) TermFrequency(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	argsBuilder := func(conf *corpus.MQCorpusSetup, q string) rdb.TermFrequencyArgs {
		return rdb.TermFrequencyArgs{
			CorpusPath:        a.conf.GetRegistryPath(conf.ID),
			Query:             q,
			Attrs:             conf.PosAttrs.GetIDs(),
			ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
			RowsOffset:        0, // TODO
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
