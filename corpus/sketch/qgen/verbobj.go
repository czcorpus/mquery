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
	sql = fmt.Sprintf("SELECT f.result, f.result_type FROM scoll_query AS q "+
		"JOIN scoll_fcrit AS f ON q.id = f.scoll_query_id "+
		"WHERE q.%s = ? AND q.%s IS NULL AND q.%s = ? AND q.%s = ? AND f.attr = ?",
		gen.SketchConf.LemmaAttr, gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr)
	args = []any{
		word,
		gen.SketchConf.NounObjectValue,
		gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr,
	}
	return
}

func (gen *VerbObjectQGen) FxQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.FreqDistribResultType {
		panic("invalid worker result type for VerbObjectQGen")
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
	)
	var val, rType string
	if result != nil {
		val = string(result.Value)
		rType = result.ResultType
	}
	args = []any{
		word,
		gen.SketchConf.NounObjectValue,
		gen.SketchConf.VerbValue,
		val,
		rType,
	}
	return
}

func (gen *VerbObjectQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *VerbObjectQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
	sql = "INSERT INTO scoll_fcrit (scoll_query_id, attr, result, result_type) VALUES (?, ?, ?, ?)"
	args = append(
		args,
		query_id,
		gen.SketchConf.ParLemmaAttr,
		result.Value,
		result.ResultType,
	)
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
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM scoll_query "+
			"WHERE %s = ? AND %s = ? AND %s = ? AND %s IS NULL ",
		gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.ParLemmaAttr, gen.SketchConf.LemmaAttr,
	)
	args = append(args, gen.SketchConf.NounObjectValue, gen.SketchConf.VerbValue, collCandidate)
	return
}

func (gen *VerbObjectQGen) FyQueryInsertSQL(collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ConcSizeResultType {
		panic("invalid worker result type for VerbObjectQGen")
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.ParLemmaAttr,
	)
	args = append(
		args,
		gen.SketchConf.NounObjectValue,
		gen.SketchConf.VerbValue,
		collCandidate,
		result.Value,
		result.ResultType,
	)
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
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM scoll_query WHERE %s = ? AND %s = ? AND %s = ? AND %s = ? ",
		gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.ParLemmaAttr,
	)
	args = append(args, word, gen.SketchConf.NounObjectValue, gen.SketchConf.VerbValue, collCandidate)
	return
}

func (gen *VerbObjectQGen) FxyQueryInsertSQL(word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query (%s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?)",
		gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.ParLemmaAttr,
	)
	args = append(
		args,
		word,
		gen.SketchConf.NounObjectValue,
		gen.SketchConf.VerbValue,
		collCandidate,
		result.Value,
		result.ResultType,
	)
	return
}