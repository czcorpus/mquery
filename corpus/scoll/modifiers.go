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

package scoll

import (
	sqlLib "database/sql"
	"fmt"
	"mquery/rdb"
	"mquery/results"
)

// ModifiersOfQGen
// Fx: [p_lemma="team" & deprel="nmod" & upos="NOUN"]
// Fy: [lemma="value" & deprel="nmod" & upos="NOUN"]
// Fxy: [p_lemma="team" & lemma="value" & deprel="nmod" & upos="NOUN"]
type ModifiersOfQGen struct {
	SketchConf *CorpusSketchSetup
	CorpusName string
}

func (gen *ModifiersOfQGen) FxQuery(word Word) string {
	if word.PoS == "" {
		return fmt.Sprintf(
			"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
			gen.SketchConf.ParLemmaAttr, word.V,
			gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
			gen.SketchConf.PosAttr, gen.SketchConf.NounValue,
		)
	}
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.ParLemmaAttr, word.V,
		gen.SketchConf.ParPosAttr, word.PoS,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.PosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *ModifiersOfQGen) FxQuerySelectSQL(word Word) (sql string, args []any) {
	if word.PoS == "" {
		sql = fmt.Sprintf(
			"SELECT f.result, f.result_type FROM scoll_query_%s AS q "+
				"JOIN scoll_fcrit_%s AS f ON q.id = f.scoll_query_id "+
				"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s IS NULL AND q.%s = ? AND q.%s = ? AND f.attr = ?",
			gen.CorpusName,
			gen.CorpusName,
			gen.SketchConf.ParLemmaAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.FuncAttr, gen.SketchConf.PosAttr,
		)
		args = append(
			args,
			word.V, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, gen.SketchConf.LemmaAttr,
		)
		return
	}
	sql = fmt.Sprintf(
		"SELECT f.result, f.result_type FROM scoll_query_%s AS q "+
			"JOIN scoll_fcrit_%s AS f ON q.id = f.scoll_query_id "+
			"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s = ? AND q.%s = ? AND q.%s = ? AND f.attr = ?",
		gen.CorpusName,
		gen.CorpusName,
		gen.SketchConf.ParLemmaAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.FuncAttr, gen.SketchConf.PosAttr,
	)
	args = append(
		args,
		word.V, word.PoS, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, gen.SketchConf.ParLemmaAttr,
	)
	return
}

func (gen *ModifiersOfQGen) FxQueryInsertSQL(word Word, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.ResultTypeFx {
		panic(fmt.Sprintf("invalid worker result type for ModifiersOfQGen.Fx: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query_%s (%s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?)",
		gen.CorpusName, gen.SketchConf.ParLemmaAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.FuncAttr,
		gen.SketchConf.PosAttr,
	)
	var val string
	var rType results.ResultType
	if result != nil {
		val = string(result.Value)
		rType = result.ResultType
	}
	var posValue sqlLib.NullString
	if word.PoS != "" {
		posValue.String = word.PoS
		posValue.Valid = true
	}
	args = []any{
		word.V,
		posValue,
		gen.SketchConf.NounModifiedValue,
		gen.SketchConf.NounValue,
		val,
		rType,
	}
	return
}

func (gen *ModifiersOfQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.LemmaAttr)
}

func (gen *ModifiersOfQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
	sql = fmt.Sprintf(
		"INSERT INTO scoll_fcrit_%s (scoll_query_id, attr, result, result_type) VALUES (?, ?, ?, ?)",
		gen.CorpusName,
	)
	args = append(
		args,
		query_id,
		gen.SketchConf.LemmaAttr,
		result.Value,
		result.ResultType,
	)
	return
}

func (gen *ModifiersOfQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr, collCandidate,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.NounValue,
	)
}

func (gen *ModifiersOfQGen) FyQuerySelectSQL(collCandidate string) (sql string, args []any) {
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM scoll_query_%s "+
			"WHERE result_type = 'Fy' AND %s = ? AND %s = ? AND %s = ?",
		gen.CorpusName, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.LemmaAttr,
	)
	args = append(args, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, collCandidate)
	return
}

func (gen *ModifiersOfQGen) FyQueryInsertSQL(collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ResultTypeFy {
		panic(fmt.Sprintf("invalid worker result type for ModifiersOfQGen.Fy: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query_%s (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.CorpusName, gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr,
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

func (gen *ModifiersOfQGen) FxyQuery(word Word, collCandidate string) string {
	if word.PoS == "" {
		return fmt.Sprintf(
			"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
			gen.SketchConf.ParLemmaAttr, word.V,
			gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
			gen.SketchConf.PosAttr, gen.SketchConf.NounValue,
			gen.SketchConf.LemmaAttr, collCandidate,
		)
	}
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.ParLemmaAttr, word.V,
		gen.SketchConf.ParPosAttr, word.PoS,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.PosAttr, gen.SketchConf.NounValue,
		gen.SketchConf.LemmaAttr, collCandidate,
	)
}

func (gen *ModifiersOfQGen) FxyQuerySelectSQL(word Word, collCandidate string) (sql string, args []any) {
	if word.PoS == "" {
		sql = fmt.Sprintf(
			"SELECT result, result_type FROM scoll_query_%s "+
				"WHERE result_type = 'Fxy' AND %s = ? AND %s IS NULL AND %s = ? AND %s = ? AND %s = ? ",
			gen.CorpusName,
			gen.SketchConf.ParLemmaAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.LemmaAttr,
		)
		args = append(
			args,
			word.V, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, collCandidate,
		)
		return
	}
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM scoll_query_%s "+
			"WHERE result_type = 'Fxy' AND %s = ? AND %s = ? AND %s = ? AND %s = ? AND %s = ? ",
		gen.CorpusName,
		gen.SketchConf.ParLemmaAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.FuncAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.LemmaAttr,
	)
	args = append(
		args,
		word.V, word.PoS, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, collCandidate,
	)
	return

}

func (gen *ModifiersOfQGen) FxyQueryInsertSQL(word Word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ResultTypeFxy {
		panic(fmt.Sprintf("invalid worker result type for ModifiersOfQGen.Fxy: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO scoll_query_%s (%s, %s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?, ?)",
		gen.CorpusName, gen.SketchConf.ParLemmaAttr, gen.SketchConf.ParPosAttr, gen.SketchConf.LemmaAttr, gen.SketchConf.FuncAttr, gen.SketchConf.PosAttr,
	)
	var posValue sqlLib.NullString
	if word.PoS != "" {
		posValue.String = word.PoS
		posValue.Valid = true
	}
	args = append(
		args,
		word.V,
		posValue,
		collCandidate,
		gen.SketchConf.NounModifiedValue,
		gen.SketchConf.NounValue,
		result.Value,
		result.ResultType,
	)
	return
}
