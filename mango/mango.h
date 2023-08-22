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

#ifdef __cplusplus
extern "C" {
#endif

typedef void* PosAttrV;
typedef void* CorpusV;
typedef void* StructV;
typedef void* ConcV;
typedef void* MVector;
typedef void* KWICRowsV;
typedef void* CollsV;


typedef long long int PosInt;

/**
 * CorpusRetval wraps both
 * a returned Manatee corpus object
 * and possible error
 */
typedef struct CorpusRetval {
    CorpusV value;
    const char * err;
} CorpusRetval;


typedef struct CorpusSizeRetrval {
    PosInt value;
    const char * err;
} CorpusSizeRetrval;


typedef struct CorpusStringRetval {
    const char * value;
    const char * err;
} CorpusStringRetval;

typedef struct ConcRetval {
    ConcV value;
    const char * err;
} ConcRetval;

typedef struct ConcSizeRetVal {
    PosInt value;
    PosInt corpusSize;
    const char * err;
} ConcSizeRetVal;

typedef struct FreqsRetval {
    MVector words;
    MVector freqs;
    MVector norms;
    PosInt concSize;
    PosInt corpusSize;
    const char * err;
} FreqsRetval;


typedef struct CollItem {
    double score;
    double freq;
    char *word;
} CollItem;

typedef struct CollsRetVal {
    CollsV items;
    PosInt resultSize;
    PosInt concSize;
    PosInt corpusSize;
    const char * err;
} CollsRetVal;

typedef struct KWICRowsRetval {
    KWICRowsV value;
    PosInt size;
    const char * err;
} KWICRowsRetval;

/**
 * Create a Manatee corpus instance
 */
CorpusRetval open_corpus(const char* corpusPath);

void close_corpus(CorpusV corpus);

CorpusSizeRetrval get_corpus_size(CorpusV corpus);

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop);

ConcSizeRetVal concordance_size(const char* corpusPath, const char* query);

void delete_str_vector(MVector v);

void delete_int_vector(MVector v);

const char* str_vector_get_element(MVector v, int i);

PosInt str_vector_get_size(MVector v);

PosInt int_vector_get_element(MVector v, int i);

PosInt int_vector_get_size(MVector v);

FreqsRetval freq_dist_from_conc(CorpusV corpus, ConcV conc, char* fcrit, PosInt flimit);

FreqsRetval freq_dist(const char* corpusPath, const char* query, const char* fcrit, PosInt flimit);

KWICRowsRetval conc_examples(const char* corpusPath, const char*query, const char* attrs, PosInt limit);

void conc_examples_free(KWICRowsV value, int numItems);

CollsRetVal collocations(
    const char* corpusPath,
    const char* query,
    const char * attrName,
    char collFn,
    char sortFunCode,
    PosInt minfreq,
    PosInt minbgr,
    int fromw,
    int tow,
    int maxitems
);

CollItem get_coll_item(CollsRetVal data, int idx);

void coll_examples_free(CollsV items, int numItems);

typedef struct CollVal {
    const char* word;
    double value;
    PosInt freq;
    const char * err;
} CollVal;


#ifdef __cplusplus
}
#endif