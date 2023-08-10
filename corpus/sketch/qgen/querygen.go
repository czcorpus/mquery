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

import "mquery/rdb"

const (
	QueryNounsModifiedBy QueryType = iota
	QueryModifiersOf
	QueryVerbsSubject
	QueryVerbsObject
)

type QueryType int

// QueryGenerator defines an object capable of producing
// both CQL and SQL queries for a single "sketch" (e.g. 'nouns modified by...')
//
// Please note that while the `word` argument has its own type `Word` (to include
// also the optional PoS), the `collCandidate` is just a string. This is due to the
// fact, that the PoS of collocation candidates is specified in queries themselves.
// E.g. in Fx `[lemma="team" & deprel="nmod" & p_upos="NOUN"]` we look for NOUNs which
// means that in Fy, Fxy, the respective words for `collCandidate` are already NOUNs.
type QueryGenerator interface {
	FxQuery(word Word) string
	FxQuerySelectSQL(word Word) (string, []any)
	FxQueryInsertSQL(word Word, result *rdb.WorkerResult) (string, []any)
	FxCrit() string
	FxCritInsertSQL(query_id int64, result *rdb.WorkerResult) (string, []any) // we need `word` here to be able to join tables

	FyQuery(collCandidate string) string
	FyQuerySelectSQL(collCandidate string) (string, []any)
	FyQueryInsertSQL(collCandidate string, result *rdb.WorkerResult) (string, []any)

	FxyQuery(word Word, collCandidate string) string
	FxyQuerySelectSQL(word Word, collCandidate string) (string, []any)
	FxyQueryInsertSQL(word Word, collCandidate string, result *rdb.WorkerResult) (string, []any)
}

func NewQueryGenerator(qType QueryType, conf *CorpusSketchSetup) QueryGenerator {
	switch qType {
	case QueryNounsModifiedBy:
		return &NounsModifiedByQGen{conf}
	case QueryModifiersOf:
		return &ModifiersOfQGen{conf}
	case QueryVerbsSubject:
		return &VerbSubjectQGen{conf}
	case QueryVerbsObject:
		return &VerbObjectQGen{conf}
	default:
		panic("invalid query type for QGenFactory")
	}
}
