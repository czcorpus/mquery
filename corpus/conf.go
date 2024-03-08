// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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

package corpus

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

const (
	DfltSplitChunkSize = 100000000
	DfltMaximumRecords = 50
)

type PosAttr struct {
	Name string `json:"name"`
}

type PosAttrList []PosAttr

func (pal PosAttrList) GetIDs() []string {
	ans := make([]string, len(pal))
	for i, v := range pal {
		ans[i] = v.Name
	}
	return ans
}

type StructAttr struct {
	Name string `json:"name"`
}

type SyntaxConcordance struct {
	ParentAttr string `json:"parentAttr"`

	// ResultAttrs is a list of positional attributes
	// we need to provide all the required information about
	// syntax in for the "syntax-conc-examples" endpoint
	ResultAttrs []string `json:"resultAttrs"`
}

type TextTypes map[string][]string

type Subcorpus struct {
	ID          string            `json:"id"`
	TextTypes   TextTypes         `json:"textTypes"`
	Description map[string]string `json:"description"`
}

type CorpusVariant struct {
	ID          string            `json:"id"`
	FullName    map[string]string `json:"fullName"`
	Description map[string]string `json:"description"`
}

type CorpusSetup struct {
	ID                string               `json:"id"`
	FullName          map[string]string    `json:"fullName"`
	Description       map[string]string    `json:"description"`
	SyntaxConcordance SyntaxConcordance    `json:"syntaxConcordance"`
	PosAttrs          PosAttrList          `json:"posAttrs"`
	StructAttrs       []StructAttr         `json:"structAttrs"`
	MaximumRecords    int                  `json:"maximumRecords"`
	TTOverviewAttrs   []string             `json:"ttOverviewAttrs"`
	Subcorpora        map[string]Subcorpus `json:"subcorpora"`
	// ViewContextStruct is a structure used to specify "units"
	// for KWIC left and right context. Typically, this is
	// a structure representing a sentence or a speach.
	ViewContextStruct string                   `json:"viewContextStruct"`
	Variants          map[string]CorpusVariant `json:"variants"`
}

func (cs *CorpusSetup) IsDynamic() bool {
	return strings.Contains(cs.ID, "*")
}

func (cs *CorpusSetup) ValidateAndDefaults() error {
	if cs.IsDynamic() {
		for _, variant := range cs.Variants {
			if len(variant.FullName) == 0 || variant.FullName["en"] == "" {
				return fmt.Errorf("missing corpus variant `fullName`, at least `en` value must be set")
			}
		}

	} else {
		if len(cs.FullName) == 0 || cs.FullName["en"] == "" {
			return fmt.Errorf("missing corpus `fullName`, at least `en` value must be set")
		}
	}
	if len(cs.PosAttrs) == 0 {
		return fmt.Errorf("at least one positional attribute in `posAttrs` must be defined")
	}
	if cs.MaximumRecords == 0 {
		cs.MaximumRecords = DfltMaximumRecords
		log.Warn().
			Int("value", cs.MaximumRecords).
			Msg("missing or zero `maximumRecords`, using default")
	}
	if len(cs.TTOverviewAttrs) == 0 {
		log.Warn().
			Msg("no `ttOverviewAttrs` defined, some freq. function will be disabled")
	}
	return nil
}

type Resources []*CorpusSetup

func (rscs Resources) Get(name string) *CorpusSetup {
	for _, v := range rscs {
		if strings.Contains(v.ID, "*") {
			ptrn := regexp.MustCompile(strings.ReplaceAll(v.ID, "*", ".*"))
			if ptrn.MatchString(name) {
				if v.Variants != nil {
					variant, ok := v.Variants[name]
					if ok {
						// make a copy of CorpusSetup and replace values for specific variant
						merged := *v
						merged.Variants = nil
						merged.ID = variant.ID
						merged.FullName = variant.FullName
						merged.Description = variant.Description
						return &merged
					}
				}
				return v
			}

		} else if v.ID == name {
			return v
		}
	}
	return nil
}

// CorporaSetup defines mquery application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDir     string `json:"registryDir"`
	SplitCorporaDir string `json:"splitCorporaDir"`

	// MultiprocChunkSize defines a subcorpus size for large
	// corpora when processed in a parallel way.
	// Please note that once created, the subcorpora will be
	// applied in their original way until explicitly removed.
	// I.e. the value only affects newly created splits.
	MultiprocChunkSize int64 `json:"multiprocChunkSize"`

	MktokencovPath string `json:"mktokencovPath"`

	Resources Resources `json:"resources"`
}

func (cs *CorporaSetup) GetRegistryPath(corpusID string) string {
	return filepath.Join(cs.RegistryDir, corpusID)
}

func (cs *CorporaSetup) ValidateAndDefaults(confContext string) error {
	if cs == nil {
		return fmt.Errorf("missing configuration section `%s`", confContext)
	}
	if cs.RegistryDir == "" {
		return fmt.Errorf("missing `%s.registryDir`", confContext)
	}
	isDir, err := fs.IsDir(cs.RegistryDir)
	if err != nil {
		return fmt.Errorf("failed to test `%s.registryDir`: %w", confContext, err)
	}
	if !isDir {
		return fmt.Errorf("`%s.registryDir` is not a directory", confContext)
	}
	if cs.SplitCorporaDir == "" {
		return fmt.Errorf("missing `%s.splitCorporaDir`", confContext)
	}
	isDir, err = fs.IsDir(cs.SplitCorporaDir)
	if err != nil {
		return fmt.Errorf("failed to test `%s.splitCorporaDir`: %w", confContext, err)
	}
	if !isDir {
		return fmt.Errorf("`%s.splitCorporaDir` is not a directory", confContext)
	}

	if cs.MultiprocChunkSize == 0 {
		log.Warn().
			Int("value", DfltSplitChunkSize).
			Msgf("`%s.multiprocChunkSize` not set, using default", confContext)
	}

	isFile, err := fs.IsFile(cs.MktokencovPath)
	if err != nil {
		return fmt.Errorf("failed to test `%s.mktokencovPath` file %w", confContext, err)
	}
	if !isFile {
		return fmt.Errorf("the `%s.mktokencovPath` does not point to a file", confContext)
	}
	for _, v := range cs.Resources {
		if err := v.ValidateAndDefaults(); err != nil {
			return err
		}
	}
	return nil
}
