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

type VerbObjectQGen struct {
	SketchConf *CorpusSketchSetup
}

func (gen *VerbObjectQGen) FxQuery(word string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr, word,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounObjectValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *VerbObjectQGen) FxQuerySelectSQL(word string) (sql string, args []any) {
	sql = ""       // TODO
	args = []any{} // TODO
	return
}

func (gen *VerbObjectQGen) FxQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.FreqDistribResultType {
		panic("invalid worker result type for VerbObjectQGen")
	}
	sql = ""       // TODO
	args = []any{} // TODO
	return
}

func (gen *VerbObjectQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *VerbObjectQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
	return
}

func (gen *VerbObjectQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		`[deprel="%s" & p_upos="%s" & p_lemma="%s"]`, // TODO generalize attrs !!!
		gen.SketchConf.NounObjectValue,
		gen.SketchConf.VerbValue,
		collCandidate,
	)
}

func (gen *VerbObjectQGen) FyQuerySelectSQL(collCandidate string) (sql string, args []any) {
	return
}

func (gen *VerbObjectQGen) FyQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	return
}

func (gen *VerbObjectQGen) FxyQuery(word, collCandidate string) string {
	return fmt.Sprintf(
		`[%s="%s" & %s="%s" & %s="%s" & %s="%s"]`,
		gen.SketchConf.LemmaAttr, word,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounObjectValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr, collCandidate,
	)
}

func (gen *VerbObjectQGen) FxyQuerySelectSQL(word, collCandidate string) (sql string, args []any) {
	return
}

func (gen *VerbObjectQGen) FxyQueryInsertSQL(word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	return
}
