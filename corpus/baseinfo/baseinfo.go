// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
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

package baseinfo

type Item struct {
	Name        string `json:"name"`
	Size        int    `json:"size"`
	Description string `json:"description,omitempty"`
}

type Citation struct {
	DefaultRef        string   `json:"default_ref"`
	ArticleRef        []string `json:"article_ref"`
	OtherBibliography string   `json:"other_bibliography"`
}

type Tagset struct {
	ID            string     `json:"ident"`
	Type          string     `json:"type"`
	CorpusName    string     `json:"corpusName"`
	PosAttr       string     `json:"posAttr"`
	FeatAttr      string     `json:"featAttr"`
	WidgetEnabled bool       `json:"widgetEnabled"`
	DocUrlLocal   string     `json:"docUrlLocal"`
	DocUrlEn      string     `json:"docUrlEn"`
	PosCategory   [][]string `json:"posCategory"`
}

type Corpus struct {
	Corpname       string         `json:"corpname"`
	Description    string         `json:"description"`
	Size           int64          `json:"size"`
	AttrList       []Item         `json:"attrList"`
	StructList     []Item         `json:"structList"`
	TextProperties []TextProperty `json:"textProperties"`
	WebUrl         string         `json:"webUrl"`
	CitationInfo   *Citation      `json:"citationInfo"`
	SrchKeywords   []string       `json:"srchKeywords"`
}
