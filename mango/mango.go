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

package mango

// #cgo LDFLAGS:  -lmanatee -L${SRCDIR} -Wl,-rpath='$ORIGIN'
// #include <stdlib.h>
// #include "mango.h"
import "C"

import (
	"fmt"
	"strings"
	"unicode"
	"unsafe"

	"github.com/czcorpus/cnc-gokit/maths"
)

type GoVector struct {
	v C.MVector
}

type Freqs struct {
	Words      []string
	Freqs      []int64
	Norms      []int64
	ConcSize   int64
	CorpusSize int64
	SearchSize int64
}

// ---

type GoConcSize struct {
	Value      int64
	CorpusSize int64
}

type GoConcordance struct {
	Lines    []string
	ConcSize int
}

type GoCollItem struct {
	Word     string
	Score    float64
	ScoreLCI float64
	ScoreRCI float64
	Stdev    float64
	Freq     int64
}

type GoColls struct {
	Colls      []*GoCollItem
	ConcSize   int64
	CorpusSize int64
}

func GetCorpusSize(corpusPath string) (int64, error) {
	ans := C.get_corpus_size(C.CString(corpusPath))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return 0, err
	}
	return int64(ans.value), nil
}

func GetConcSize(corpusPath, query string) (GoConcSize, error) {
	ans := C.concordance_size(C.CString(corpusPath), C.CString(query))
	var ret GoConcSize
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return ret, err
	}
	ret.CorpusSize = int64(ans.corpusSize)
	ret.Value = int64(ans.value)
	return ret, nil
}

func CompileSubcFreqs(corpusPath, subcPath, attr string) error {
	ans := C.compile_subc_freqs(C.CString(corpusPath), C.CString(subcPath), C.CString(attr))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return err
	}

	return nil
}

func GetConcordance(corpusPath, query string, attrs []string, maxItems int) (GoConcordance, error) {
	ans := C.conc_examples(
		C.CString(corpusPath), C.CString(query), C.CString(strings.Join(attrs, ",")), C.longlong(maxItems))
	var ret GoConcordance
	ret.Lines = make([]string, 0, maxItems)
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return ret, err

	} else {
		defer C.conc_examples_free(ans.value, C.int(ans.size))
	}
	tmp := (*[1000]*C.char)(unsafe.Pointer(ans.value))
	for i := 0; i < int(ans.size); i++ {
		ret.Lines = append(ret.Lines, C.GoString(tmp[i]))
	}
	return ret, nil
}

func CalcFreqDist(corpusID, subcID, query, fcrit string, flimit int) (*Freqs, error) {
	var ret Freqs
	ans := C.freq_dist(C.CString(corpusID), C.CString(subcID), C.CString(query), C.CString(fcrit), C.longlong(flimit))
	defer func() { // the 'new' was called before any possible error so we have to do this
		C.delete_int_vector(ans.freqs)
		C.delete_int_vector(ans.norms)
		C.delete_str_vector(ans.words)
	}()
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return &ret, err
	}
	ret.Freqs = IntVectorToSlice(GoVector{ans.freqs})
	ret.Norms = IntVectorToSlice(GoVector{ans.norms})
	ret.Words = StrVectorToSlice(GoVector{ans.words})
	ret.ConcSize = int64(ans.concSize)
	ret.CorpusSize = int64(ans.corpusSize)
	ret.SearchSize = int64(ans.searchSize)
	return &ret, nil
}

func normalizeMultiword(w string) string {
	return strings.TrimSpace(strings.Map(func(c rune) rune {
		if unicode.IsSpace(c) {
			return ' '
		}
		return c
	}, w))
}

func StrVectorToSlice(vector GoVector) []string {
	size := int(C.str_vector_get_size(vector.v))
	slice := make([]string, size)
	for i := 0; i < size; i++ {
		cstr := C.str_vector_get_element(vector.v, C.int(i))
		slice[i] = normalizeMultiword(C.GoString(cstr))
	}
	return slice
}

func IntVectorToSlice(vector GoVector) []int64 {
	size := int(C.int_vector_get_size(vector.v))
	slice := make([]int64, size)
	for i := 0; i < size; i++ {
		v := C.int_vector_get_element(vector.v, C.int(i))
		slice[i] = int64(v)
	}
	return slice
}

// GetCollcations
//
// 't': 'T-score',
// 'm': 'MI',
// '3': 'MI3',
// 'l': 'log likelihood',
// 's': 'min. sensitivity',
// 'p': 'MI.log_f',
// 'r': 'relative freq. [%]',
// 'f': 'absolute freq.',
// 'd': 'logDice'
func GetCollcations(
	corpusID, subcID, query string,
	attrName string,
	calcFn byte,
	minFreq int64,
	maxItems int,
) (GoColls, error) {
	colls := C.collocations(
		C.CString(corpusID), C.CString(subcID), C.CString(query), C.CString(attrName), C.char(calcFn), C.char(calcFn),
		C.longlong(minFreq), C.longlong(minFreq), -5, 5, C.int(maxItems))
	if colls.err != nil {
		err := fmt.Errorf(C.GoString(colls.err))
		defer C.free(unsafe.Pointer(colls.err))
		return GoColls{}, err
	}
	items := make([]*GoCollItem, colls.resultSize)
	for i := 0; i < int(colls.resultSize); i++ {
		tmp := C.get_coll_item(colls, C.int(i))
		items[i] = &GoCollItem{
			Word:  C.GoString(tmp.word),
			Score: maths.RoundToN(float64(tmp.score), 4),
			Freq:  int64(tmp.freq),
		}
	}
	//C.coll_examples_free(colls.items, colls.numItems)
	return GoColls{
		Colls:      items,
		ConcSize:   int64(colls.concSize),
		CorpusSize: int64(colls.corpusSize),
	}, nil
}

func GetTextTypesNorms(corpusPath string, attr string) (map[string]int64, error) {
	ans := make(map[string]int64)
	attrSplit := strings.Split(attr, ".")
	if len(attrSplit) != 2 {
		panic("invalid attribute format (must be `struct.attr`)")
	}
	norms := C.get_attr_values_sizes(
		C.CString(corpusPath), C.CString(attrSplit[0]), C.CString(attrSplit[1]))
	if norms.err != nil {
		err := fmt.Errorf(C.GoString(norms.err))
		defer C.free(unsafe.Pointer(norms.err))
		return ans, err
	}
	defer C.delete_attr_values_sizes(norms.sizes)

	iter := C.get_attr_val_iterator(norms.sizes)
	defer C.delete_attr_val_iterator(iter)
	for {
		val := C.get_next_attr_val_size(norms.sizes, iter)
		if val.value == nil {
			break
		}
		ans[C.GoString(val.value)] = int64(val.freq)
	}

	return ans, nil
}

// GetCorpusConf returns a corpus configuration item
// stored in a corpus configuration file (aka "registry file")
func GetCorpusConf(corpusPath string, prop string) (string, error) {
	ans := (C.get_corpus_conf(C.open_corpus(C.CString(corpusPath)).value, C.CString(prop)))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return "", err
	}
	return C.GoString(ans.value), nil
}

func GetPosAttrSize(corpusPath string, name string) (int, error) {
	ans := C.get_posattr_size(C.CString(corpusPath), C.CString(name))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return 0, err
	}
	return int(ans.value), nil
}

func GetStructSize(corpusPath string, name string) (int, error) {
	ans := C.get_struct_size(C.CString(corpusPath), C.CString(name))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return 0, err
	}
	return int(ans.value), nil
}
