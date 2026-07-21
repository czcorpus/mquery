// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Deparment of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func requireCorpusID(request mcp.CallToolRequest) (string, *mcp.CallToolResult) {
	corpusID := request.GetString("corpus_id", "")
	if corpusID == "" {
		return "", mcp.NewToolResultError("missing corpus_id argument")
	}
	return corpusID, nil
}

func CreateCorpInfoTool(srv *server.MCPServer, conf *Conf) {
	t := mcp.NewTool("corpus_info",
		mcp.WithDescription("Get information about a corpus, including important information about its structure required for proper CQL queries"),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to get info about")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "info", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run info: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateTermSrchTool(srv *server.MCPServer, conf *Conf) {
	t := mcp.NewTool("term_frequency",
		mcp.WithDescription("Retrieve frequency, instances per million (IPM), and Average Reduced Frequency (ARF) of a searched term within a corpus. The result is for all the matching entries given the query, regardless of the number of concrete matching words (n-grams)."),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithString("subcorpus", mcp.Description("Optional ID of a subcorpus")),
		mcp.WithString("q", mcp.Required(), mcp.Description("CQL query string")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "term-frequency", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run term-frequency: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{
					"subcorpus": request.GetString("subcorpus", ""),
					"q":         request.GetString("q", ""),
				},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateFreqsTool(srv *server.MCPServer, conf *Conf) {

	defaultAttr := "lemma"
	defaultMaxItems := 20
	defaultFlimit := 1

	t := mcp.NewTool("freqs",
		mcp.WithDescription("Calculate a frequency distribution of the first word of matching KWICs."),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithString("subcorpus", mcp.Description("Optional ID of a subcorpus")),
		mcp.WithString("q", mcp.Required(), mcp.Description("CQL query string")),
		mcp.WithString("attr", mcp.Description("a positional attribute (e.g. `word`, `lemma`, `tag`) the frequency will be calculated on"), mcp.DefaultString(defaultAttr)),
		mcp.WithBoolean("match_case", mcp.Description("if true then words with the same letters but different letter cases will be treated separately")),
		mcp.WithInteger("max_items", mcp.Description("maximum number of result items"), mcp.DefaultNumber(defaultMaxItems)),
		mcp.WithInteger("flimit", mcp.Description("minimum frequency of result items to be included in the result set"), mcp.DefaultNumber(defaultFlimit)),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "freqs", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run freqs: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{
					"subcorpus": request.GetString("subcorpus", ""),
					"q":         request.GetString("q", ""),
					"attr":      request.GetString("attr", defaultAttr),
					"matchCase": request.GetBool("match_case", false),
					"maxItems":  request.GetInt("max_items", defaultMaxItems),
					"flimit":    request.GetInt("flimit", defaultFlimit),
				},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateTextTypesTool(srv *server.MCPServer, conf *Conf) {

	defaultMaxItems := 20
	defaultFlimit := 1

	t := mcp.NewTool("text_types",
		mcp.WithDescription("Calculates frequencies of all the values of a requested structural attribute found in structures matching required query (e.g. all the authors via doc.author)"),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithString("subcorpus", mcp.Description("Optional ID of a subcorpus")),
		mcp.WithString("attr", mcp.Required(), mcp.Description("a structural attribute the frequencies will be calculated for (e.g. `doc.pubyear`, `text.author`,...)")),
		mcp.WithInteger("max_items", mcp.Description("maximum number of result items"), mcp.DefaultNumber(defaultMaxItems)),
		mcp.WithInteger("flimit", mcp.Description("minimum frequency of result items to be included in the result set"), mcp.DefaultNumber(defaultFlimit)),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "text-types", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run freqs: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{
					"subcorpus": request.GetString("subcorpus", ""),
					"q":         request.GetString("q", ""),
					"attr":      request.GetString("attr", ""),
					"maxItems":  request.GetInt("max_items", defaultMaxItems),
					"flimit":    request.GetInt("flimit", defaultFlimit),
				},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateTextTypesOverviewTool(srv *server.MCPServer, conf *Conf) {

	defaultFlimit := 1

	t := mcp.NewTool("text_types_overview",
		mcp.WithDescription("Shows the text types (= values of predefined structural attributes) of a searched term. This tool provides a similar result to the `text_types` called multiple times on a fixed set of attributes (typically: publication years, authors, text types, media"),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithString("subcorpus", mcp.Description("Optional ID of a subcorpus")),
		mcp.WithString("q", mcp.Required(), mcp.Description("CQL query string")),
		mcp.WithInteger("flimit", mcp.Description("minimum frequency of result items to be included in the result set"), mcp.DefaultNumber(1)),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "text-types-overview", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run freqs: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{
					"subcorpus": request.GetString("subcorpus", ""),
					"q":         request.GetString("q", ""),
					"flimit":    request.GetInt("flimit", defaultFlimit),
				},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateCollocationsTool(srv *server.MCPServer, conf *Conf) {

	defaultMeasure := "logDice"
	defaultSrchLeft := 5
	defaultSrchRight := 5
	defaultSrchAttr := "lemma"
	defaultMinCollFreq := 3
	defaultMaxItems := 20

	t := mcp.NewTool("collocations",
		mcp.WithDescription("Calculate a defined collocation profile of a searched expression. Values are sorted in descending order by their collocation score"),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithString("subcorpus", mcp.Description("Optional ID of a subcorpus")),
		mcp.WithString("q", mcp.Required(), mcp.Description("CQL query string")),
		mcp.WithString("measure", mcp.Description(""), mcp.Enum("absFreq", "logLikelihood", "logDice", "minSensitivity", "mutualInfo", "mutualInfo3", "mutualInfoLogF", "relFreq", "tScore"), mcp.DefaultString(defaultMeasure)),
		mcp.WithInteger("srch_left", mcp.Description("left range for candidates searching; values must be greater or equal to 1 (1 stands for words right before the searched term)"), mcp.DefaultNumber(defaultSrchLeft)),
		mcp.WithInteger("srch_right", mcp.Description("right range for candidates searching; values must be greater or equal to 1 (1 stands for words right after the searched term)"), mcp.DefaultNumber(defaultSrchRight)),
		mcp.WithString("srch_attr", mcp.Description("a positional attribute considered when collocations are calculated"), mcp.DefaultString(defaultSrchAttr)),
		mcp.WithInteger("min_coll_freq", mcp.Description("the minimum frequency that a collocate must have in the searched range."), mcp.DefaultNumber(defaultMinCollFreq)),
		mcp.WithInteger("max_items", mcp.Description("maximum number of result items"), mcp.DefaultNumber(defaultMaxItems)),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "collocations", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run freqs: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{
					"subcorpus":   request.GetString("subcorpus", ""),
					"q":           request.GetString("q", ""),
					"measure":     request.GetString("measure", defaultMeasure),
					"srchLeft":    request.GetInt("srch_left", defaultSrchLeft),
					"srchRight":   request.GetInt("srch_right", defaultSrchRight),
					"srchAttr":    request.GetString("srch_attr", defaultSrchAttr),
					"minCollFreq": request.GetInt("min_coll_freq", defaultMinCollFreq),
					"maxItems":    request.GetInt("max_items", defaultMaxItems),
				},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateConcordanceTool(srv *server.MCPServer, conf *Conf) {

	defaultFormat := "json"
	defaultShowMarkup := false
	defaultTextPropsVerbosity := 0
	defaultContextWidth := 10
	defaultRowsOffset := 0
	defaultMaxRows := 100

	t := mcp.NewTool("concordance",
		mcp.WithDescription("Calculate a defined collocation profile of a searched expression. Values are sorted in descending order by their collocation score"),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithString("subcorpus", mcp.Description("Optional ID of a subcorpus")),
		mcp.WithString("q", mcp.Required(), mcp.Description("CQL query string")),
		mcp.WithString("format", mcp.Description("Set output format"), mcp.Enum("json", "markdown"), mcp.DefaultString(defaultFormat)),
		mcp.WithBoolean("show_markup", mcp.Description("if true, then markup specifying formatting and structure of text will be displayed along with tokens"), mcp.DefaultBool(defaultShowMarkup)),
		mcp.WithInteger("text_props_verbosity", mcp.Description("if 1, then basic text metadata (e.g. author, publication year) will be attached to each line. Value 2 shows all the available attributes"), mcp.Min(0), mcp.Max(2), mcp.DefaultNumber(defaultTextPropsVerbosity)),
		mcp.WithInteger("context_width", mcp.Description("Defines number of tokens around KWIC. For a value K, the left context is floor(K / 2) and for the right context, it is ceil(K / 2)."), mcp.Min(0), mcp.Max(50), mcp.DefaultNumber(defaultContextWidth)),
		mcp.WithString("context_struct", mcp.Description("By default, tokens are used for specifying context window. Setting this value will change the units to structs (typically a sentence)")),
		mcp.WithInteger("rows_offset", mcp.Description("Take results starting from this row number (first row = 0)"), mcp.DefaultNumber(defaultRowsOffset)),
		mcp.WithInteger("max_rows", mcp.Description("Max. number of concordance lines to return. Default is corpus-dependent but the API specifies 100 as a fallback limit")),
		mcp.WithString("coll", mcp.Description("Optional collocate query (CQL)")),
		mcp.WithString("coll_range", mcp.Description("Specifies where to search the collocate. I.e. this only applies if the `coll` is filled. Format: left,right where negative numbers are on the left side of the KWIC.")),
		mcp.WithBoolean("no_shuffle", mcp.Description("if true, then the order of matches will be the same as in the source corpus")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "concordance", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run freqs: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				map[string]any{
					"subcorpus":          request.GetString("subcorpus", ""),
					"q":                  request.GetString("q", ""),
					"format":             request.GetString("format", defaultFormat),
					"showMarkup":         request.GetBool("show_markup", defaultShowMarkup),
					"textPropsVerbosity": request.GetInt("text_props_verbosity", defaultTextPropsVerbosity),
					"contextWidth":       request.GetInt("context_width", defaultContextWidth),
					"contextStruct":      request.GetString("context_struct", ""),
					"rowsOffset":         request.GetInt("rows_offset", defaultRowsOffset),
					"maxRows":            request.GetInt("max_rows", defaultMaxRows),
					"coll":               request.GetString("coll", ""),
					"collRange":          request.GetString("coll_range", ""),
					"noShuffle":          request.GetBool("no_shuffle", false),
				},
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}

func CreateTextTypesAvailValuesTool(srv *server.MCPServer, conf *Conf) {

	t := mcp.NewTool("text_types_avail_values",
		mcp.WithDescription("Get all the values for all the reasonably small structural attributes (typically - media types, orig. language, author gender, text category, publication year) "),
		mcp.WithString("corpus_id", mcp.Required(), mcp.Description("An ID of a corpus to search in")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	srv.AddTool(
		t,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			corpusID, errResult := requireCorpusID(request)
			if errResult != nil {
				return errResult, nil
			}
			url, err := JoinURL(conf.APIUrl, "text-types-avail-values", corpusID)
			if err != nil {
				return nil, fmt.Errorf("failed to run text-types-avail-values: %w", err)
			}
			ans, err2 := httpRequest(
				ctx,
				"GET",
				url,
				nil,
				conf.APIHeaders,
			)
			if err2.IsSoftError() {
				return mcp.NewToolResultErrorFromErr("action failed", err2), nil
			}
			if err2.IsHardError() {
				return nil, err2
			}
			return mcp.NewToolResultText(ans), nil
		},
	)
}
