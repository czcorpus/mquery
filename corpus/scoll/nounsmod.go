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

// NounsModifiedByQGen
// example:
// Fx: [lemma="team" & deprel="nmod" & p_upos="NOUN"]
// Fy: [p_lemma="value" & deprel="nmod" & p_upos="NOUN"]
// Fxy: [lemma="team" & p_lemma="value" & deprel="nmod" & p_upos="NOUN"]
type NounsModifiedByQGen struct {
	SketchConf *CorpusSketchSetup
	CorpusName string
}

func (gen *NounsModifiedByQGen) FxQuery(word Word) string {
	if word.PoS == "" {
		return fmt.Sprintf(
			"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
			gen.SketchConf.LemmaAttr.Name, word.V,
			gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounModifiedValue,
			gen.SketchConf.ParPosAttr.Name, gen.SketchConf.NounValue,
		)
	}
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr.Name, word.V,
		gen.SketchConf.PosAttr.Name, word.PoS,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.NounValue,
	)
}

func (gen *NounsModifiedByQGen) FxQuerySelectSQL(word Word) (sql string, args []any) {
	if word.PoS == "" {
		sql = fmt.Sprintf(
			"SELECT f.result, f.result_type FROM %s_scoll_query AS q "+
				"JOIN %s_scoll_fcrit AS f ON q.id = f.scoll_query_id "+
				"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s IS NULL AND q.%s = ? AND q.%s = ? AND f.attr = ?",
			gen.CorpusName, gen.CorpusName,
			gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
			gen.SketchConf.ParPosAttr.Name,
		)
		args = append(
			args,
			word.V, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, gen.SketchConf.ParLemmaAttr.Name,
		)
		return
	}
	sql = fmt.Sprintf(
		"SELECT f.result, f.result_type FROM %s_scoll_query AS q "+
			"JOIN %s_scoll_fcrit AS f ON q.id = f.scoll_query_id "+
			"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s = ? AND q.%s = ? AND q.%s = ? AND f.attr = ?",
		gen.CorpusName, gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
		gen.SketchConf.ParPosAttr.Name,
	)
	args = append(
		args,
		word.V, word.PoS, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue, gen.SketchConf.ParLemmaAttr.Name,
	)
	return
}

func (gen *NounsModifiedByQGen) FxQueryInsertSQL(word Word, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.ResultTypeFx {
		panic(fmt.Sprintf("invalid worker result type for NounsModifiedByQGen.Fx: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO %s_scoll_query (%s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?)",
		gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
		gen.SketchConf.ParPosAttr.Name,
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

func (gen *NounsModifiedByQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr.Name)
}

func (gen *NounsModifiedByQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
	sql = fmt.Sprintf(
		"INSERT INTO %s_scoll_fcrit (scoll_query_id, attr, result, result_type) VALUES (?, ?, ?, ?)",
		gen.CorpusName,
	)
	args = append(
		args,
		query_id,
		gen.SketchConf.ParLemmaAttr.Name,
		result.Value,
		result.ResultType,
	)
	return
}

func (gen *NounsModifiedByQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.ParLemmaAttr.Name, collCandidate,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.NounValue,
	)
}

func (gen *NounsModifiedByQGen) FyQuerySelectSQL(collCandidates []string) (sql string, args []any) {
	placeholders := ""
	for i, _ := range collCandidates {
		placeholders += "?"
		if i+1 < len(collCandidates) {
			placeholders += ","
		}
	}

	sql = fmt.Sprintf(
		"SELECT %s AS id, result, result_type FROM %s_scoll_query "+
			"WHERE result_type = 'Fy' AND %s = ? AND %s = ? AND %s IN (%s) ",
		gen.SketchConf.ParLemmaAttr.Name, gen.CorpusName, gen.SketchConf.FuncAttr.Name, gen.SketchConf.ParPosAttr.Name, gen.SketchConf.ParLemmaAttr.Name, placeholders,
	)
	args = append(args, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue)
	for _, v := range collCandidates {
		args = append(args, v)
	}
	return
}

func (gen *NounsModifiedByQGen) FyQueryInsertSQL(collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ResultTypeFy {
		panic(fmt.Sprintf("invalid worker result type for NounsModifiedByQGen.Fy: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO %s_scoll_query (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.CorpusName,
		gen.SketchConf.ParLemmaAttr.Name, gen.SketchConf.FuncAttr.Name, gen.SketchConf.ParPosAttr.Name,
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

func (gen *NounsModifiedByQGen) FxyQuery(word Word, collCandidate string) string {
	if word.PoS == "" {
		return fmt.Sprintf(
			"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
			gen.SketchConf.LemmaAttr.Name, word.V,
			gen.SketchConf.ParLemmaAttr.Name, collCandidate,
			gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounModifiedValue,
			gen.SketchConf.ParPosAttr.Name, gen.SketchConf.NounValue,
		)
	}
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr.Name, word.V,
		gen.SketchConf.PosAttr.Name, word.PoS,
		gen.SketchConf.ParLemmaAttr.Name, collCandidate,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounModifiedValue,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.NounValue,
	)
}

func (gen *NounsModifiedByQGen) FxyQuerySelectSQL(word Word, collCandidate string) (sql string, args []any) {
	if word.PoS == "" {
		sql = fmt.Sprintf(
			"SELECT result, result_type FROM %s_scoll_query "+
				"WHERE result_type = 'Fxy' AND %s = ? AND %s IS NULL AND %s = ? AND %s = ? AND %s = ? ",
			gen.CorpusName,
			gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.ParLemmaAttr.Name,
			gen.SketchConf.FuncAttr.Name, gen.SketchConf.ParPosAttr.Name,
		)
		args = append(args, word.V, collCandidate, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue)
		return
	}
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM %s_scoll_query "+
			"WHERE result_type = 'Fxy' AND %s = ? AND %s = ? AND %s = ? AND %s = ? AND %s = ? ",
		gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.ParLemmaAttr.Name,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.ParPosAttr.Name,
	)
	args = append(args, word.V, word.PoS, collCandidate, gen.SketchConf.NounModifiedValue, gen.SketchConf.NounValue)
	return
}

func (gen *NounsModifiedByQGen) FxyQueryInsertSQL(word Word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result.ResultType != results.ResultTypeFxy {
		panic(fmt.Sprintf("invalid worker result type for NounsModifiedByQGen.Fxy: %s", result.ResultType))
	}
	sql = fmt.Sprintf(
		"INSERT INTO %s_scoll_query (%s, %s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?, ?)",
		gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.ParLemmaAttr.Name,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.ParPosAttr.Name,
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
