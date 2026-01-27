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

package corpus

import (
	"errors"
	"fmt"
	"mquery/rdb"
	"os"
	"path/filepath"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

var (
	ErrNotFound = errors.New("corpus not found")
)

// ------------------ split corpus (into multiple subcorpora) -------------------------

type SplitCorpus struct {
	CorpusPath string
	Subcorpora []string
}

func (sc *SplitCorpus) GetSubcorpora() []string {
	return sc.Subcorpora
}

func OpenSplitCorpus(subcBaseDir, corpPath string) (*SplitCorpus, error) {
	ans := &SplitCorpus{
		CorpusPath: corpPath,
		Subcorpora: make([]string, 0, 30),
	}
	corpName := filepath.Base(corpPath)
	p := filepath.Join(subcBaseDir, corpName)
	files, err := os.ReadDir(p)
	if err != nil {
		return ans, fmt.Errorf("failed to open split corpus: %w", err)
	}
	for _, item := range files {
		suff := filepath.Ext(item.Name())
		if suff == ".subc" {
			ans.Subcorpora = append(ans.Subcorpora, filepath.Join(p, item.Name()))
		}
	}
	return ans, nil
}

// --------------- saved subcorpus ----------------------------

func CheckSavedSubcorpus(baseDir, corp, subcID string) (string, bool) {
	path := filepath.Join(baseDir, subcID[:2], subcID, "data.subc")
	isf, err := fs.IsFile(path)
	if err != nil {
		log.Error().Err(err).Msg("failed to check saved subcorpus path")
		return path, false
	}
	return path, isf
}

// ------------------------------------------------------------

type QueryHandler interface {
	// PublishQuery sends a query to a worker and returns a channel where
	// where the sender can wait for the result.
	// The workerTimeout value can be 0 (or even negative) in which case,
	// default configured value is used instead. I.e. there is no way
	// how to make a query with an infinite timeout.
	PublishQuery(query rdb.Query, workerTimeout time.Duration) (<-chan rdb.WorkerResult, error)
}
