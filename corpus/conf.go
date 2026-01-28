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
	"os"
	"path/filepath"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

const (
	DfltSplitChunkSize            = 100000000
	DfltPosAttrDelimiter          = 47
	DfltMaximumRecords            = 50
	DfltMaximumTokenContextWindow = 50
)

type PosAttrDelimiter int

func (pad PosAttrDelimiter) Validate() error {
	if pad != 9 && pad != 31 && pad != 47 {
		return fmt.Errorf("unsupported delimiter ascii code (supported values are: 9, 31, 47)")
	}
	return nil
}

func (pad PosAttrDelimiter) AsString() string {
	switch pad {
	case 9:
		return "\x09"
	case 31:
		return "\x1f"
	case 47:
		return "/"
	default:
		panic(fmt.Errorf("unsupported value for PosAttrDelimiter: %d", pad))
	}
}

// CorporaSetup defines a root configuration of corpora
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

	ConfFilesDir       string    `json:"confFilesDir"`
	Resources          Resources `json:"resources"`
	SavedSubcorporaDir string    `json:"savedSubcorporaDir"`
	AudioFilesDir      string    `json:"audioFilesDir"`
	ZeroConfCorpora    bool      `json:"zeroConfCorpora"`

	autoConfCache map[string]*MQCorpusSetup
}

// GetCorp returns a corpus configuration.
// If zeroConfCorpora is enabled, the function may panic
// if it cannot produce a configuration based on an inferred
// registry file.
func (cs *CorporaSetup) GetCorp(corpusID string) *MQCorpusSetup {
	if c := cs.Resources.get(corpusID); c != nil {
		return c
	}
	if !cs.ZeroConfCorpora {
		return nil
	}
	if cs.autoConfCache == nil {
		cs.autoConfCache = make(map[string]*MQCorpusSetup)
	}
	autoConf, ok := cs.autoConfCache[corpusID]
	if ok {
		return autoConf
	}
	autoConf = AutogenerateConf(cs.RegistryDir, corpusID)
	cs.autoConfCache[corpusID] = autoConf
	return autoConf
}

func (cs *CorporaSetup) safeGetCorp(corpusID string) *MQCorpusSetup {
	defer func() {
		if err := recover(); err != nil {
			log.Error().Str("corpus", corpusID).Msg("failed to get corpus info for listing")
		}
	}()
	return cs.GetCorp(corpusID)
}

func (cs *CorporaSetup) GetAllCorpora(substrFilter string) []*MQCorpusSetup {
	if !cs.ZeroConfCorpora {
		ans := make([]*MQCorpusSetup, 0, len(cs.Resources)*3)
		for _, v := range cs.Resources {
			if len(v.Variants) > 0 {
				for _, variant := range v.Variants {
					item := cs.Resources.get(variant.ID)
					ans = append(ans, item)
				}

			} else {
				ans = append(ans, v)
			}
		}
		return ans

	} else {
		files, err := os.ReadDir(cs.RegistryDir)
		if err != nil {
			panic(fmt.Errorf("failed to get list of registry files: %w", err))
		}
		ans := make([]*MQCorpusSetup, 0, len(files))
		for _, f := range files {
			if substrFilter == "" || strings.Contains(strings.ToLower(f.Name()), strings.ToLower(substrFilter)) {
				item := cs.safeGetCorp(f.Name())
				if item != nil {
					ans = append(ans, item)
				}
			}
		}
		return ans
	}
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
