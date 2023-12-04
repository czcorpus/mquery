// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
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

package engine

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// KontextDatabase
// note: the lifecycle of the instance
// is "per request"
type KontextDatabase struct {
	db          *sql.DB
	corpusTable string
	language    string
	ctx         context.Context
}

type StructAttr struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

type CitationInfo struct {
	DefaultRef        string   `json:"default_ref"`
	ArticleRef        []string `json:"article_ref"`
	OtherBibliography string   `json:"other_bibliography"`
}

type Keyword struct {
	Name  string `json:"name"`
	Color string `json:"color"`
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

type CorpusInfo struct {
	Corpname     string       `json:"corpname"`
	Description  string       `json:"description"`
	Size         int          `json:"size"`
	AttrList     []StructAttr `json:"attrlist"`
	StructList   []StructAttr `json:"structlist"`
	WebUrl       string       `json:"webUrl"`
	CitationInfo CitationInfo `json:"citationInfo"`
	Keywords     []Keyword    `json:"keywords"`
	Tagsets      []Tagset     `json:"tagsets"`
}

func (kdb *KontextDatabase) LoadCorpusInfo(corpusID string) (*CorpusInfo, error) {

	sql1 := "SELECT c.name, c.description_%s, c.size, c.web, " +
		"GROUP_CONCAT(CONCAT(kk.label_%s, ':', COALESCE(kk.color, \"rgba(0, 0, 0, 0.0)\")), ';') " +
		"FROM %s AS c " +
		"LEFT JOIN kontext_keyword_corpus AS kkc ON kkc.corpus_name = c.name " +
		"LEFT JOIN kontext_keyword AS kk ON kkc.keyword_id = kk.id " +
		"WHERE c.active = 1 AND c.name = ? " +
		"GROUP BY c.name "

	log.Debug().Str("sql", sql1).Msgf("going to corpus info for %s", corpusID)
	var info CorpusInfo
	row := kdb.db.QueryRow(fmt.Sprintf(sql1, kdb.language, kdb.language, kdb.corpusTable), corpusID)
	var keywords, web sql.NullString
	err := row.Scan(&info.Corpname, &info.Description, &info.Size, &web, &keywords)
	if err != nil {
		return nil, err
	}
	info.WebUrl = web.String
	if keywords.Valid {
		for _, keyword := range strings.Split(keywords.String, ";") {
			values := strings.Split(keyword, ":")
			info.Keywords = append(info.Keywords, Keyword{Name: values[0], Color: values[1]})
		}
	}
	return &info, err
}

func NewKontextDatabase(db *sql.DB, corpusTable string, language string) *KontextDatabase {
	return &KontextDatabase{
		db:          db,
		corpusTable: corpusTable,
		language:    language,
		ctx:         context.Background(),
	}
}
