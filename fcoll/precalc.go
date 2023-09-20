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
	"mquery/corpus/scoll"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/rs/zerolog/log"
	"github.com/tomachalek/vertigo/v5"
)

const (
	bulkInsertChunkSize = 500
)

type CTItem struct {
	Lemma  string
	PLemma string
	Deprel string
	Upos   string
	PUpos  string
	Freq   int64
}

type CounterTable map[string]*CTItem

func (table CounterTable) mkKey(lemma, upos, pLemma, pUpos, deprel string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", lemma, upos, deprel, pLemma, pUpos)
}

func (table CounterTable) Add(lemma, upos, pLemma, pUpos, deprel string, val int64) {
	key := table.mkKey(lemma, upos, pLemma, pUpos, deprel)
	v, ok := table[key]
	if !ok {
		v = &CTItem{
			Lemma:  lemma,
			Upos:   upos,
			PLemma: pLemma,
			PUpos:  pUpos,
			Deprel: deprel,
		}
		table[key] = v
	}
	v.Freq += val
}

type VertProcessor struct {
	DeprelCol   int
	DeprelTypes []string
	scoll.SketchConfig
	conf  *scoll.CorpusSketchSetup
	Table CounterTable
}

func (vp *VertProcessor) ProcToken(token *vertigo.Token, line int, err error) error {
	if err != nil {
		return err
	}
	if len(token.Attrs) < 12 {
		log.Error().Msgf("Too few token columns on line %d", line)
		return nil
	}
	// below, we index always [k-1] because `word` in Vertigo is separated
	deprelTmp := token.Attrs[vp.conf.FuncAttr.VerticalCol-1]
	lemma := token.Attrs[vp.conf.LemmaAttr.VerticalCol-1]
	upos := token.Attrs[vp.conf.PosAttr.VerticalCol-1]
	pUpos := token.Attrs[vp.conf.ParPosAttr.VerticalCol-1]
	pLemma := token.Attrs[vp.conf.ParLemmaAttr.VerticalCol-1]
	for _, deprel := range expandDeprelMultivalue(deprelTmp) {
		if collections.SliceContains(vp.DeprelTypes, deprel) {
			vp.Table.Add(lemma, upos, pLemma, pUpos, deprel, 1)
		}
	}
	//useFirstNonWordPosAttr(tokenAttrs[0])

	return nil
}

func (vp *VertProcessor) ProcStruct(strc *vertigo.Structure, line int, err error) error {
	return nil
}

func (vp *VertProcessor) ProcStructClose(strc *vertigo.StructureClose, line int, err error) error {
	return nil
}

func insertColl(db *sql.DB, item *CTItem) error {
	_, err := db.Exec("INSERT INTO intercorp_v13ud_cs_fcolls (lemma, upos, p_lemma, p_upos, deprel, freq) "+
		"VALUES (?, ?, ?, ?, ?, ?)", item.Lemma, item.Upos, item.PLemma, item.PUpos, item.Deprel, item.Freq,
	)
	if err != nil {
		return err
	}
	return nil
}

func expandDeprelMultivalue(value string) []string {
	ans := make([]string, 0, 2)
	tmp := strings.Split(value, "|")
	if len(tmp) > 2 {
		log.Warn().
			Str("expression", value).
			Msg("deprel expression not fully supported")
	}
	for _, t := range tmp {
		ans = append(ans, t)
	}
	// this along with individual items does not cover whole
	// expression but it should be ok
	ans = append(ans, value)
	return ans
}

func expandDeprelMultivalues(values []string) []string {
	ans := make([]string, 0, len(values)+2)
	for _, v := range values {
		ans = append(ans, expandDeprelMultivalue(v)...)
	}
	return ans
}

// TODO: update intercorp_v13ud_cs_fcolls set chunk = (FLOOR( 1 + RAND( ) *32))

func runForDeprel(corpusID, vertPath string, conf *scoll.CorpusSketchSetup, db *sql.DB) error {
	pc := &vertigo.ParserConf{
		InputFilePath:         vertPath,
		Encoding:              "utf-8",
		StructAttrAccumulator: "comb",
	}
	table := make(CounterTable)
	proc := &VertProcessor{
		DeprelTypes: expandDeprelMultivalues(
			[]string{
				conf.NounModifiedValue,
				conf.NounSubjectValue,
				conf.NounObjectValue,
			},
		),
		conf:  conf,
		Table: table,
	}
	err := vertigo.ParseVerticalFile(pc, proc)
	if err != nil {
		return err
	}

	log.Info().Int("size", len(table)).Msg("collocation table done")

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf("DELETE FROM %s_fcolls", corpusID))
	if err != nil {
		return err
	}

	i := 0
	valueStrings := make([]string, 0, bulkInsertChunkSize)
	args := make([]any, 0, bulkInsertChunkSize*6)

	mkStmt := func() string {
		return fmt.Sprintf(
			"INSERT INTO %s_fcolls (lemma, upos, p_lemma, p_upos, deprel, freq) "+
				"VALUES %s", corpusID, strings.Join(valueStrings, ", "))
	}

	log.Info().Msg("writing data into database")
	t0 := time.Now()

	for _, v := range table {
		if i == bulkInsertChunkSize {
			_, err := db.Exec(mkStmt(), args...)
			if err != nil {
				tx.Rollback()
				return err
			}
			valueStrings = make([]string, 0, bulkInsertChunkSize)
			args = make([]any, 0, bulkInsertChunkSize*6)
			i = 0
		}

		args = append(args, v.Lemma, v.Upos, v.PLemma, v.PUpos, v.Deprel, v.Freq)
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?)")
		i++
	}

	if len(args) > 0 {
		_, err = db.Exec(mkStmt(), args...)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	log.Info().Float64("durationSec", time.Since(t0).Seconds()).Msg("...writing done")
	return err
}

func Run(corpusID, vertPath string, conf *scoll.CorpusSketchSetup, db *sql.DB) error {
	return runForDeprel(
		corpusID,
		vertPath,
		conf,
		db,
	)

}
