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
typedef void* AttrValMap;
typedef void* AttrValMapIterator;


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
    double arf;
    PosInt corpusSize;
    const char * err;
} ConcSizeRetVal;

typedef struct CompileFrqRetVal {
    const char * err;
} CompileFrqRetVal;

typedef struct FreqsRetval {
    MVector words;
    MVector freqs;
    MVector norms;
    PosInt concSize;
    PosInt corpusSize;
    PosInt searchSize;
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
    PosInt searchSize;
    const char * err;
} CollsRetVal;

typedef struct KWICRowsRetval {
    KWICRowsV value;
    PosInt size;
    PosInt concSize;
    const char * err;
    int errorCode;
} KWICRowsRetval;

typedef struct CorpRegionRetval {
    const char* err;
    const char* text;
} CorpRegionRetval;


typedef struct AttrValSizes {
    const char * err;
    AttrValMap sizes;
} AttrValSizes;

/**
 * Create a Manatee corpus instance
 */
CorpusRetval open_corpus(const char* corpusPath);

void close_corpus(CorpusV corpus);

CorpusSizeRetrval get_corpus_size(const char* corpusPath);

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop);

ConcSizeRetVal concordance_size(const char* corpusPath, const char* query);

CompileFrqRetVal compile_subc_freqs(const char* corpusPath, const char* subcPath, const char* attr);

void delete_str_vector(MVector v);

void delete_int_vector(MVector v);

const char* str_vector_get_element(MVector v, int i);

PosInt str_vector_get_size(MVector v);

PosInt int_vector_get_element(MVector v, int i);

PosInt int_vector_get_size(MVector v);

FreqsRetval freq_dist_from_conc(CorpusV corpus, ConcV conc, char* fcrit, PosInt flimit);

FreqsRetval freq_dist(const char* corpusPath, const char* subcPath, const char* query, const char* fcrit, PosInt flimit);

/**
 * @brief Based on provided query, return at most `limit` sentences matching the query.
 * The returned string is always in form "[kwic_token_id] [rest...]" - so to parse the
 * data properly, the ID must be cut from the rest of the data.
 * Please note that when called from Go via function `GetConcExamples`, the Go function
 * checks the `limit` argument against `mango.MaxRecordsInternalLimit` and will not allow
 * larger value.
 *
 * @param corpusPath
 * @param query
 * @param attrs Positional attributes (comma-separated) to be attached to returned tokens
 * @param limit
 * @return KWICRowsRetval
 */
KWICRowsRetval conc_examples(
    const char* corpusPath,
    const char*query,
    const char* attrs,
    const char* structs,
    const char* refs,
    const char* refsSplitter,
    PosInt fromLine,
    PosInt limit,
    PosInt maxContext,
    const char* viewContextStruct);

void conc_examples_free(KWICRowsV value, int numItems);


KWICRowsRetval conc_examples_with_coll_phrase(
    const char* corpusPath,
    const char* query,
    const char* collQuery,
    const char* lctx,
    const char* rctx,
    const char* attrs,
    const char* structs,
    const char* refs,
    const char* refsSplitter,
    PosInt fromLine,
    PosInt limit,
    PosInt maxContext,
    const char* viewContextStruct);

CorpRegionRetval get_corp_region(
    const char* corpusPath,
    PosInt position,
    PosInt numTok,
    const char* attrs,
    const char* structs
);

CollsRetVal collocations(
    const char* corpusPath,
    const char* subcPath,
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


AttrValSizes get_attr_values_sizes(
    const char* corpus_path,
    const char* struct_name,
    const char* attr_name
);


void delete_attr_values_sizes(AttrValMap sizes);


typedef struct AttrVal {
    const char* value;
    PosInt freq;
} AttrVal;

AttrValMapIterator get_attr_val_iterator(AttrValMap srcMap);

void delete_attr_val_iterator(AttrValMapIterator itr);

AttrVal get_next_attr_val_size(AttrValMap srcMap, AttrValMapIterator itr);

CorpusSizeRetrval get_posattr_size(const char* corpus_path, const char* name);

CorpusSizeRetrval get_struct_size(const char* corpus_path, const char* name);

void free_string(char* str);

#ifdef __cplusplus
}
#endif