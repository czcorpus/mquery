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

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

const (
	DfltSplitChunkSize       = 100000000
	DfltMultisampledSubcSize = 100000000
)

type PosAttrProps struct {
	Name string `json:"name"`
}

type CorpusSetup struct {
	SyntaxParentAttr PosAttrProps `json:"syntaxParentAttr"`
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

	// MultisampledCorporaDir serves for an experimental collocation
	// calculation module where multiple calculations are performed
	// on random samples (= subcorpora)
	MultisampledCorporaDir string `json:"multisampledCorporaDir"`

	MultisampledSubcSize int64 `json:"multisampledSubcSize"`

	MktokencovPath string `json:"mktokencovPath"`

	Resources map[string]*CorpusSetup `json:"resources"`
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

	if cs.MultisampledSubcSize == 0 {
		log.Warn().
			Int("value", DfltMultisampledSubcSize).
			Msgf("`%s.multisampledSubcSize` not set, using default", confContext)
	}

	isFile, err := fs.IsFile(cs.MktokencovPath)
	if err != nil {
		return fmt.Errorf("failed to test `%s.mktokencovPath` file %w", confContext, err)
	}
	if !isFile {
		return fmt.Errorf("the `%s.mktokencovPath` does not point to a file", confContext)
	}
	return nil
}
