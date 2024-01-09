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

package database

import (
	"context"
	"database/sql"
	"fmt"
	"mquery/corpus"
	"mquery/corpus/baseinfo"
	"mquery/tools"
	"strings"

	"github.com/rs/zerolog/log"
)

// manateeStructsProvider specifies an object able to provide
// structural attribute info via Manatee-open library. Because
// even if corpus information database is available, Mquery does
// not expect it to provide structural and positional attribute
// information (though CNC's database contains such information)
type manateeStructsProvider interface {
	FillStructAndAttrs(corpPath string, info *baseinfo.Corpus) error
	GetRegistryPath(corpusID string) string
}

// KontextDatabase
// note: the lifecycle of the instance
// is "per request"
type KontextDatabase struct {
	db          *sql.DB
	corpusTable string
	ctx         context.Context
	minfo       manateeStructsProvider
}

func (kdb *KontextDatabase) loadCitationInfo(corpusID string) (*baseinfo.Citation, error) {
	sql1 := "SELECT ca.role, a.entry " +
		"FROM kontext_article AS a " +
		"JOIN kontext_corpus_article AS ca ON ca.article_id = a.id " +
		"WHERE ca.corpus_name = ?"
	log.Debug().Str("sql", sql1).Msgf("going to get articles for %s", corpusID)
	rows, err := kdb.db.Query(sql1, corpusID)
	if err != nil {
		return nil, err
	}
	var citationInfo baseinfo.Citation
	for rows.Next() {
		var role, entry string
		err := rows.Scan(&role, &entry)
		if err != nil {
			return nil, err
		}
		switch role {
		case "default":
			citationInfo.DefaultRef = tools.MDToHTML(entry)
		case "standard":
			citationInfo.ArticleRef = append(citationInfo.ArticleRef, tools.MDToHTML(entry))
		case "other":
			citationInfo.OtherBibliography = tools.MDToHTML(entry)
		}
	}
	return &citationInfo, nil
}

func (kdb *KontextDatabase) loadTagsets(corpusID string) ([]baseinfo.Tagset, error) {
	sql1 := "SELECT ct.corpus_name, ct.pos_attr, ct.feat_attr, t.tagset_type, ct.tagset_name, " +
		"ct.kontext_widget_enabled, t.doc_url_local, t.doc_url_en, " +
		"GROUP_CONCAT(CONCAT_WS(',', tpc.tag_search_pattern, tpc.pos) SEPARATOR ';') " +
		"FROM tagset AS t " +
		"JOIN corpus_tagset AS ct ON ct.tagset_name = t.name " +
		"LEFT JOIN tagset_pos_category AS tpc ON ct.tagset_name = tpc.tagset_name " +
		"WHERE ct.corpus_name = ? " +
		"GROUP BY tagset_name"
	log.Debug().Str("sql", sql1).Msgf("going to get tagsets for %s", corpusID)
	rows, err := kdb.db.Query(sql1, corpusID)
	if err != nil {
		return nil, err
	}
	var tagsets []baseinfo.Tagset
	for rows.Next() {
		var tagset baseinfo.Tagset
		var posAttr, docUrlLocal, docUrlEn sql.NullString
		var posCategory string
		err := rows.Scan(&tagset.CorpusName, &posAttr, &tagset.FeatAttr, &tagset.Type, &tagset.ID, &tagset.WidgetEnabled, &docUrlLocal, &docUrlEn, &posCategory)
		if err != nil {
			return nil, err
		}
		tagset.PosAttr = posAttr.String
		tagset.DocUrlLocal = docUrlLocal.String
		tagset.DocUrlEn = docUrlEn.String
		for _, v := range strings.Split(posCategory, ";") {
			if v != "" {
				tagset.PosCategory = append(tagset.PosCategory, strings.Split(v, ","))
			}
		}
		tagsets = append(tagsets, tagset)
	}
	return tagsets, nil
}

// LoadCorpusInfo loads corpus information from database.
// In case a requested corpus is not found, corpus.ErrNotFound is returned.
func (kdb *KontextDatabase) LoadCorpusInfo(corpusID string, language string) (*baseinfo.Corpus, error) {

	sql1 := "SELECT c.name, c.description_%s, c.size, c.web, " +
		"GROUP_CONCAT(CONCAT(kk.label_%s, ':', COALESCE(kk.color, \"rgba(0, 0, 0, 0.0)\")) SEPARATOR ';') " +
		"FROM %s AS c " +
		"LEFT JOIN kontext_keyword_corpus AS kkc ON kkc.corpus_name = c.name " +
		"LEFT JOIN kontext_keyword AS kk ON kkc.keyword_id = kk.id " +
		"WHERE c.active = 1 AND c.name = ? " +
		"GROUP BY c.name "

	log.Debug().Str("sql", sql1).Msgf("going to select corpus info for %s", corpusID)
	var info baseinfo.Corpus
	row := kdb.db.QueryRow(fmt.Sprintf(sql1, language, language, kdb.corpusTable), corpusID)
	var description, keywords, web sql.NullString
	err := row.Scan(&info.Corpname, &description, &info.Size, &web, &keywords)
	if err == sql.ErrNoRows {
		return nil, corpus.ErrNotFound

	} else if err != nil {
		return nil, err
	}
	info.Description = description.String
	info.WebUrl = web.String
	if keywords.Valid {
		for _, keyword := range strings.Split(keywords.String, ";") {
			if keyword != "" {
				values := strings.Split(keyword, ":")
				info.Keywords = append(
					info.Keywords,
					baseinfo.Keyword{Name: values[0], Color: values[1]},
				)
			}
		}
	}
	info.CitationInfo, err = kdb.loadCitationInfo(corpusID)
	if err != nil {
		return nil, err
	}
	info.Tagsets, err = kdb.loadTagsets(corpusID)
	if err != nil {
		return nil, err
	}

	corpPath := kdb.minfo.GetRegistryPath(corpusID)
	err = kdb.minfo.FillStructAndAttrs(corpPath, &info)
	if err != nil {
		return nil, err
	}

	return &info, err
}

func NewKontextDatabase(
	db *sql.DB,
	minfo manateeStructsProvider,
	corpusTable string,
) *KontextDatabase {
	return &KontextDatabase{
		db:          db,
		minfo:       minfo,
		corpusTable: corpusTable,
		ctx:         context.Background(),
	}
}
