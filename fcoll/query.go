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

type batchFreqArgs struct {
	Lemma        string
	Upos         string
	Deprel       string
	fxy          int64
	isParentSrch bool
}

type CollDatabase struct {
	db       *sql.DB
	corpusID string
}

func (cdb *CollDatabase) createFreqQueryProps(arg batchFreqArgs) (string, []any) {
	whereSQL := make([]string, 0, 4)
	whereArgs := make([]any, 0, 4)
	if arg.Deprel != "" {
		deprelParsed := strings.Split(arg.Deprel, "|")
		deprelArgs := make([]any, len(deprelParsed))
		deprelSql := make([]string, len(deprelParsed))
		for i, dp := range deprelParsed {
			deprelSql[i] = fmt.Sprintf("deprel = ?")
			deprelArgs[i] = dp
		}
		whereSQL = append(whereSQL, fmt.Sprintf("(%s)", strings.Join(deprelSql, " OR ")))
		whereArgs = append(whereArgs, deprelArgs...)
	}
	if arg.isParentSrch {
		if arg.Lemma != "" {
			whereSQL = append(whereSQL, "p_lemma = ?")
			whereArgs = append(whereArgs, arg.Lemma)
		}
		if arg.Upos != "" {
			whereSQL = append(whereSQL, "p_upos = ?")
			whereArgs = append(whereArgs, arg.Upos)
		}

	} else {
		whereSQL = append(whereSQL, "lemma = ?")
		whereArgs = append(whereArgs, arg.Lemma)

		whereSQL = append(whereSQL, "upos = ?")
		whereArgs = append(whereArgs, arg.Upos)
	}

	return "(" + strings.Join(whereSQL, " AND ") + ")", whereArgs
}

// GetFreqBatch
// note: we expect all the brachFreqArgs:
// 1) have all either "lemma", "upos" or "p_lemma", "p_upos" filled in
// 2) have the same deprel value
func (cdb *CollDatabase) GetFreqBatch(args []batchFreqArgs) ([]Candidate, error) {
	whereSQL := make([]string, 0, len(args))
	whereArgs := make([]any, 0, len(args))
	for _, arg := range args {
		ws, wa := cdb.createFreqQueryProps(arg)
		whereSQL = append(whereSQL, ws)
		whereArgs = append(whereArgs, wa...)
	}

	groupBySQL := make([]string, 0, len(args))
	attrSelSQL := make([]string, 0, len(args))
	if args[0].isParentSrch {
		groupBySQL = append(groupBySQL, "p_lemma", "p_upos", "deprel")
		attrSelSQL = append(attrSelSQL, "p_lemma AS lemma_val", "p_upos AS upos_val")

	} else {
		groupBySQL = append(groupBySQL, "lemma", "upos", "deprel")
		attrSelSQL = append(attrSelSQL, "lemma AS lemma_val", "upos AS upos_val")
	}

	sql := fmt.Sprintf(
		"SELECT SUM(freq), %s "+
			"FROM %s_fcolls "+
			"WHERE %s "+
			"GROUP BY %s",
		strings.Join(attrSelSQL, ", "),
		cdb.corpusID, strings.Join(whereSQL, " OR "), strings.Join(groupBySQL, ", "))
	t0 := time.Now()
	row, err := cdb.db.Query(sql, whereArgs...)
	if err != nil {
		return []Candidate{}, err
	}
	freqs := make([]Candidate, 0, len(args))
	for row.Next() {
		var cand Candidate
		err := row.Scan(&cand.Freq, &cand.Lemma, &cand.Upos)
		if err != nil {
			return []Candidate{}, err
		}
		freqs = append(freqs, cand)
	}
	log.Debug().
		Float64("proctime", time.Since(t0).Seconds()).
		Int("resultSize", len(freqs)).
		Int("batchSize", len(args)).
		Msg("DONE select cumulative freq. (batch mode)")
	return freqs, nil
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
	t0 := time.Now()
	row := cdb.db.QueryRow(sql, whereArgs...)
	if row.Err() != nil {
		return 0, row.Err()
	}
	var ans int64
	row.Scan(&ans)
	log.Debug().Float64("proctime", time.Since(t0).Seconds()).Msg("DONE select cumulative freq.")
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
