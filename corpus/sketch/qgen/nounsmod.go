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

package qgen

import (
	"fmt"
	"mquery/rdb"
	"mquery/results"
)

type NounsModifiedByQGen struct {
	SketchConf *CorpusSketchSetup
}

func (gen *NounsModifiedByQGen) FxQuery(word string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr, word,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *NounsModifiedByQGen) FxQuerySelectSQL(word string) (sql string, args []any) {
	sql = ""       // TODO
	args = []any{} // TODO
	return
}

func (gen *NounsModifiedByQGen) FxQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.FreqDistribResultType {
		panic("invalid worker result type for NounsModifiedByQGen")
	}
	sql = ""       // TODO
	args = []any{} // TODO
	return
}

func (gen *NounsModifiedByQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *NounsModifiedByQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
	return
}

func (gen *NounsModifiedByQGen) FyQuery(collCandidate string) string {
	return ""
}

func (gen *NounsModifiedByQGen) FyQuerySelectSQL(collCandidate string) (sql string, args []any) {
	return
}

func (gen *NounsModifiedByQGen) FyQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	return
}

func (gen *NounsModifiedByQGen) FxyQuery(word, collCandidate string) string {
	return ""
}

func (gen *NounsModifiedByQGen) FxyQuerySelectSQL(word, collCandidate string) (sql string, args []any) {
	return
}

func (gen *NounsModifiedByQGen) FxyQueryInsertSQL(word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	return
}
