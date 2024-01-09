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
	"encoding/json"
	"mquery/corpus"
	"mquery/corpus/baseinfo"
	"mquery/rdb"
)

type Manatee struct {
	conf         *corpus.CorporaSetup
	queryHandler corpus.QueryHandler
}

func (kdb *Manatee) LoadCorpusInfo(corpusId string, language string) (*baseinfo.Corpus, error) {
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

func NewManatee(
	queryHandler corpus.QueryHandler,
	conf *corpus.CorporaSetup,
) *Manatee {
	return &Manatee{
		queryHandler: queryHandler,
		conf:         conf,
	}
}
