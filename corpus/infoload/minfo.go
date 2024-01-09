// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
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
	"mquery/corpus"
	"mquery/corpus/baseinfo"
	"mquery/mango"
	"strings"
)

func FillStructAndAttrs(corpPath string, info *baseinfo.Corpus) error {
	attrs, err := mango.GetCorpusConf(corpPath, "ATTRLIST")
	if err != nil {
		return err
	}
	for _, v := range strings.Split(attrs, ",") {
		size, err := mango.GetPosAttrSize(corpPath, v)
		if err != nil {
			return err
		}
		info.AttrList = append(info.AttrList, baseinfo.Item{
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
		info.StructList = append(info.StructList, baseinfo.Item{
			Name: v,
			Size: size,
		})
	}
	return nil
}

// AttributeFiller is a helper object providing access
// to corpus structural and positional attribute information
// for
type AttributeFiller struct {
	conf *corpus.CorporaSetup
}

func (mp *AttributeFiller) FillStructAndAttrs(corpPath string, info *baseinfo.Corpus) error {
	return FillStructAndAttrs(corpPath, info)
}

func (mp *AttributeFiller) GetRegistryPath(corpusID string) string {
	return mp.conf.GetRegistryPath(corpusID)
}

func NewAttributeFiller(conf *corpus.CorporaSetup) *AttributeFiller {
	return &AttributeFiller{conf: conf}
}
