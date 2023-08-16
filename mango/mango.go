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
)

// GoCorpus is a Go wrapper for Manatee Corpus instance
type GoCorpus struct {
	corp C.CorpusV
}

func (gc *GoCorpus) Close() {
	C.close_corpus(gc.corp)
}

type GoVector struct {
	v C.MVector
}

type Freqs struct {
	Words      []string
	Freqs      []int64
	Norms      []int64
	ConcSize   int64
	CorpusSize int64
}

type GoConc struct {
	conc     C.ConcV
	corpSize int64
	corpus   *GoCorpus
}

func (gc *GoConc) CorpSize() int64 {
	return gc.corpSize
}

func (gc *GoConc) Corpus() *GoCorpus {
	return gc.corpus
}

// ---

type GoConcSize struct {
	Value      int64
	CorpusSize int64
}

type GoConcExamples struct {
	Lines    []string
	ConcSize int
}

type GoCollsItem struct {
	Word  string
	Value float64
	Freq  int64
}

type GoColls struct {
	Colls      []GoCollsItem
	ConcSize   int64
	CorpusSize int64
}

// OpenCorpus is a factory function creating
// a Manatee corpus wrapper.
func OpenCorpus(path string) (*GoCorpus, error) {
	ret := &GoCorpus{}
	var err error
	ans := C.open_corpus(C.CString(path))

	if ans.err != nil {
		err = fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return ret, err
	}
	ret.corp = ans.value
	if ret.corp == nil {
		return ret, fmt.Errorf("Corpus %s not found", path)
	}
	return ret, nil
}

// CloseCorpus closes all the resources accompanying
// the corpus. The instance should become unusable.
func CloseCorpus(corpus *GoCorpus) error {
	C.close_corpus(corpus.corp)
	return nil
}

// GetCorpusSize returns corpus size in tokens
func GetCorpusSize(corpus *GoCorpus) (int64, error) {
	ans := (C.get_corpus_size(corpus.corp))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return -1, err
	}
	return int64(ans.value), nil
}

// GetCorpusConf returns a corpus configuration item
// stored in a corpus configuration file (aka "registry file")
func GetCorpusConf(corpus *GoCorpus, prop string) (string, error) {
	ans := (C.get_corpus_conf(corpus.corp, C.CString(prop)))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return "", err
	}
	return C.GoString(ans.value), nil
}

func CreateConcordance(corpus *GoCorpus, query string) (*GoConc, error) {
	var ret GoConc

	ans := C.create_concordance(corpus.corp, C.CString(query))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return nil, err
	}
	ret.conc = ans.value

	corpSize, err := GetCorpusSize(corpus)
	if err != nil {
		return nil, err
	}
	ret.corpSize = corpSize
	ret.corpus = corpus
	return &ret, nil
}

func CloseConcordance(conc *GoConc) {
	C.close_concordance(conc.conc)
}

func CalcFreqDistFromConc(conc *GoConc, fcrit string, flimit int) (*Freqs, error) {
	var ret Freqs
	ans := C.freq_dist_from_conc(conc.Corpus().corp, conc.conc, C.CString(fcrit), C.longlong(flimit))
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
	return &ret, nil
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

func GetConcExamples(corpusPath, query string, attrs []string, maxItems int) (GoConcExamples, error) {
	ans := C.conc_examples(
		C.CString(corpusPath), C.CString(query), C.CString(strings.Join(attrs, ",")), C.longlong(maxItems))
	var ret GoConcExamples
	ret.Lines = make([]string, maxItems)
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return ret, err

	} else {
		defer C.conc_examples_free(ans.value, C.int(ans.size))
	}
	tmp := (*[1000]*C.char)(unsafe.Pointer(ans.value))
	for i := 0; i < int(ans.size); i++ {
		ret.Lines[i] = C.GoString(tmp[i])
	}
	return ret, nil
}

func CalcFreqDist(corpusID, query, fcrit string, flimit int) (*Freqs, error) {
	var ret Freqs
	ans := C.freq_dist(C.CString(corpusID), C.CString(query), C.CString(fcrit), C.longlong(flimit))
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
	corpusID, query string,
	attrName string,
	calcFn byte,
	minFreq int64,
	maxItems int,
) (GoColls, error) {
	colls := C.collocations(C.CString(corpusID), C.CString(query), C.CString(attrName), C.char(calcFn),
		C.longlong(minFreq), C.longlong(minFreq), -5, 5, C.int(maxItems))
	if colls.err != nil {
		err := fmt.Errorf(C.GoString(colls.err))
		defer C.free(unsafe.Pointer(colls.err))
		return GoColls{}, err
	}
	items := make([]GoCollsItem, 0, 50) // TODO capacity
	for C.has_next_colloc(colls.value) == 1 {
		ans := C.next_colloc_item(colls.value, C.char(calcFn))
		if ans.err != nil {
			err := fmt.Errorf(C.GoString(ans.err))
			defer C.free(unsafe.Pointer(ans.err))
			return GoColls{}, err
		}
		items = append(
			items,
			GoCollsItem{
				Word:  C.GoString(ans.word),
				Value: float64(ans.value),
				Freq:  int64(ans.freq),
			},
		)
	}
	return GoColls{
		Colls:      items,
		ConcSize:   int64(colls.concSize),
		CorpusSize: int64(colls.corpusSize),
	}, nil
}
