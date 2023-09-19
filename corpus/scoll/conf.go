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

	"github.com/rs/zerolog/log"
)

const (
	DfltCollPreliminarySelSize = 20
	DfltCollResultSize         = 10
	DfltNumParallelChunks      = 2
)

type Word struct {
	V   string
	PoS string
}

func (w Word) IsValid() bool {
	return w.V != ""
}

type SketchConfig map[string]*CorpusSketchSetup

type SketchSetup struct {
	SketchAttrs            SketchConfig `json:"sketchAttrs"`
	CollPreliminarySelSize int          `json:"collPreliminarySelSize"`
	CollResultSize         int          `json:"collResultSize"`

	// NumParallelChunks specifies how many parallel chunks should
	// be solved when dealing with Fx or Fxy. Please note that
	// due to the fact that there are two functions (Fx, Fxy), MQuery
	// will actually create twice as many goroutines when calculating
	// required collocations.
	NumParallelChunks int `json:"numParallelChunks"`
}

func (setup *SketchSetup) ValidateAndDefaults(confContext string) error {
	if setup.CollPreliminarySelSize == 0 {
		log.Warn().
			Int("value", DfltCollPreliminarySelSize).
			Msgf("`%s.collPreliminarySelSize` not set, using default", confContext)
		setup.CollPreliminarySelSize = DfltCollPreliminarySelSize
	}
	if setup.CollResultSize == 0 {
		log.Warn().
			Int("value", DfltCollResultSize).
			Msgf("`%s.collResultSize` not set, using default", confContext)
		setup.CollResultSize = DfltCollResultSize
	}
	if setup.NumParallelChunks == 0 {
		log.Warn().
			Int("value", DfltNumParallelChunks).
			Msgf("`%s.numParallelChunks` not set, using default", confContext)
		setup.NumParallelChunks = DfltNumParallelChunks
	}
	for k, corpSetup := range setup.SketchAttrs {
		err := corpSetup.ValidateAndDefaults(fmt.Sprintf("%s.sketchAttrs.%s", confContext, k))
		if err != nil {
			return err
		}
	}
	return nil
}

type PosAttrProps struct {
	Name        string `json:"name"`
	VerticalCol int    `json:"verticalCol"`
}

type CorpusSketchSetup struct {

	// ParentIdxAttr specifies a positional attribute providing
	// information about relative position of a parent token.
	ParentIdxAttr PosAttrProps `json:"parentIdxAttr"`

	// LemmaAttr - an attribute specifying lemma
	// (in intercorp_v13ud: `lemma`)
	LemmaAttr PosAttrProps `json:"lemmaAttr"`

	// ParLemmaAttr - an attribute specifying lemma in parent
	// (in intercorp_v13ud: `p_lemma`)
	ParLemmaAttr PosAttrProps `json:"parLemmaAttr"`

	// PosAttr - an attr specifying part of speech
	// (in intercorp_v13ud: `upos`)
	PosAttr PosAttrProps `json:"posAttr"`

	// ParPosAttr - an attr specifying part of speech in parent
	// (in intercorp_v13ud: `p_upos`)
	ParPosAttr PosAttrProps `json:"parPosAttr"`

	// (in intercorp_v13ud: `deprel`)
	FuncAttr PosAttrProps `json:"funcAttr"`

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

func (conf *CorpusSketchSetup) ValidateAndDefaults(confContext string) error {
	if conf.ParentIdxAttr.Name == "" {
		return fmt.Errorf("missing `%s.parentIdxAttr`", confContext)
	}
	if conf.LemmaAttr.Name == "" {
		return fmt.Errorf("missing `%s.lemmaAttr`", confContext)
	}
	if conf.ParLemmaAttr.Name == "" {
		return fmt.Errorf("missing `%s.parLemmaAttr`", confContext)
	}
	if conf.PosAttr.Name == "" {
		return fmt.Errorf("missing `%s.posAttr`", confContext)
	}
	if conf.ParPosAttr.Name == "" {
		return fmt.Errorf("missing `%s.parPosAttr`", confContext)
	}
	if conf.FuncAttr.Name == "" {
		return fmt.Errorf("missing `%s.funcAttr`", confContext)
	}
	if conf.NounValue == "" {
		return fmt.Errorf("missing `%s.nounPosValue`", confContext)
	}
	if conf.VerbValue == "" {
		return fmt.Errorf("missing `%s.verbPosValue`", confContext)
	}
	if conf.NounModifiedValue == "" {
		return fmt.Errorf("missing `%s.nounModifiedValue`", confContext)
	}
	if conf.NounSubjectValue == "" {
		return fmt.Errorf("missing `%s.nounSubjectValue`", confContext)
	}
	if conf.NounObjectValue == "" {
		return fmt.Errorf("missing `%s.nounObjectValue`", confContext)
	}
	return nil
}
