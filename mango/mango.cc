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


#include "corp/corpus.hh"
#include "corp/subcorp.hh"
#include "concord/concord.hh"
#include "concord/concstat.hh"
#include "concord/concget.hh"
#include "query/cqpeval.hh"
#include "mango.h"
#include <string.h>
#include <stdio.h>
#include <iostream>
#include <memory>
#include <sstream>
#include <map>

using namespace std;


CorpusRetval open_corpus(const char* corpusPath) {
    string tmp(corpusPath);
    CorpusRetval ans;
    ans.err = nullptr;
    ans.value = nullptr;
    try {
        ans.value = new Corpus(tmp);

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    return ans;
}

void close_corpus(CorpusV corpus) {
    delete (Corpus *)corpus;
}

CorpusSizeRetrval get_corpus_size(const char* corpusPath) {
    CorpusSizeRetrval ans;
    ans.err = nullptr;
    Corpus* corp = nullptr;
    try {
        Corpus* corp = new Corpus(corpusPath);
        ans.value = corp->size();
        delete corp;

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    return ans;
}

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop) {
    CorpusStringRetval ans;
    ans.err = nullptr;
    ans.value = nullptr;
    string tmp(prop);
    try {
        const char * s = ((Corpus*)corpus)->get_conf(tmp).c_str();
        ans.value = s;
        return ans;

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
        return ans;
    }
}

ConcSizeRetVal concordance_size(const char* corpusPath, const char* query) {
    string cPath(corpusPath);
    ConcSizeRetVal ans;
    ans.err = nullptr;
    ans.value = 0;
    Corpus* corp = nullptr;
    Concordance* conc = nullptr;
    try {
        corp = new Corpus(cPath);
        ans.corpusSize = corp->size();
        conc = new Concordance(
            corp, corp->filter_query(eval_cqpquery(query, corp)));
        conc->sync();
        ans.value = conc->size();

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }

    delete conc;
    delete corp;

    return ans;
}

CompileFrqRetVal compile_subc_freqs(const char* corpusPath, const char* subcPath, const char* attr) {
    CompileFrqRetVal ans;
    ans.err = nullptr;
    string cPath(corpusPath);
    Corpus* corp = nullptr;
    SubCorpus* subc = nullptr;

    try {
        corp = new Corpus(cPath);
        subc = new SubCorpus(corp, subcPath);
        subc->compile_frq(attr);


    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }

    delete subc;
    delete corp;

    return ans;
}

FreqsRetval freq_dist_from_conc(CorpusV corpus, ConcV conc, char* fcrit, PosInt flimit) {
    Corpus* corpusObj = (Corpus*)corpus;
    Concordance* concObj = (Concordance *)conc;

    auto xwords = new vector<string>;
    vector<string>& words = *xwords;
    auto xfreqs = new vector<PosInt>;
    vector<PosInt>& freqs = *xfreqs;
    auto xnorms = new vector<PosInt>;
    vector<PosInt>& norms = *xnorms;

    corpusObj->freq_dist (concObj->RS(), fcrit, flimit, words, freqs, norms);
    FreqsRetval ans {
        static_cast<void*>(xwords),
        static_cast<void*>(xfreqs),
        static_cast<void*>(xnorms),
        0, // TODO
        0, // TODO
        0, // TODO
        nullptr
    };
    return ans;
}


FreqsRetval freq_dist(const char* corpusPath, const char* subcPath, const char* query, const char* fcrit, PosInt flimit) {
    string cPath(corpusPath);
    try {
        Corpus* corp = new Corpus(cPath);
        Concordance* conc = nullptr;
        SubCorpus* subc = nullptr;
        auto xwords = new vector<string>;
        vector<string>& words = *xwords;
        auto xfreqs = new vector<PosInt>;
        vector<PosInt>& freqs = *xfreqs;
        auto xnorms = new vector<PosInt>;
        vector<PosInt>& norms = *xnorms;
        PosInt concSize;
        PosInt corpSize;
        PosInt searchSize;

        if (subcPath && *subcPath != '\0') {
            subc = new SubCorpus(corp, subcPath);
            conc = new Concordance(subc, subc->filter_query(eval_cqpquery(query, subc)));
            conc->sync();
            subc->freq_dist(conc->RS(), fcrit, flimit, words, freqs, norms);
            concSize = conc->size();
            corpSize = corp->size();
            searchSize = subc->search_size();

        } else {
            conc = new Concordance(corp, corp->filter_query(eval_cqpquery(query, corp)));
            conc->sync();
            corp->freq_dist(conc->RS(), fcrit, flimit, words, freqs, norms);
            concSize = conc->size();
            corpSize = corp->size();
            searchSize = corp->size();
        }
        FreqsRetval ans {
            static_cast<void*>(xwords),
            static_cast<void*>(xfreqs),
            static_cast<void*>(xnorms),
            concSize,
            corpSize,
            searchSize,
            nullptr
        };
        delete conc;
        delete subc;
        delete corp;
        return ans;

    } catch (std::exception &e) {
        FreqsRetval ans {
            nullptr,
            nullptr,
            nullptr,
            0,
            0,
            0,
            strdup(e.what())
        };
        return ans;
    }
}

/**
 * @brief Based on provided query, return at most `limit` sentences matching the query.
 *
 * @param corpusPath
 * @param query
 * @param attrs Positional attributes (comma-separated) to be attached to returned tokens
 * @param limit
 * @return KWICRowsRetval
 */
KWICRowsRetval conc_examples(
    const char* corpusPath, const char* query, const char* attrs, PosInt fromLine, PosInt limit,
        PosInt maxContext, const char* viewContextStruct) {

    string cPath(corpusPath);
    try {
        Corpus* corp = new Corpus(cPath);
        Concordance* conc = new Concordance(
            corp, corp->filter_query(eval_cqpquery(query, corp)));
        conc->sync();
        if (conc->size() == 0 && fromLine == 0) {
            KWICRowsRetval ans {
                nullptr,
                0,
                0,
                nullptr
            };
            return ans;
        }
        if (conc->size() < fromLine) {
            const char* msg = "line range out of result size";
            char* dynamicStr = static_cast<char*>(malloc(strlen(msg) + 1));
            strcpy(dynamicStr, msg);
            KWICRowsRetval ans {
                nullptr,
                0,
                0,
                dynamicStr,
                1
            };
            return ans;
        }
        conc->shuffle();
        PosInt concSize = conc->size();
        KWICLines* kl = new KWICLines(
            corp,
            conc->RS(true, fromLine, fromLine+limit),
            ("-1:"+std::string(viewContextStruct)).c_str(),
            ("1:"+std::string(viewContextStruct)).c_str(),
            attrs,
            attrs,
            "",
            "#",
            maxContext,
            false
        );
        if (conc->size() < limit) {
            limit = conc->size();
        }
        char** lines = (char**)malloc(limit * sizeof(char*));
        int i = 0;
        while (kl->nextline()) {
            auto lft = kl->get_left();
            auto kwc = kl->get_kwic();
            auto rgt = kl->get_right();
            std::ostringstream buffer;

            buffer << kl->get_refs() << " ";

            for (size_t i = 0; i < lft.size(); ++i) {
                if (i > 0) {
                    buffer << " ";
                }
                buffer << lft.at(i);
            }
            for (size_t i = 0; i < kwc.size(); ++i) {
                if (i > 0) {
                    buffer << " ";
                }
                buffer << kwc.at(i);
            }
            for (size_t i = 0; i < rgt.size(); ++i) {
                if (i > 0) {
                    buffer << " ";
                }
                buffer << rgt.at(i);
            }
            lines[i] = strdup(buffer.str().c_str());
            i++;
            if (i == limit) {
                break;
            }
        }
        // We've allocated memory for `limit` rows,
        // but it's possible that there is less rows
        // available so here we fill the remaining items
        // with empty strings.
        for (int i2 = i; i2 < limit; i2++) {
            lines[i2] = strdup("");
        }
        delete conc;
        delete corp;
        KWICRowsRetval ans {
            lines,
            limit,
            concSize,
            nullptr,
            0
        };
        return ans;

    } catch (std::exception &e) {
        KWICRowsRetval ans {
            nullptr,
            0,
            0,
            strdup(e.what()),
            0
        };
        return ans;
    }
}

/**
 * @brief This function frees all the allocated memory
 * for a concordance example. It is intended to be called
 * from Go.
 *
 * @param value
 * @param numItems
 */
void conc_examples_free(KWICRowsV value, int numItems) {
    char** tValue = (char**)value;
    for (int i = 0; i < numItems; i++) {
        free(tValue[i]);
    }
    free(tValue);
}


void delete_str_vector(MVector v) {
    vector<string>* vectorObj = (vector<string>*)v;
    delete vectorObj;
}

void delete_int_vector(MVector v) {
    vector<PosInt>* vectorObj = (vector<PosInt>*)v;
    delete vectorObj;
}

const char* str_vector_get_element(MVector v, int i) {
    vector<string>* vectorObj = (vector<string>*)v;
    return vectorObj->at(i).c_str();
}

PosInt str_vector_get_size(MVector v) {
    vector<string>* vectorObj = (vector<string>*)v;
    return vectorObj->size();
}

PosInt int_vector_get_element(MVector v, int i) {
    vector<PosInt>* vectorObj = (vector<PosInt>*)v;
    return vectorObj->at(i);
}

PosInt int_vector_get_size(MVector v) {
    vector<PosInt>* vectorObj = (vector<PosInt>*)v;
    return vectorObj->size();
}

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
) {
    CollsRetVal ans;
    ans.err = nullptr;
    string cPath(corpusPath);
    Corpus* corp = nullptr;
    Concordance* conc = nullptr;
    CollocItems* collocs = nullptr;
    SubCorpus* subc = nullptr;

    try {
        corp = new Corpus(cPath);

        if (subcPath && *subcPath != '\0') {
            subc = new SubCorpus(corp, subcPath);
            conc = new Concordance(subc, subc->filter_query(eval_cqpquery(query, subc)));

        } else {
            conc = new Concordance(corp, corp->filter_query(eval_cqpquery(query, corp)));
        }
        ans.corpusSize = corp->size();
        conc->sync();
        ans.concSize = conc->size();
        ans.searchSize = corp->size();
        ans.resultSize = 0;
        collocs = new CollocItems(conc, string(attrName), sortFunCode, minfreq, minbgr, fromw, tow, maxitems);
        CollItem* items = (CollItem*) malloc(maxitems * sizeof(CollItem));
        int i = 0;
        while (collocs->eos() == false && i < maxitems) {
            CollItem item;
            item.score = collocs->get_bgr(collFn);
            item.freq = collocs->get_cnt();
            item.word = strdup(collocs->get_item());
            items[i] = item;
            ans.resultSize++;
            i++;
            collocs->next();
        }
        ans.items = items;
    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    delete collocs;
    delete conc;
    delete subc;
    delete corp;
    return ans;
}


CollItem get_coll_item(CollsRetVal data, int idx) {
    return ((CollItem*)data.items)[idx];
}

void coll_examples_free(CollsV items, int numItems) {
    CollItem* tItems = (CollItem*)items;
    for (int i = 0; i < numItems; i++) {
        free(tItems[i].word);
    }
    free(tItems);
}


AttrValSizes get_attr_values_sizes(
    const char* corpus_path,
    const char* struct_name,
    const char* attr_name
) {
    AttrValSizes ans;
    ans.err = nullptr;
    ans.sizes = nullptr;
    Corpus* corp = nullptr;
    Structure* strct = nullptr;
    PosAttr* attr = nullptr;

    try {
        corp = new Corpus(corpus_path);
        strct = corp->get_struct(struct_name);
        attr = strct->get_attr(attr_name);

        map<PosInt, PosInt> normvals;
        auto sizes = new map<string, PosInt>;

        for (PosInt i = 0; i < strct->size(); i++) {
            normvals[strct->rng->beg_at(i)] = strct->rng->end_at(i) - strct->rng->beg_at(i);
        }

        for (PosInt i = 0; i < attr->id_range(); i++) {
            const char* value = attr->id2str(i);
            PosInt valid = attr->str2id(value);
            RangeStream* rng = corp->filter_query(strct->rng->part(attr->id2poss(valid)));
            PosInt cnt = 0;
            while (!rng->end()) {
                cnt += normvals[rng->peek_beg()];
                rng->next();
            }
            (*sizes)[value] = cnt;
        }
        ans.sizes = static_cast<void*>(sizes);

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    delete corp;

    return ans;
}


void delete_attr_values_sizes(AttrValMap sizes) {
    auto tSizes = (map<string, PosInt>*)sizes;
    delete tSizes;
}


AttrValMapIterator get_attr_val_iterator(AttrValMap srcMap) {
    map<string, PosInt>* sizes = (map<string, PosInt>*)srcMap;
    auto* itr = new std::map<string, PosInt>::iterator(sizes->begin());
    return static_cast<void*>(itr);
}


void delete_attr_val_iterator(AttrValMapIterator itr) {
    auto tItr = (std::map<string, PosInt>::iterator*)itr;
    delete tItr;
}


AttrVal get_next_attr_val_size(AttrValMap srcMap, AttrValMapIterator itr) {
    map<string, PosInt>::iterator* tItr = (map<string, PosInt>::iterator*)itr;
    map<string, PosInt>* srcMapObj = (map<string, PosInt>*)srcMap;
    AttrVal ans;
    ans.value = nullptr;
    if (*tItr == srcMapObj->end()) {
        return ans;
    }
    ans.freq = (*tItr)->second;
    ans.value = (*tItr)->first.c_str();
    ++(*tItr);
    return ans;
}

CorpusSizeRetrval get_posattr_size(const char* corpus_path, const char* name) {
    CorpusSizeRetrval ans;
    ans.err = nullptr;
    Corpus* corp = nullptr;
    try {
        corp = new Corpus(corpus_path);
        ans.value = corp->get_attr(name, false)->id_range();
    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    delete corp;
    return ans;
}

CorpusSizeRetrval get_struct_size(const char* corpus_path, const char* name) {
    CorpusSizeRetrval ans;
    ans.err = nullptr;
    Corpus* corp = nullptr;
    try {
        corp = new Corpus(corpus_path);
        ans.value = corp->get_struct(name)->size();
    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    delete corp;
    return ans;
}
