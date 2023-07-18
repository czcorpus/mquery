// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

#ifdef __cplusplus
extern "C" {
#endif

typedef void* PosAttrV;
typedef void* CorpusV;
typedef void* StructV;
typedef void* ConcV;
typedef void* MVector;
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

typedef struct FreqsRetval {
    MVector words;
    MVector freqs;
    MVector norms;
    const char * err;
} FreqsRetval;

typedef struct CollsRetVal {
    CollsV value;
    const char * err;
} CollsRetVal;

/**
 * Create a Manatee corpus instance
 */
CorpusRetval open_corpus(const char* corpusPath);

void close_corpus(CorpusV corpus);

CorpusSizeRetrval get_corpus_size(CorpusV corpus);

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop);

ConcRetval create_concordance(CorpusV corpus, char* query);

PosInt concordance_size(ConcV conc);

void delete_str_vector(MVector v);

void delete_int_vector(MVector v);

const char* str_vector_get_element(MVector v, int i);

PosInt str_vector_get_size(MVector v);

PosInt int_vector_get_element(MVector v, int i);

PosInt int_vector_get_size(MVector v);

FreqsRetval freq_dist(CorpusV corpus, ConcV conc, char* fcrit, PosInt flimit);

CollsRetVal collocations(ConcV conc, const char * attr_name, char sort_fun_code,
             PosInt minfreq, PosInt minbgr, int fromw, int tow, int maxitems);

typedef struct CollVal {
    const char* word;
    double value;
    PosInt freq;
    const char * err;
} CollVal;


CollVal next_colloc_item(CollsV colls, char collFn);

int has_next_colloc(CollsV colls);


#ifdef __cplusplus
}
#endif