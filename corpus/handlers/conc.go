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
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/results"
	"net/http"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/gin-gonic/gin"
)

const (
	dfltMaxContext                = 50
	concFormatJSON     concFormat = "json"
	concFormatMarkdown concFormat = "markdown"
)

type concFormat string

func (cf concFormat) Validate() error {
	if cf == concFormatJSON || cf == concFormatMarkdown {
		return nil
	}
	return fmt.Errorf("unknown concordance format type: %s", cf)
}

type ConcArgsBuilder func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs

func mkStrongMarkdown(tk *concordance.Token, conf *corpus.CorpusSetup) string {
	var tmp strings.Builder
	if tk.Strong {
		tmp.WriteString(fmt.Sprintf("**%s** *{", tk.Word))
		var i int
		for _, v := range conf.PosAttrs {
			if v.Name == "word" {
				continue
			}
			if i > 0 {
				tmp.WriteString(", ")
			}
			tmp.WriteString(fmt.Sprintf("%s=%s", v.Name, tk.Attrs[v.Name]))
			i++
		}
		tmp.WriteString("}*")
		return tmp.String()
	}
	return tk.Word
}

func concToMarkdown(data results.Concordance, conf *corpus.CorpusSetup) string {
	var ans strings.Builder
	for _, line := range data.Lines {
		for i, ch := range line.Text {

			if i > 0 {
				ans.WriteString(" " + mkStrongMarkdown(ch, conf))

			} else {
				ans.WriteString(mkStrongMarkdown(ch, conf))
			}
		}
		ans.WriteString("\n\n")
	}
	return ans.String()
}

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
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        dfltMaxContext,
				ViewContextStruct: conf.ViewContextStruct,
			}
		},
	)
}

func (a *Actions) Concordance(ctx *gin.Context) {
	format := concFormat(ctx.DefaultQuery("format", "json"))
	if err := format.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusUnprocessableEntity,
		)
		return
	}
	a.anyConcordance(
		ctx,
		format,
		func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {
			return rdb.ConcordanceArgs{
				CorpusPath:        a.conf.GetRegistryPath(conf.ID),
				Query:             q,
				Attrs:             conf.PosAttrs.GetIDs(),
				ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
				StartLine:         0, // TODO
				MaxItems:          conf.MaximumRecords,
				MaxContext:        dfltMaxContext,
				ViewContextStruct: conf.ViewContextStruct,
			}
		},
	)
}

func (a *Actions) anyConcordance(ctx *gin.Context, format concFormat, argsBuilder ConcArgsBuilder) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	if queryProps.hasError() {
		uniresp.RespondWithErrorJSON(ctx, queryProps.err, queryProps.status)
		return
	}

	args, err := json.Marshal(argsBuilder(
		queryProps.corpusConf,
		queryProps.query,
	))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
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
	result, err := rdb.DeserializeConcordanceResult(rawResult)
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
	switch format {
	case concFormatJSON:
		uniresp.WriteJSONResponse(ctx.Writer, &result)
	case concFormatMarkdown:
		md := concToMarkdown(result, a.conf.Resources.Get(queryProps.corpus))
		ctx.Header("content-type", "text/markdown; charset=utf-8")
		ctx.Writer.WriteString(md)
	default:
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("invalid format: %s", format), http.StatusUnprocessableEntity)
	}
}

func (a *Actions) TermFrequency(ctx *gin.Context) {
	queryProps := DetermineQueryProps(ctx, a.conf)
	argsBuilder := func(conf *corpus.CorpusSetup, q string) rdb.ConcordanceArgs {
		return rdb.ConcordanceArgs{
			CorpusPath:        a.conf.GetRegistryPath(conf.ID),
			Query:             q,
			Attrs:             conf.PosAttrs.GetIDs(),
			ParentIdxAttr:     conf.SyntaxConcordance.ParentAttr,
			StartLine:         0, // TODO
			MaxItems:          1,
			MaxContext:        dfltMaxContext,
			ViewContextStruct: conf.ViewContextStruct,
		}
	}
	args, err := json.Marshal(argsBuilder(
		queryProps.corpusConf,
		queryProps.query,
	))
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionErrorFrom(err),
			http.StatusInternalServerError,
		)
		return
	}

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
	result, err := rdb.DeserializeConcSizeResult(rawResult)
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
	uniresp.WriteJSONResponse(ctx.Writer, &result)
}
