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
)

var (
	ErrNotFound = errors.New("corpus not found")
)

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

type MultisampledCorpus struct {
	CorpusPath string
	Subcorpora []string
}

func (mc *MultisampledCorpus) GetSubcorpora() []string {
	return mc.Subcorpora
}

func OpenMultisampledCorpus(subcBaseDir, corpPath string) (*MultisampledCorpus, error) {
	ans := &MultisampledCorpus{
		CorpusPath: corpPath,
		Subcorpora: make([]string, 0, 30),
	}
	corpName := filepath.Base(corpPath)
	p := filepath.Join(subcBaseDir, corpName)
	files, err := os.ReadDir(p)
	if err != nil {
		return ans, fmt.Errorf("failed to open multisampled corpus: %w", err)
	}
	for _, item := range files {
		suff := filepath.Ext(item.Name())
		if suff == ".subc" {
			ans.Subcorpora = append(ans.Subcorpora, filepath.Join(p, item.Name()))
		}
	}
	return ans, nil
}

type QueryHandler interface {
	PublishQuery(query rdb.Query) (<-chan *rdb.WorkerResult, error)
}
