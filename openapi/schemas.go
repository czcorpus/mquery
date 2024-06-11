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

func createSchemas() ObjectProperties {
	ans := make(ObjectProperties)
	ans["Corplist"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"id": ObjectProperty{
				Type: "string",
			},
			"fullName": ObjectProperty{
				Type: "string",
			},
			"description": ObjectProperty{
				Type: "string",
			},
			"flags": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "string",
				},
			},
		},
	}
	ans["Info"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"data": ObjectProperty{
				Type: "object",
				Properties: ObjectProperties{
					"corpname": ObjectProperty{
						Type: "string",
					},
					"description": ObjectProperty{
						Type: "string",
					},
					"size": ObjectProperty{
						Type: "number",
					},
					"attrList": ObjectProperty{
						Type: "array",
						Items: &arrayItem{
							Type: "object",
							Properties: ObjectProperties{
								"name": ObjectProperty{
									Type: "string",
								},
								"size": ObjectProperty{
									Type: "integer",
								},
								"description": ObjectProperty{
									Type: "string",
								},
							},
						},
					},
					"textProperties": ObjectProperty{
						Type: "array",
						Items: &arrayItem{
							Type: "string",
						},
					},
					"structList": ObjectProperty{
						Type: "array",
						Items: &arrayItem{
							Type: "object",
							Properties: ObjectProperties{
								"name": ObjectProperty{
									Type: "string",
								},
								"size": ObjectProperty{
									Type: "integer",
								},
							},
						},
					},
				},
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"corpusInfo"},
			},
			"locale": ObjectProperty{
				Type: "string",
			},
		},
	}

	ans["Concordance"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"lines": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "object",
					Properties: ObjectProperties{
						"text": ObjectProperty{
							Type: "array",
							Items: &arrayItem{
								Type: "object",
								Properties: ObjectProperties{
									"word": ObjectProperty{
										Type: "string",
									},
									"strong": ObjectProperty{
										Type: "boolean",
									},
									"attrs": ObjectProperty{
										Type: "object",
										AdditionalProperties: &AdditionalProperty{
											Type: "string",
										},
									},
								},
							},
						},
						"ref": ObjectProperty{
							Type: "string",
						},
					},
				},
			},
			"concSize": ObjectProperty{
				Type: "number",
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"conc"},
			},
		},
	}

	ans["Sentences"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"lines": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "object",
					Properties: ObjectProperties{
						"text": ObjectProperty{
							Type: "array",
							Items: &arrayItem{
								Type: "object",
								Properties: ObjectProperties{
									"word": ObjectProperty{
										Type: "string",
									},
									"strong": ObjectProperty{
										Type: "boolean",
									},
									"attrs": ObjectProperty{
										Type: "object",
										AdditionalProperties: &AdditionalProperty{
											Type: "string",
										},
									},
								},
							},
						},
						"ref": ObjectProperty{
							Type: "string",
						},
					},
				},
			},
			"concSize": ObjectProperty{
				Type: "number",
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"conc"},
			},
		},
	}

	ans["TextTypes"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"concSize": ObjectProperty{
				Type: "number",
			},
			"corpusSize": ObjectProperty{
				Type: "number",
			},
			"subcSize": ObjectProperty{
				Type:        "number",
				Description: "In case a subcorpus is involved, the `corpusSize` refers to the parent corpus so this presents the actual searched data size.",
			},
			"freqs": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "object",
					Properties: ObjectProperties{
						"value": ObjectProperty{
							Type:        "string",
							Description: "an analyzed term",
						},
						"freq": ObjectProperty{
							Type:        "number",
							Description: "absolute frequency",
						},
						"base": ObjectProperty{
							Type:        "number",
							Description: "A base corpus size the ipm was calculated against. Here it means all the texts with the analyzed value of the required textProperty",
						},
						"ipm": ObjectProperty{
							Type:        "number",
							Description: "Instances Per Million (relativized frequency according to the `base`)",
						},
					},
				},
			},
			"fcrit": ObjectProperty{
				Type:        "string",
				Description: "An internal translation of the required frequency properties into the appropriate corpus search engine format. This is mainly for backlinking purposes.",
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"freqs"},
			},
		},
	}

	ans["TextTypesOverview"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"freqs": ObjectProperty{
				Type: "object", // TODO describe the object
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"multipleFreqs"},
			},
		},
	}

	ans["TermFrequency"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"total": ObjectProperty{
				Type: "number",
			},
			"arf": ObjectProperty{
				Type: "number",
			},
			"ipm": ObjectProperty{
				Type: "number",
			},
			"corpusSize": ObjectProperty{
				Type: "number",
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"termFrequency"},
			},
		},
	}

	ans["Freqs"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"concSize": ObjectProperty{
				Type: "number",
			},
			"corpusSize": ObjectProperty{
				Type: "number",
			},
			"subcSize": ObjectProperty{
				Type:        "number",
				Description: "In case a subcorpus is involved, the `corpusSize` refers to the parent corpus so this presents the actual searched data size.",
			},
			"freqs": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "object",
					Properties: ObjectProperties{
						"value": ObjectProperty{
							Type:        "string",
							Description: "an analyzed term",
						},
						"freq": ObjectProperty{
							Type:        "number",
							Description: "absolute frequency",
						},
						"base": ObjectProperty{
							Type:        "number",
							Description: "a base corpus size the ipm was calculated against",
						},
						"ipm": ObjectProperty{
							Type:        "number",
							Description: "Instances Per Million (relativized frequency according to the `base`)",
						},
					},
				},
			},
			"fcrit": ObjectProperty{
				Type:        "string",
				Description: "An internal translation of the required frequency properties into the appropriate corpus search engine format. This is mainly for backlinking purposes.",
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"freqs"},
			},
		},
	}

	ans["Collocations"] = ObjectProperty{
		Type: "object",
		Properties: ObjectProperties{
			"corpusSize": ObjectProperty{
				Type: "number",
			},
			"subcSize": ObjectProperty{
				Type:        "number",
				Description: "In case a subcorpus is involved, the `corpusSize` refers to the parent corpus so this presents the actual searched data size.",
			},
			"colls": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "object",
					Properties: ObjectProperties{
						"word": ObjectProperty{
							Type: "string",
						},
						"score": ObjectProperty{
							Type: "number",
						},
						"freq": ObjectProperty{
							Type: "number",
						},
					},
				},
			},
			"resultType": ObjectProperty{
				Type: "string",
				Enum: []any{"resultType"},
			},
			"measure": ObjectProperty{
				Type: "string",
				Enum: []any{
					"absFreq", "logLikelihood", "logDice",
					"minSensitivity", "mutualInfo", "mutualInfo3",
					"mutualInfoLogF", "relFreq", "tScore",
				},
			},
			"srchContext": ObjectProperty{
				Type: "array",
				Items: &arrayItem{
					Type: "number",
				},
				Description: "always a two-item array with left + right context",
			},
		},
	}

	ans["TranslatedQuery"] = ObjectProperty{
		Type:        "string",
		Description: "a CQL variant of the entered query",
	}

	return ans
}
