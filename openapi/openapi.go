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

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type Server struct {
	URL string `json:"url"`
}

type ParamSchema struct {
	Type string `json:"type"`
}

type Parameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Schema      ParamSchema `json:"schema"`
}

type Method struct {
	Description string      `json:"description"`
	OperationID string      `json:"operationId"`
	Parameters  []Parameter `json:"parameters"`
	Deprecated  bool        `json:"deprecated"`
}

type Methods struct {
	Get    *Method `json:"get,omitempty"`
	Post   *Method `json:"post,omitempty"`
	Put    *Method `json:"put,omitempty"`
	Delete *Method `json:"delete,omitempty"`
}

type Response struct {
	OpenAPI string             `json:"openapi"`
	Info    Info               `json:"info"`
	Servers []Server           `json:"servers"`
	Paths   map[string]Methods `json:"paths"`
}

func NewResponse(ver, url string) *Response {
	paths := make(map[string]Methods)

	paths["/corplist"] = Methods{
		Get: &Method{
			Description: "Shows a list of available corpora with their basic properties.",
			OperationID: "Corplist",
			Parameters: []Parameter{
				{
					Name:        "locale",
					In:          "query",
					Description: "An ISO 639-1 locale code of response. By default, `en` is used.",
					Required:    false,
					Schema: ParamSchema{
						Type: "string",
					},
				},
			},
		},
	}

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
					Description: "An ISO 639-1 locale code of response. By default, `en` is used.",
					Required:    false,
					Schema: ParamSchema{
						Type: "string",
					},
				},
			},
		},
	}

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
			},
		},
	}

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
			},
		},
	}

	paths["/text-types-overview/{corpusId}"] = Methods{
		Get: &Method{
			Description: "Show text types of a searched term",
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
					Name:        "attr",
					In:          "query",
					Description: "a structural attribute the frequencies will be calculated for (e.g. `doc.pubyear`, `text.author`,...)",
					Required:    false,
					Schema: ParamSchema{
						Type: "string",
					},
				},
			},
		},
	}

	paths["/freqs/{corpusId}"] = Methods{
		Get: &Method{
			Description: "Calculate a frequency distribution for the searched term (KWIC).",
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
					Name:        "fcrit",
					In:          "query",
					Description: "an encoded frequency criterion (e.g. tag 0~0>0); if omitted lemma 0~0>0 is used",
					Required:    false,
					Schema: ParamSchema{
						Type: "string",
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
				},
			},
		},
	}

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
					Description: "a collocation measure. If omitted, logDice is used. The available values are: absFreq, logLikelihood, logDice, minSensitivity, mutualInfo, mutualInfo3, mutualInfoLogF, relFreq, tScore",
					Required:    false,
					Schema: ParamSchema{
						Type: "string",
					},
				},
				{
					Name:        "srchLeft",
					In:          "query",
					Description: "left range for candidates searching (0 is KWIC, values < 0 are on the left side of the KWIC, values > 0 are to the right of the KWIC). The argument can be omitted in which case -5 is used",
					Required:    false,
					Schema: ParamSchema{
						Type: "integer",
					},
				},
				{
					Name:        "srchRight",
					In:          "query",
					Description: "right range for candidates searching (the meaning of concrete values is the same as in srchLeft). The argument can be omitted in which case -5 is used.",
					Required:    false,
					Schema: ParamSchema{
						Type: "integer",
					},
				},
				{
					Name:        "minCollFreq",
					In:          "query",
					Description: " the minimum frequency that a collocate must have in the searched range. The argument is optional with default value of 3",
					Required:    false,
					Schema: ParamSchema{
						Type: "integer",
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
			},
		},
	}

	return &Response{
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
	}
}
