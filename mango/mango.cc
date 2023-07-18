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


#include "corp/corpus.hh"
#include "concord/concord.hh"
#include "concord/concstat.hh"
#include "query/cqpeval.hh"
#include "mango.h"
#include <string.h>
#include <stdio.h>
#include <iostream>
#include <memory>

using namespace std;

// a bunch of wrapper functions we need to get data
// from Manatee


CorpusRetval open_corpus(const char* corpusPath) {
    string tmp(corpusPath);
    CorpusRetval ans;
    ans.err = nullptr;
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

CorpusSizeRetrval get_corpus_size(CorpusV corpus) {
    CorpusSizeRetrval ans;
    ans.err = nullptr;
    try {
        ans.value = ((Corpus*)corpus)->size();

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    return ans;
}

CorpusStringRetval get_corpus_conf(CorpusV corpus, const char* prop) {
    CorpusStringRetval ans;
    ans.err = nullptr;
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


ConcRetval create_concordance(CorpusV corpus, char* query) {
    string q(query);
    ConcRetval ans;
    ans.err = nullptr;
    Corpus* corpusObj = (Corpus*)corpus;

    try {
        ans.value = new Concordance(
            corpusObj, corpusObj->filter_query(eval_cqpquery(q.c_str(), (Corpus*)corpus)));
        ((Concordance*)ans.value)->sync();
    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    return ans;
}

PosInt concordance_size(ConcV conc) {
    return ((Concordance *)conc)->size();
}

FreqsRetval freq_dist(CorpusV corpus, ConcV conc, char* fcrit, PosInt flimit) {
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
        static_cast<void*>(xnorms)
    };
    return ans;
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

CollsRetVal collocations(ConcV conc, const char * attr_name, char sort_fun_code,
             PosInt minfreq, PosInt minbgr, int fromw, int tow, int maxitems) {
    CollsRetVal ans;
    ans.err = nullptr;
    Concordance* concObj = (Concordance*)conc;

    try {
        ans.value = new CollocItems(concObj, string(attr_name), sort_fun_code, minfreq, minbgr, fromw, tow, maxitems);
    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    return ans;
}


CollVal next_colloc_item(CollsV colls, char collFn) {
    CollVal ans;
    ans.err = nullptr;
    CollocItems* collsObj = (CollocItems*)colls;
    try {
        string word = string(collsObj->get_item());
        double value = collsObj->get_bgr(collFn);
        ans.value = value;
        ans.word = word.c_str();
        ans.freq = collsObj->get_cnt();
        collsObj->next();

    } catch (std::exception &e) {
        ans.err = strdup(e.what());
    }
    return ans;
}

int has_next_colloc(CollsV colls) {
    CollVal ans;
    ans.err = nullptr;
    CollocItems* collsObj = (CollocItems*)colls;
    return collsObj->eos() == true ? 0 : 1;
}