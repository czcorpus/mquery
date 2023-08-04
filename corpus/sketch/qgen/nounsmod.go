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

// NounsModifiedByQGen
// example:
// Fx: [lemma="team" & deprel="nmod" & p_upos="NOUN"]
// Fy: [p_lemma="value" & deprel="nmod" & p_upos="NOUN"]
// Fxy: [lemma="team" & p_lemma="value" & deprel="nmod" & p_upos="NOUN"]
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
	sql = fmt.Sprintf(
		"SELECT f.result, f.result_type FROM scoll_query AS q "+
			"JOIN scoll_fcrit AS f ON q.id = f.scoll_query_id "+
			"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s = ? AND q.%s = ? AND f.attr = ?",
		gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
	)
	args = append(
		args,
		word, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, gen.SketchConf.ParLemmaAttr,
	)
	return
}

func (gen *NounsModifiedByQGen) FxQueryInsertSQL(word string, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.ResultTypeFx {
		panic(fmt.Sprintf("invalid worker result type for NounsModifiedByQGen.Fx: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
	)
	var val string
	var rType results.ResultType
	if result != nil {
		val = string(result.Value)
		rType = result.ResultType
	}
	args = []any{
		word,
		gen.SketchConf.NounModifiedValue,
		gen.SketchConf.NounValue,
		val,
		rType,
	}
	return
}

func (gen *NounsModifiedByQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *NounsModifiedByQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
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

func (gen *NounsModifiedByQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.ParLemmaAttr, collCandidate,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *NounsModifiedByQGen) FyQuerySelectSQL(collCandidate string) (sql string, args []any) {
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM scoll_query "+
			"WHERE result_type = 'Fy' AND %s = ? AND %s = ? AND %s = ?",
		gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.ParLemmaAttr,
	)
	args = append(args, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, collCandidate)
	return
}

func (gen *NounsModifiedByQGen) FyQueryInsertSQL(collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ResultTypeFy {
		panic(fmt.Sprintf("invalid worker result type for NounsModifiedByQGen.Fy: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.SketchConf.ParLemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
	)
	args = append(
		args,
		collCandidate,
		gen.SketchConf.NounModifiedValue,
		gen.SketchConf.NounValue,
		result.Value,
		result.ResultType,
	)
	return
}

func (gen *NounsModifiedByQGen) FxyQuery(word, collCandidate string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr, word,
		gen.SketchConf.ParLemmaAttr, collCandidate,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *NounsModifiedByQGen) FxyQuerySelectSQL(word, collCandidate string) (sql string, args []any) {
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM scoll_query "+
			"WHERE result_type = 'Fxy' AND %s = ? AND %s = ? AND %s = ? AND %s = ? ",
		gen.SketchConf.LemmaAttr, gen.SketchConf.ParLemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
	)
	args = append(args, word, collCandidate, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue)
	return
}

func (gen *NounsModifiedByQGen) FxyQueryInsertSQL(word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ResultTypeFxy {
		panic(fmt.Sprintf("invalid worker result type for NounsModifiedByQGen.Fxy: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query (%s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?)",
		gen.SketchConf.LemmaAttr, gen.SketchConf.ParLemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
	)
	args = append(
		args,
		word,
		collCandidate,
		gen.SketchConf.NounModifiedValue,
		gen.SketchConf.NounValue,
		result.Value,
		result.ResultType,
	)
	return
}
