// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infoload

import (
	"fmt"
	"mquery/corpus"
	"mquery/rdb"
	"mquery/rdb/results"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/mquery-common/corp"
)

type Manatee struct {
	conf         *corpus.CorporaSetup
	queryHandler corpus.QueryHandler
	cache        map[string]*results.CorpusInfo
}

func mergeConfigInfo(conf *corpus.MQCorpusSetup, cinfo *results.CorpusInfo, lang string) {
	newAttrList := make([]corp.Attr, len(cinfo.Data.AttrList))
	for i, attr := range cinfo.Data.AttrList {
		srch := conf.GetPosAttr(attr.Name)
		if !srch.IsZero() {
			attr.Description = srch.LocaleDescription(lang)
		}
		newAttrList[i] = attr
	}
	cinfo.Data.AttrList = newAttrList
	desc := conf.LocaleDescription(lang)
	if desc != "" {
		cinfo.Data.Description = desc
	}
	cinfo.Data.SrchKeywords = conf.SrchKeywords
	if cinfo.Data.SrchKeywords == nil {
		cinfo.Data.SrchKeywords = []string{}
	}
}

func (kdb *Manatee) makeCacheKey(corpusId string, language string) string {
	return fmt.Sprintf("%s#%s", corpusId, language)
}

func (kdb *Manatee) LoadCorpusInfo(corpusId string, language string) (*results.CorpusInfo, error) {
	val, ok := kdb.cache[kdb.makeCacheKey(corpusId, language)]
	if ok {
		return val, nil
	}

	corpusPath := kdb.conf.GetRegistryPath(corpusId)
	registryExists, err := fs.IsFile(corpusPath)
	if err != nil {
		return nil, err
	}
	if !registryExists {
		return nil, corpus.ErrNotFound
	}
	wait, err := kdb.queryHandler.PublishQuery(rdb.Query{
		Func: "corpusInfo",
		Args: rdb.CorpusInfoArgs{
			CorpusPath: corpusPath,
			Language:   language,
		},
	})
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	corpusInfo, ok := rawResult.Value.(results.CorpusInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected type for CorpusInfo")
	}
	if corpusInfo.Err() != nil {
		return nil, corpusInfo.Err()
	}
	mergeConfigInfo(kdb.conf.Resources.Get(corpusId), &corpusInfo, language)
	kdb.cache[kdb.makeCacheKey(corpusId, language)] = &corpusInfo
	return &corpusInfo, nil
}

func NewManatee(
	queryHandler corpus.QueryHandler,
	conf *corpus.CorporaSetup,
) *Manatee {
	return &Manatee{
		queryHandler: queryHandler,
		conf:         conf,
		cache:        make(map[string]*results.CorpusInfo),
	}
}
