// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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

package handlers

import (
	"encoding/json"
	"mquery/corpus"
	"testing"

	"github.com/czcorpus/mquery-common/corp"
	"github.com/stretchr/testify/assert"
)

func TestNewCorpusBriefInfo(t *testing.T) {
	conf := &corpus.MQCorpusSetup{
		CorpusSetup: corp.CorpusSetup{
			ID:           "syn2020",
			FullName:     map[string]string{"en": "SYN2020", "cs": "SYN2020 (cs)"},
			Description:  map[string]string{"en": "a long description"},
			SrchKeywords: []string{"czech", "written"},
			Subcorpora: map[string]corp.Subcorpus{
				"fiction": {}, "academic": {},
			},
		},
	}
	info := newCorpusBriefInfo(conf, "cs")
	assert.Equal(t, corpusBriefInfo{
		ID:         "syn2020",
		FullName:   "SYN2020 (cs)",
		Flags:      []string{"czech", "written"},
		Subcorpora: []string{"academic", "fiction"},
	}, info)

	// fallback to English
	assert.Equal(t, "SYN2020", newCorpusBriefInfo(conf, "de").FullName)
}

func TestCorpusBriefInfoOmitsEmpty(t *testing.T) {
	conf := &corpus.MQCorpusSetup{
		CorpusSetup: corp.CorpusSetup{
			ID:       "bnc",
			FullName: map[string]string{"en": "British National Corpus"},
		},
	}
	data, err := json.Marshal(newCorpusBriefInfo(conf, "en"))
	assert.NoError(t, err)
	assert.JSONEq(t, `{"id":"bnc","fullName":"British National Corpus"}`, string(data))
}
