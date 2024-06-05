// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Institute of the Czech National Corpus,
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

package openapi

import (
	"fmt"
	"mquery/corpus/handlers"

	"github.com/czcorpus/cnc-gokit/collections"
)

func NewResponse(ver, url, subscriber string) *APIResponse {
	paths := make(map[string]Methods)

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/corplist"] = Methods{
			Get: &Method{
				Description: "Shows a list of available corpora with their basic properties.",
				OperationID: "Corplist",
				Parameters: []Parameter{
					{
						Name:        "locale",
						In:          "query",
						Description: "An ISO 639-1 locale code of response.",
						Required:    false,
						Schema: ParamSchema{
							Type:    "string",
							Default: "en",
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/Corplist",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/info/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Shows a detailed corpus information, including size in tokens, available positional and structural attributes.",
				OperationID: "CorpusInfo",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to get info about",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "locale",
						In:          "query",
						Description: "An ISO 639-1 locale code of response.",
						Required:    false,
						Schema: ParamSchema{
							Type:    "string",
							Default: "en",
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/Info",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/concordance/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Search in a corpus for concordances",
				OperationID: "Concordance",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "format",
						In:          "query",
						Description: "For a concordance formatted in Markdown, `markdown` value can be passed",
						Required:    false,
						Schema: ParamSchema{
							Type:    "string",
							Enum:    []any{"json", "markdown"},
							Default: "json",
						},
					},
					{
						Name: "contextWidth",
						In:   "query",
						Description: "Defines number of tokens around KWIC. For a value K, the left context is " +
							"floor(K / 2) and for the right context, it is ceil(K / 2). The maximum value is " +
							fmt.Sprintf("%d", handlers.ConcordanceMaxWidth),
						Required: false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.ConcordanceDefaultWidth,
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/Concordance",
								},
							},
							"text/markdown; charset=utf-8": MethodResponseContent{
								Schema: MethodResponseSchema{
									Type: "string",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/sentences/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Search in a corpus for matching sentences. This is an alternative to the /concordance/{corpusId} endpoint.",
				OperationID: "Sentences",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "format",
						In:          "query",
						Description: "For sentences formatted in Markdown, `markdown` value can be passed",
						Required:    false,
						Schema: ParamSchema{
							Type:    "string",
							Enum:    []any{"json", "markdown"},
							Default: "json",
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/Sentences",
								},
							},
							"text/markdown; charset=utf-8": MethodResponseContent{
								Schema: MethodResponseSchema{
									Type: "string",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/text-types/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Calculates frequencies of all the values of a requested structural attribute found in structures matching required query (e.g. all the authors found in &lt;doc author=\"...\"&gt;)",
				OperationID: "TextTypes",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "attr",
						In:          "query",
						Description: "a structural attribute the frequencies will be calculated for (e.g. `doc.pubyear`, `text.author`,...)",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/TextTypes",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/text-types-overview/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Shows the text types (= values of predefined structural attributes) of a searched term. " +
					"This endpoint provides a similar result to the endpoint `/text-types/{corpusId}` called multiple times " +
					"on a fixed set of attributes (typically: publication years, authors, text types, media)",
				OperationID: "TTOverview",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "flimit",
						In:          "query",
						Description: "minimum frequency of result items to be included in the result set",
						Required:    false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.DefaultFreqLimit,
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/TextTypesOverview",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/term-frequency/{corpusId}"] = Methods{
			Get: &Method{
				OperationID: "TermFrequency",
				Description: "This endpoint retrieves the frequency, instances per million (IPM), and " +
					"Average Reduced Frequency (ARF) of a searched term within a corpus. It provides a concise " +
					"aggregated frequency overview for a given query, regardless of the number of concrete words " +
					" (n-grams) it covers. ",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/TermFrequency",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/freqs/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Calculate a frequency distribution for a searched term (KWIC).",
				OperationID: "Freqs",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "attr",
						In:          "query",
						Description: "a positional attribute (e.g. `word`, `lemma`, `tag`) the frequency will be calculated on",
						Required:    false,
						Schema: ParamSchema{
							Type:    "string",
							Default: handlers.DefaultFreqAttr,
						},
					},
					{
						Name:        "matchCase",
						In:          "query",
						Description: "",
						Schema: ParamSchema{
							Type: "integer",
							Enum: []any{0, 1},
						},
					},
					{
						Name:        "maxItems",
						In:          "query",
						Description: "maximum number of result items",
						Required:    false,
						Schema: ParamSchema{
							Type: "integer",
						},
					},
					{
						Name:        "flimit",
						In:          "query",
						Description: "minimum frequency of result items to be included in the result set",
						Required:    false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.DefaultFreqLimit,
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/Freqs",
								},
							},
						},
					},
				},
			},
		}
	}

	if collections.SliceContains([]string{"corpus-linguist", ""}, subscriber) {
		paths["/collocations/{corpusId}"] = Methods{
			Get: &Method{
				Description: "Calculate a defined collocation profile of a searched expression. Values are sorted in descending order by their collocation score.",
				OperationID: "Collocations",
				Parameters: []Parameter{
					{
						Name:        "corpusId",
						In:          "path",
						Description: "An ID of a corpus to search in",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "q",
						In:          "query",
						Description: "The translated query",
						Required:    true,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "subcorpus",
						In:          "query",
						Description: "An ID of a subcorpus",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
						},
					},
					{
						Name:        "measure",
						In:          "query",
						Description: "a collocation measure. If omitted, logDice is used.",
						Required:    false,
						Schema: ParamSchema{
							Type: "string",
							Enum: []any{
								"absFreq", "logLikelihood", "logDice", "minSensitivity", "mutualInfo",
								"mutualInfo3", "mutualInfoLogF", "relFreq", "tScore",
							},
							Default: handlers.DefaultCollocationFunc,
						},
					},
					{
						Name:        "srchLeft",
						In:          "query",
						Description: "left range for candidates searching; values must be greater or equal to 1 (1 stands for words right before the searched term)",
						Required:    false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.DefaultSrchLeft,
						},
					},
					{
						Name:        "srchRight",
						In:          "query",
						Description: "right range for candidates searching; values must be greater or equal to 1 (1 stands for words right after the searched term)",
						Required:    false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.DefaultSrchRight,
						},
					},
					{
						Name:        "minCollFreq",
						In:          "query",
						Description: " the minimum frequency that a collocate must have in the searched range.",
						Required:    false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.DefaultMinCollFreq,
						},
					},
					{
						Name:        "maxItems",
						In:          "query",
						Description: "maximum number of result items",
						Required:    false,
						Schema: ParamSchema{
							Type:    "integer",
							Default: handlers.DefaultCollMaxItems,
						},
					},
				},
				Responses: MethodResponses{
					200: MethodResponse{
						Content: map[string]MethodResponseContent{
							"application/json": MethodResponseContent{
								Schema: MethodResponseSchema{
									Ref: "#/components/schemas/Collocations",
								},
							},
						},
					},
				},
			},
		}
	}

	return &APIResponse{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       "MQuery - query and analyze corpus data",
			Description: "Retrieves concordances, frequency information and collocations from language corpora",
			Version:     ver,
		},
		Servers: []Server{
			{URL: url},
		},
		Paths: paths,
		Components: Components{
			Schemas: createSchemas(),
		},
	}
}
