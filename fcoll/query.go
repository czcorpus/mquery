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

package fcoll

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	candidatesFreqLimit = 10
)

type Candidate struct {
	Lemma string
	Upos  string
	Freq  int64
}

type CollDatabase struct {
	db       *sql.DB
	corpusID string
}

func (cdb *CollDatabase) GetFreq(lemma, upos, pLemma, pUpos, deprel string) (int64, error) {

	whereSQL := make([]string, 0, 4)
	whereArgs := make([]any, 0, 4)
	if deprel != "" {
		deprelParsed := strings.Split(deprel, "|")
		deprelArgs := make([]any, len(deprelParsed))
		deprelSql := make([]string, len(deprelParsed))
		for i, dp := range deprelParsed {
			deprelSql[i] = fmt.Sprintf("deprel = ?")
			deprelArgs[i] = dp
		}
		whereSQL = append(whereSQL, fmt.Sprintf("(%s)", strings.Join(deprelSql, " OR ")))
		whereArgs = append(whereArgs, deprelArgs...)
	}
	if lemma != "" {
		whereSQL = append(whereSQL, "lemma = ?")
		whereArgs = append(whereArgs, lemma)
	}
	if upos != "" {
		whereSQL = append(whereSQL, "upos = ?")
		whereArgs = append(whereArgs, upos)
	}
	if pLemma != "" {
		whereSQL = append(whereSQL, "p_lemma = ?")
		whereArgs = append(whereArgs, pLemma)
	}
	if pUpos != "" {
		whereSQL = append(whereSQL, "p_upos = ?")
		whereArgs = append(whereArgs, pUpos)
	}
	sql := fmt.Sprintf("SELECT SUM(freq) FROM %s_fcolls WHERE %s", cdb.corpusID, strings.Join(whereSQL, " AND "))
	log.Debug().Str("sql", sql).Any("args", whereArgs).Msg("going to SELECT cumulative freq.")
	t0 := time.Now()
	row := cdb.db.QueryRow(sql, whereArgs...)
	if row.Err() != nil {
		return 0, row.Err()
	}
	var ans int64
	row.Scan(&ans)
	log.Debug().Float64("proctime", time.Since(t0).Seconds()).Msg(".... DONE (select cumulative freq.)")
	return ans, nil
}

func (cdb *CollDatabase) GetChildCandidates(pLemma, pUpos, deprel string, minFreq int) ([]*Candidate, error) {
	whereSQL := make([]string, 0, 4)
	whereSQL = append(whereSQL, "p_lemma = ?", "freq >= ?")
	whereArgs := make([]any, 0, 4)
	whereArgs = append(whereArgs, pLemma, minFreq)

	if deprel != "" {
		deprelParsed := strings.Split(deprel, "|")
		deprelArgs := make([]any, len(deprelParsed))
		deprelSql := make([]string, len(deprelParsed))
		for i, dp := range deprelParsed {
			deprelSql[i] = fmt.Sprintf("deprel = ?")
			deprelArgs[i] = dp
		}
		whereSQL = append(whereSQL, fmt.Sprintf("(%s)", strings.Join(deprelSql, " OR ")))
		whereArgs = append(whereArgs, deprelArgs...)
	}

	if pUpos != "" {
		whereSQL = append(whereSQL, "p_upos = ?")
		whereArgs = append(whereArgs, pUpos)
	}
	sql := fmt.Sprintf(
		"SELECT lemma, upos, freq FROM %s_fcolls WHERE %s ",
		cdb.corpusID, strings.Join(whereSQL, " AND "),
	)
	log.Debug().Str("sql", sql).Any("args", whereArgs).Msg("going to SELECT child candidates")
	t0 := time.Now()
	rows, err := cdb.db.Query(sql, whereArgs...)
	if err != nil {
		return []*Candidate{}, err
	}
	ans := make([]*Candidate, 0, 100)
	for rows.Next() {
		item := &Candidate{}
		err := rows.Scan(&item.Lemma, &item.Upos, &item.Freq)
		if err != nil {
			return ans, err
		}
		ans = append(ans, item)
	}
	log.Debug().Float64("proctime", time.Since(t0).Seconds()).Msg(".... DONE (SELECT child candidates)")
	return ans, nil
}

func (cdb *CollDatabase) GetParentCandidates(lemma, upos, deprel string, minFreq int) ([]*Candidate, error) {
	whereSQL := make([]string, 0, 4)
	whereSQL = append(whereSQL, "lemma = ?", "freq >= ?")
	whereArgs := make([]any, 0, 4)
	whereArgs = append(whereArgs, lemma, minFreq)

	if deprel != "" {
		deprelParsed := strings.Split(deprel, "|")
		deprelArgs := make([]any, len(deprelParsed))
		deprelSql := make([]string, len(deprelParsed))
		for i, dp := range deprelParsed {
			deprelSql[i] = fmt.Sprintf("deprel = ?")
			deprelArgs[i] = dp
		}
		whereSQL = append(whereSQL, fmt.Sprintf("(%s)", strings.Join(deprelSql, " OR ")))
		whereArgs = append(whereArgs, deprelArgs...)
	}

	if upos != "" {
		whereSQL = append(whereSQL, "upos = ?")
		whereArgs = append(whereArgs, upos)
	}
	sql := fmt.Sprintf(
		"SELECT p_lemma, p_upos, freq FROM %s_fcolls WHERE %s ",
		cdb.corpusID, strings.Join(whereSQL, " AND "),
	)
	log.Debug().Str("sql", sql).Any("args", whereArgs).Msg("going to SELECT parent candidates")
	t0 := time.Now()
	rows, err := cdb.db.Query(sql, whereArgs...)
	if err != nil {
		return []*Candidate{}, err
	}
	ans := make([]*Candidate, 0, 100)
	for rows.Next() {
		item := &Candidate{}
		err := rows.Scan(&item.Lemma, &item.Upos, &item.Freq)
		if err != nil {
			return ans, err
		}
		ans = append(ans, item)
	}
	log.Debug().Float64("proctime", time.Since(t0).Seconds()).Msg(".... DONE (SELECT parent candidates)")
	return ans, nil
}

func NewCollDatabase(db *sql.DB, corpusID string) *CollDatabase {
	return &CollDatabase{
		db:       db,
		corpusID: corpusID,
	}
}
