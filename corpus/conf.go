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
	"errors"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const (
	DfltSplitChunkSize = 100000000
)

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
}

func (cs *CorporaSetup) GetRegistryPath(corpusID string) string {
	return filepath.Join(cs.RegistryDir, corpusID)
}

func (cs *CorporaSetup) DefaultsAndValidate() error {
	if cs.RegistryDir == "" {
		return errors.New("missing `corporaSetup.registryDir`")
	}
	if cs.SplitCorporaDir == "" {
		return errors.New("missing `corporaSetup.splitCorporaDir`")
	}
	if cs.MultiprocChunkSize == 0 {
		log.Warn().
			Int("value", DfltSplitChunkSize).
			Msg("`corporaSetup.multiprocChunkSize` not set, using default")
	}
	return nil
}
