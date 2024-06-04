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
	Type string   `json:"type"`
	Enum []string `json:"enum,omitempty"`
}

type Parameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Schema      ParamSchema `json:"schema"`
}

type Method struct {
	Description string          `json:"description"`
	OperationID string          `json:"operationId"`
	Parameters  []Parameter     `json:"parameters"`
	Responses   MethodResponses `json:"responses"`
	Deprecated  bool            `json:"deprecated"`
}

type Methods struct {
	Get    *Method `json:"get,omitempty"`
	Post   *Method `json:"post,omitempty"`
	Put    *Method `json:"put,omitempty"`
	Delete *Method `json:"delete,omitempty"`
}

type arrayItem struct {
	Type        string           `json:"type"`
	Properties  ObjectProperties `json:"properties,omitempty"`
	Description string           `json:"description,omitempty"`
}

type AdditionalProperty struct {
	Type string `json:"type"`
}

type ObjectProperty struct {
	Type                 string             `json:"type"`
	Enum                 []string           `json:"enum,omitempty"`
	Properties           ObjectProperties   `json:"properties,omitempty"`
	Items                *arrayItem         `json:"items,omitempty"`
	AdditionalProperties AdditionalProperty `json:"additionalProperties,omitempty"`
	Description          string             `json:"description,omitempty"`
}

type ObjectProperties map[string]ObjectProperty

type MethodResponseSchema struct {
	Type       string           `json:"type"`
	Properties ObjectProperties `json:"properties,omitempty"`
	Format     string           `json:"format,omitempty"`
}

type MethodResponseContent struct {
	Schema MethodResponseSchema `json:"schema"`
}

type MethodResponse struct {
	Description string                           `json:"description"`
	Content     map[string]MethodResponseContent `json:"content"`
}

type MethodResponses map[int]MethodResponse

type APIResponse struct {
	OpenAPI string             `json:"openapi"`
	Info    Info               `json:"info"`
	Servers []Server           `json:"servers"`
	Paths   map[string]Methods `json:"paths"`
}
