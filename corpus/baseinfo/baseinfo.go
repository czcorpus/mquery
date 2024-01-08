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

package baseinfo

import (
	"encoding/json"
	"mquery/corpus"
	"mquery/engine"
	"mquery/mango"
	"mquery/rdb"
	"strings"
)

type ManateeCorpusInfo struct {
	conf         *corpus.CorporaSetup
	queryHandler corpus.QueryHandler
}

func (kdb *ManateeCorpusInfo) LoadCorpusInfo(corpusId string, language string) (*engine.CorpusInfo, error) {
	args, err := json.Marshal(rdb.CorpusInfoArgs{
		CorpusPath: kdb.conf.GetRegistryPath(corpusId),
		Language:   language,
	})
	if err != nil {
		return nil, err
	}
	wait, err := kdb.queryHandler.PublishQuery(rdb.Query{
		Func: "corpusInfo",
		Args: args,
	})
	if err != nil {
		return nil, err
	}
	rawResult := <-wait
	corpusInfo, err := rdb.DeserializeCorpusInfoDataResult(rawResult)
	if err != nil {
		return nil, err
	}
	if corpusInfo.Err() != nil {
		return nil, corpusInfo.Err()
	}
	return &corpusInfo.Data, nil
}

func NewManateeCorpusInfo(
	queryHandler corpus.QueryHandler,
	conf *corpus.CorporaSetup,
) *ManateeCorpusInfo {
	return &ManateeCorpusInfo{
		queryHandler: queryHandler,
		conf:         conf,
	}
}

func FillStructAndAttrsInfo(corpPath string, info *engine.CorpusInfo) error {
	attrs, err := mango.GetCorpusConf(corpPath, "ATTRLIST")
	if err != nil {
		return err
	}
	for _, v := range strings.Split(attrs, ",") {
		size, err := mango.GetPosAttrSize(corpPath, v)
		if err != nil {
			return err
		}
		info.AttrList = append(info.AttrList, engine.Item{
			Name: v,
			Size: size,
		})
	}
	structs, err := mango.GetCorpusConf(corpPath, "STRUCTLIST")
	if err != nil {
		return err
	}
	for _, v := range strings.Split(structs, ",") {
		size, err := mango.GetStructSize(corpPath, v)
		if err != nil {
			return err
		}
		info.StructList = append(info.StructList, engine.Item{
			Name: v,
			Size: size,
		})
	}
	return nil
}
