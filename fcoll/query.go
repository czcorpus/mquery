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
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	candidatesFreqLimit = 15
)

type Candidate struct {
	Lemma  string
	Upos   string
	FreqXY int64
	FreqY  int64
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
	partialResults := make(chan []*Candidate)
	wg := sync.WaitGroup{}
	go func() {
		for i := 1; i <= 32; i++ {
			wg.Add(1)
			go func(chunkID int) {
				defer wg.Done()
				ans, err := cdb.getChildCandidatesForChunk(pLemma, pUpos, deprel, minFreq, chunkID)
				if err != nil {
					log.Error().Err(err).Msg("Failed to process") // TODO
				}
				partialResults <- ans
			}(i)
		}
		wg.Wait()
		close(partialResults)
	}()

	totalResult := make([]*Candidate, 0, 32*10)
	for pr := range partialResults {
		totalResult = append(totalResult, pr...)
	}
	return totalResult, nil // TODO err
}

func (cdb *CollDatabase) getChildCandidatesForChunk(pLemma, pUpos, deprel string, minFreq int, chunk int) ([]*Candidate, error) {
	whereSQL := make([]string, 0, 4)
	whereSQL = append(whereSQL, "p_lemma = ?", "freq >= ?", "chunk = ?")
	whereArgs := make([]any, 0, 4)
	whereArgs = append(whereArgs, pLemma, minFreq, chunk)

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
		"SELECT a.lemma, a.upos, a.freq, "+
			"(SELECT SUM(freq) FROM intercorp_v13ud_cs_fcolls AS b "+
			" WHERE b.lemma = a.lemma AND b.upos = a.upos AND b.deprel = a.deprel) "+
			"FROM %s_fcolls AS a WHERE %s ",
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
		err := rows.Scan(&item.Lemma, &item.Upos, &item.FreqXY, &item.FreqXY)
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
		"SELECT p_lemma, p_upos, freq, "+
			"(SELECT SUM(freq) FROM intercorp_v13ud_cs_fcolls AS b "+
			" WHERE b.p_lemma = a.p_lemma AND b.p_upos = a.p_upos AND b.deprel = a.deprel) "+
			"FROM %s_fcolls AS a WHERE %s ",
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
		err := rows.Scan(&item.Lemma, &item.Upos, &item.FreqXY, &item.FreqY)
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