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
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/czcorpus/mquery/rdb"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

var (
	ErrNotFound     = errors.New("corpus not found or available")
	ErrAccessDenied = errors.New("access denied")
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
	if len(subcID) < 2 || strings.Contains(subcID, "..") || strings.ContainsRune(subcID, filepath.Separator) {
		return subcID, false
	}
	path := filepath.Join(baseDir, subcID[:2], subcID, "data.subc")
	isf, err := fs.IsFile(path)
	if err != nil {
		log.Error().Err(err).Msg("failed to check saved subcorpus path")
		return path, false
	}
	return path, isf
}

// subcRangeSize is the size of a single range in a Manatee subcorpus
// file (a `map64` ranges file: pairs of little-endian int64 [beg, end) positions)
const subcRangeSize = 16

// SavedSubcInfo contains basic properties of a saved subcorpus calculated
// from its ranges file
type SavedSubcInfo struct {
	// Size is the number of positions (tokens) in the subcorpus
	Size int64
	// NumRanges is the number of position ranges the subcorpus consists of
	NumRanges int64
	// MaxPos is the end (exclusive) of the last range. For a subcorpus
	// created from a corpus, it must not exceed the corpus size.
	MaxPos int64
}

// ReadSavedSubcorpusInfo reads a Manatee subcorpus file (`data.subc`)
// and calculates its size the same way Manatee does (sum of range sizes).
// Please note that the file contains no reference to the corpus
// it was created from.
func ReadSavedSubcorpusInfo(path string) (SavedSubcInfo, error) {
	var ans SavedSubcInfo
	f, err := os.Open(path)
	if err != nil {
		return ans, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ans, err
	}
	if st.Size()%subcRangeSize != 0 {
		return ans, fmt.Errorf("invalid subcorpus file size %d", st.Size())
	}
	rd := bufio.NewReaderSize(f, 64*1024)
	var item [subcRangeSize]byte
	for {
		if _, err := io.ReadFull(rd, item[:]); err == io.EOF {
			break

		} else if err != nil {
			return ans, err
		}
		beg := int64(binary.LittleEndian.Uint64(item[:8]))
		end := int64(binary.LittleEndian.Uint64(item[8:]))
		if beg < 0 || end < beg {
			return ans, fmt.Errorf("invalid subcorpus range [%d, %d)", beg, end)
		}
		ans.Size += end - beg
		ans.NumRanges++
		ans.MaxPos = max(ans.MaxPos, end)
	}
	return ans, nil
}

// SubcFreqDataPath returns a path of Manatee's auxiliary frequency file
// for a positional attribute `attr` of a subcorpus `subcPath` (e.g. for
// `/a/b/data.subc` and `lemma` it returns `/a/b/data.lemma.frq`).
// Note: Manatee may also create a 64-bit variant with the `64` suffix
// (e.g. `data.lemma.frq64`) in case some frequency exceeds uint32.
func SubcFreqDataPath(subcPath, attr string) string {
	// this mimics Manatee's SubCorpus SUBCPATH handling
	prefix := strings.TrimSuffix(subcPath, filepath.Ext(subcPath))
	return fmt.Sprintf("%s.%s.frq", prefix, attr)
}

// SubcFreqDataExists tests whether auxiliary frequency data for
// a positional attribute `attr` of a subcorpus `subcPath` exist.
func SubcFreqDataExists(subcPath, attr string) (bool, error) {
	path := SubcFreqDataPath(subcPath, attr)
	for _, p := range []string{path, path + "64"} {
		isf, err := fs.IsFile(p)
		if err != nil {
			return false, fmt.Errorf("failed to test subc. freq. data file %s: %w", p, err)
		}
		if isf {
			return true, nil
		}
	}
	return false, nil
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
