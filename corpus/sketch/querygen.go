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

package sketch

import "fmt"

type QueryGenerator interface {
	FxQuery(word string) string
	FxCrit() string
	FyQuery(collCandidate string) string
	FxyQuery(word, collCandidate string) string
}

// ------

type VerbSubjectQGen struct {
	SketchConf *CorpusSketchSetup
}

func (gen *VerbSubjectQGen) FxQuery(word string) string {
	return fmt.Sprintf(
		"[%s=\"%s\" & %s=\"%s\" & %s=\"%s\"]",
		gen.SketchConf.LemmaAttr, word,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounSubjectValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.VerbValue,
	)
}

func (gen *VerbSubjectQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *VerbSubjectQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		`[%s="%s" & %s="%s" & %s="%s"]`,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounSubjectValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr, collCandidate,
	)
}

func (gen *VerbSubjectQGen) FxyQuery(word, collCandidate string) string {
	return fmt.Sprintf(
		`[%s="%s" & %s="%s" & %s="%s" & %s="%s"]`,
		gen.SketchConf.LemmaAttr, word,
		gen.SketchConf.FuncAttr, gen.SketchConf.NounSubjectValue,
		gen.SketchConf.ParPosAttr, gen.SketchConf.VerbValue,
		gen.SketchConf.ParLemmaAttr, collCandidate,
	)
}

// ------

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

func (gen *VerbObjectQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *VerbObjectQGen) FyQuery(collCandidate string) string {
	return fmt.Sprintf(
		`[deprel="%s" & p_upos="%s" & p_lemma="%s"]`,
		gen.SketchConf.NounObjectValue,
		gen.SketchConf.VerbValue,
		collCandidate,
	)
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

// ------

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

func (gen *NounsModifiedByQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.ParLemmaAttr)
}

func (gen *NounsModifiedByQGen) FyQuery(collCandidate string) string {
	return ""
}

func (gen *NounsModifiedByQGen) FxyQuery(word, collCandidate string) string {
	return ""
}

// ------

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

func (gen *ModifiersOfQGen) FxCrit() string {
	return fmt.Sprintf("%s/i 0~0>0", gen.SketchConf.LemmaAttr)
}

func (gen *ModifiersOfQGen) FyQuery(collCandidate string) string {
	return ""
}

func (gen *ModifiersOfQGen) FxyQuery(word, collCandidate string) string {
	return ""
}

// ------

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
