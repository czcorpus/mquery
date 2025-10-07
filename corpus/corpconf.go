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

package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/rs/zerolog/log"
)

// Single corpus configuration types
// ----------------------------------------

type MQCorpusSetup struct {
	corp.CorpusSetup
}

func (cs *MQCorpusSetup) ValidateAndDefaults() error {
	if cs.CorpusSetup.IsDynamic() {
		for _, variant := range cs.CorpusSetup.Variants {
			if len(variant.FullName) == 0 || variant.FullName["en"] == "" {
				return fmt.Errorf("missing corpus variant `fullName`, at least `en` value must be set")
			}
		}

	} else {
		if len(cs.CorpusSetup.FullName) == 0 || cs.CorpusSetup.FullName["en"] == "" {
			return fmt.Errorf("missing corpus `fullName`, at least `en` value must be set")
		}
	}
	if len(cs.CorpusSetup.PosAttrs) == 0 {
		return fmt.Errorf("at least one positional attribute in `posAttrs` must be defined")
	}
	if cs.CorpusSetup.MaximumRecords == 0 {
		cs.CorpusSetup.MaximumRecords = DfltMaximumRecords
		log.Warn().
			Int("value", cs.CorpusSetup.MaximumRecords).
			Msg("missing or zero `maximumRecords`, using default")
	}
	if len(cs.CorpusSetup.TextProperties.ListOverviewProps()) == 0 {
		log.Warn().
			Msg("no `ttOverviewAttrs` defined, some freq. function will be disabled")
	}
	for prop := range cs.CorpusSetup.TextProperties {
		if !prop.Validate() {
			return fmt.Errorf("invalid text property %s", prop)
		}
	}
	if cs.CorpusSetup.MaximumTokenContextWindow == 0 {
		log.Warn().
			Int("value", DfltMaximumTokenContextWindow).
			Msg("`maximumTokenContextWindow` not specified, using default")
		cs.CorpusSetup.MaximumTokenContextWindow = DfltMaximumTokenContextWindow
	}
	return nil
}

// Multiple corpora configuration types
// -------------------------------------

type Resources []*MQCorpusSetup

func (rscs *Resources) Load(directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("failed to load corpora configs: %w", err)
	}
	for _, f := range files {
		confPath := filepath.Join(directory, f.Name())
		tmp, err := os.ReadFile(confPath)
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", confPath).
				Msg("encountered invalid corpus configuration file, skipping")
			continue
		}
		var conf MQCorpusSetup
		err = sonic.Unmarshal(tmp, &conf)
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", confPath).
				Msg("encountered invalid corpus configuration file, skipping")
			continue
		}
		*rscs = append(*rscs, &conf)
		log.Info().Str("name", conf.ID).Msg("loaded corpus configuration file")
	}
	return nil
}

func (rscs Resources) Get(name string) *MQCorpusSetup {
	for _, v := range rscs {
		if strings.Contains(v.ID, "*") {
			ptrn := regexp.MustCompile(strings.ReplaceAll(v.ID, "*", ".*"))
			if ptrn.MatchString(name) {
				fmt.Printf(">>>>>>>>>> %s >>> %#v\n", name, v)
				if v.Variants != nil {
					variant, ok := v.Variants[name]
					if ok {
						// make a copy of CorpusSetup and replace values for specific variant
						merged := *v
						merged.Variants = nil
						merged.ID = variant.ID
						if len(variant.FullName) > 0 {
							merged.FullName = variant.FullName
						}
						if len(variant.Description) > 0 {
							merged.Description = variant.Description
						}
						return &merged
					}
				}
			}

		} else if v.ID == name {
			return v
		}
	}
	return nil
}

func (rscs Resources) GetAllCorpora() []*MQCorpusSetup {
	ans := make([]*MQCorpusSetup, 0, len(rscs)*3)
	for _, v := range rscs {
		if len(v.Variants) > 0 {
			for _, variant := range v.Variants {
				item := rscs.Get(variant.ID)
				ans = append(ans, item)
			}

		} else {
			ans = append(ans, v)
		}
	}
	return ans
}
