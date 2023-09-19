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
	"fmt"
	"mquery/rdb"
	"mquery/results"

	sqlLib "database/sql"
)

type VerbSubjectQGen struct {
	SketchConf *CorpusSketchSetup
	CorpusName string
}

func (gen *VerbSubjectQGen) FxQuery(word Word) string {
	if word.PoS == "" {
		return fmt.Sprintf(
			"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
			gen.SketchConf.LemmaAttr.Name, word.V,
			gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounSubjectValue,
			gen.SketchConf.ParPosAttr.Name, gen.SketchConf.VerbValue,
		)
	}
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr.Name, word.V,
		gen.SketchConf.PosAttr.Name, word.PoS,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounSubjectValue,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.VerbValue,
	)

}

func (gen *VerbSubjectQGen) FxQuerySelectSQL(word Word) (sql string, args []any) {
	if word.PoS == "" {
		sql = fmt.Sprintf("SELECT f.result, f.result_type FROM %s_scoll_query AS q "+
			"JOIN %s_scoll_fcrit AS f ON q.id = f.scoll_query_id "+
			"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s IS NULL AND q.%s = ? AND q.%s = ? AND f.attr = ?",
			gen.CorpusName, gen.CorpusName,
			gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
			gen.SketchConf.ParLemmaAttr.Name)
		args = []any{
			word.V,
			gen.SketchConf.NounSubjectValue,
			gen.SketchConf.VerbValue,
			gen.SketchConf.ParLemmaAttr.Name,
		}
		return
	}
	sql = fmt.Sprintf("SELECT f.result, f.result_type FROM %s_scoll_query AS q "+
		"JOIN %s_scoll_fcrit AS f ON q.id = f.scoll_query_id "+
		"WHERE q.result_type = 'Fx' AND q.%s = ? AND q.%s = ? AND q.%s = ? AND q.%s = ? AND f.attr = ?",
		gen.CorpusName, gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
		gen.SketchConf.ParLemmaAttr.Name)
	args = []any{
		word.V,
		word.PoS,
		gen.SketchConf.NounSubjectValue,
		gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr.Name,
	}
	return
}

func (gen *VerbSubjectQGen) FxQueryInsertSQL(word Word, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.ResultTypeFx {
		panic("invalid worker result type for VerbSubjectQGen")
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
		gen.SketchConf.NounSubjectValue,
		gen.SketchConf.VerbValue,
		val,
		rType,
	}
	return
}

func (gen *VerbSubjectQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr.Name)
}

func (gen *VerbSubjectQGen) FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (sql string, args []any) {
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

func (gen *VerbSubjectQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		`[%s="%s" & %s="%s" & %s="%s"]`,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounSubjectValue,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr.Name, collCandidate,
	)
}

func (gen *VerbSubjectQGen) FyQuerySelectSQL(collCandidates []string) (sql string, args []any) {
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
	args = append(args, gen.SketchConf.NounSubjectValue, gen.SketchConf.VerbValue)
	for _, v := range collCandidates {
		args = append(args, v)
	}
	return
}

func (gen *VerbSubjectQGen) FyQueryInsertSQL(collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.ResultTypeFy {
		panic("invalid worker result type for VerbSubjectQGen")
	}
	sql = fmt.Sprintf(
		"INSERT INTO %s_scoll_query (%s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?)",
		gen.CorpusName,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.ParPosAttr.Name, gen.SketchConf.ParLemmaAttr.Name,
	)
	args = append(
		args,
		gen.SketchConf.NounSubjectValue,
		gen.SketchConf.VerbValue,
		collCandidate,
		result.Value,
		result.ResultType,
	)
	return
}

func (gen *VerbSubjectQGen) FxyQuery(word Word, collCandidate string) string {
	if word.PoS == "" {
		return fmt.Sprintf(
			`[%s="%s" & %s="%s" & %s="%s" & %s="%s"]`,
			gen.SketchConf.LemmaAttr.Name, word.V,
			gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounSubjectValue,
			gen.SketchConf.ParPosAttr.Name, gen.SketchConf.VerbValue,
			gen.SketchConf.ParLemmaAttr.Name, collCandidate,
		)
	}
	return fmt.Sprintf(
		`[%s="%s" & %s="%s" & %s="%s" & %s="%s" & %s="%s"]`,
		gen.SketchConf.LemmaAttr.Name, word.V,
		gen.SketchConf.PosAttr, word.PoS,
		gen.SketchConf.FuncAttr.Name, gen.SketchConf.NounSubjectValue,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr.Name, collCandidate,
	)
}

func (gen *VerbSubjectQGen) FxyQuerySelectSQL(word Word, collCandidate string) (sql string, args []any) {
	if word.PoS == "" {
		sql = fmt.Sprintf(
			"SELECT result, result_type FROM %s_scoll_query "+
				" WHERE result_type = 'Fxy' AND %s = ? AND %s IS NULL AND %s = ? AND %s = ? AND %s = ? ",
			gen.CorpusName,
			gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
			gen.SketchConf.ParPosAttr.Name, gen.SketchConf.ParLemmaAttr.Name,
		)
		args = append(args, word.V, gen.SketchConf.NounSubjectValue, gen.SketchConf.VerbValue, collCandidate)
		return
	}
	sql = fmt.Sprintf(
		"SELECT result, result_type FROM %s_scoll_query "+
			" WHERE result_type = 'Fxy' AND %s = ? AND %s = ? AND %s = ? AND %s = ? AND %s = ? ",
		gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
		gen.SketchConf.ParPosAttr.Name,
		gen.SketchConf.ParLemmaAttr.Name,
	)
	args = append(
		args,
		word.V, word.PoS, gen.SketchConf.NounSubjectValue, gen.SketchConf.VerbValue, collCandidate,
	)
	return
}

func (gen *VerbSubjectQGen) FxyQueryInsertSQL(word Word, collCandidate string, result *rdb.WorkerResult) (sql string, args []any) {
	if result != nil && result.ResultType != results.ResultTypeFxy {
		panic("invalid worker result type for VerbSubjectQGen")
	}
	sql = fmt.Sprintf(
		"INSERT INTO %s_scoll_query (%s, %s, %s, %s, %s, result, result_type) VALUES (?, ?, ?, ?, ?, ?, ?)",
		gen.CorpusName,
		gen.SketchConf.LemmaAttr.Name, gen.SketchConf.PosAttr.Name, gen.SketchConf.FuncAttr.Name,
		gen.SketchConf.ParPosAttr.Name, gen.SketchConf.ParLemmaAttr.Name,
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
		gen.SketchConf.NounSubjectValue,
		gen.SketchConf.VerbValue,
		collCandidate,
		result.Value,
		result.ResultType,
	)
	return
}
