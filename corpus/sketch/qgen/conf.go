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

type SketchConfig map[string]*CorpusSketchSetup

type SketchSetup struct {
	SketchAttrs SketchConfig `json:"sketchAttrs"`
}

type CorpusSketchSetup struct {

	// LemmaAttr - an attribute specifying lemma
	// (in intercorp_v13ud: `lemma`)
	LemmaAttr string `json:"lemmaAttr"`

	// ParLemmaAttr - an attribute specifying lemma in parent
	// (in intercorp_v13ud: `p_lemma`)
	ParLemmaAttr string `json:"parLemmaAttr"`

	// PosAttr - an attr specifying part of speech
	// (in intercorp_v13ud: `upos`)
	PosAttr string `json:"posAttr"`

	// ParPosAttr - an attr specifying part of speech in parent
	// (in intercorp_v13ud: `p_upos`)
	ParPosAttr string `json:"parPosAttr"`

	// (in intercorp_v13ud: `deprel`)
	FuncAttr string `json:"funcAttr"`

	// (in intercorp_v13ud: `NOUN`)
	NounValue string `json:"nounPosValue"`

	// (in intercorp_v13ud: `VERB`)
	VerbValue string `json:"verbPosValue"`

	// (in intercorp_v13ud: `nmod`)
	NounModifiedValue string `json:"nounModifiedValue"`

	// (in intercorp_v13ud: `nsubj`)
	NounSubjectValue string `json:"nounSubjectValue"`

	// (in intercorp_v13ud: `obj|iobj`)
	NounObjectValue string `json:"nounObjectValue"`
}
