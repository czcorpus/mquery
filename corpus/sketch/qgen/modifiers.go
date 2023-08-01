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

type ModifiersOfQGen struct {
	SketchConf *CorpusSketchSetup
}

func (gen *ModifiersOfQGen) FxQuery(word string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.ParLemmaAttr, word,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.PosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *ModifiersOfQGen) FxQuerySelectSQL(word string) (sql string, args []any) {
	sql = ""       // TODO
	args = []any{} // TODO
	return
}

func (gen *ModifiersOfQGen) FxQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.FreqDistribResultType {
		panic("invalid worker result type for ModifiersOfQGen")
	}
	sql = ""       // TODO
	args = []any{} // TODO
	return
}

func (gen *ModifiersOfQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.LemmaAttr)
}

func (gen *ModifiersOfQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
	return
}

func (gen *ModifiersOfQGen) FyQuery(collCandidate string) string {
	return ""
}

func (gen *ModifiersOfQGen) FyQuerySelectSQL(collCandidate string) (sql string, args []any) {
	return
}

func (gen *ModifiersOfQGen) FyQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	return
}

func (gen *ModifiersOfQGen) FxyQuery(word, collCandidate string) string {
	return ""
}

func (gen *ModifiersOfQGen) FxyQuerySelectSQL(word, collCandidate string) (sql string, args []any) {
	return
}

func (gen *ModifiersOfQGen) FxyQueryInsertSQL(word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	return
}
