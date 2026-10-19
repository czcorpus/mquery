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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/czcorpus/mquery-common/corp"
	"github.com/czcorpus/rexplorer/parser"
	"github.com/rs/zerolog/log"
)

// Single corpus configuration types
// ----------------------------------------

type MQCorpusSetup struct {
	corp.CorpusSetup
	IsDisabled                bool `json:"isDisabled"`
	InternalNetworkAccessOnly bool `json:"internalNetworkAccessOnly"`
	fullConcTextPropsAttrs    []string
	allPosAttrs               corp.PosAttrList
	syncedWithRegistry        string
}

// SyncWithRegistry loads values which are available only in corpus
// registry file (all positional attributes, all structural attributes).
// For positional attributes defined also in the JSON configuration,
// the JSON-defined descriptions take precedence.
// The function is expected to be called during configuration loading
// as it is not thread-safe.
func (cs *MQCorpusSetup) SyncWithRegistry(registryDir string) {
	if cs.syncedWithRegistry == registryDir {
		return
	}
	tmp := AutogenerateMinConf(registryDir, cs.ID)
	if tmp == nil {
		return
	}
	cs.fullConcTextPropsAttrs = tmp.fullConcTextPropsAttrs
	cs.allPosAttrs = tmp.PosAttrs
	for i, posattr := range cs.allPosAttrs {
		if jsonAttr := cs.CorpusSetup.GetPosAttr(posattr.Name); len(jsonAttr.Description) > 0 {
			cs.allPosAttrs[i].Description = jsonAttr.Description
		}
	}
	for _, posattr := range cs.PosAttrs {
		if !cs.allPosAttrs.Contains(posattr.Name) {
			log.Warn().
				Str("corpus", cs.ID).
				Str("attr", posattr.Name).
				Msg("configured positional attribute not found in corpus registry")
		}
	}
	cs.syncedWithRegistry = registryDir
}

func (cs *MQCorpusSetup) FullConcTextPropsAttrs() []string {
	return cs.fullConcTextPropsAttrs
}

// AllPosAttrs returns all the positional attributes defined
// in corpus registry. This is in contrast to `PosAttrs` which
// contains only attributes explicitly configured in MQuery
// (which are used e.g. as default attributes for concordances).
func (cs *MQCorpusSetup) AllPosAttrs() corp.PosAttrList {
	return cs.allPosAttrs
}

// GetPosAttr searches for a positional attribute among all
// the registry-defined attributes (see AllPosAttrs). In case
// the configuration has not been synced with the registry,
// only the JSON-configured attributes are searched.
// If nothing is found, a zero PosAttr is returned.
//
// Note: this shadows corp.CorpusSetup.GetPosAttr which searches
// only the JSON-configured attributes.
func (cs *MQCorpusSetup) GetPosAttr(name string) corp.PosAttr {
	if cs.allPosAttrs == nil {
		return cs.CorpusSetup.GetPosAttr(name)
	}
	for _, v := range cs.allPosAttrs {
		if v.Name == name {
			return v
		}
	}
	return corp.PosAttr{}
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

func checkRegistryFile(regPath string) error {
	regBytes, err := os.ReadFile(regPath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}
	reg, err := parser.ParseRegistryBytes(filepath.Base(regPath), regBytes)
	if err != nil {
		return fmt.Errorf("failed to parse registry file: %w", err)
	}
	dataPath := reg.Entries.Get("PATH")
	dpInfo, err := os.Stat(dataPath.Value())
	if err != nil {
		return fmt.Errorf("failed to validate corpus data path: %w", err)
	}
	if !dpInfo.IsDir() {
		return fmt.Errorf("corpus data path is not a directory")
	}
	return nil
}

type Resources []*MQCorpusSetup

func (rscs *Resources) Load(directory, registryDir string) error {
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
		err = json.Unmarshal(tmp, &conf)
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", confPath).
				Msg("encountered invalid corpus configuration file, skipping")
			continue
		}

		if conf.IsDisabled {
			log.Warn().Str("corpus", conf.ID).Msg("skipping disabled corpus")
			continue
		}

		regCheckCorpora := make([]string, 0, 10)
		if strings.Contains(conf.ID, "*") && len(conf.Variants) > 0 {
			for corp := range conf.Variants {
				regCheckCorpora = append(regCheckCorpora, corp)
			}

		} else {
			regCheckCorpora = append(regCheckCorpora, conf.ID)
		}
		for _, corpusID := range regCheckCorpora {
			if err := checkRegistryFile(filepath.Join(registryDir, corpusID)); err != nil {
				log.Error().Err(err).Str("corpus", corpusID)
				return err
			}

		}
		*rscs = append(*rscs, &conf)
		log.Info().Str("name", conf.ID).Msg("loaded corpus configuration file")
	}
	return nil
}

// expandVariants replaces dynamic corpora configurations (templates
// with "*" in their ID) with actual configurations of their variants.
// Each variant is a copy of its template (sharing its read-only
// slices and maps) and it is synced with its registry file.
// The function is expected to be called during configuration loading
// as it is not thread-safe. Repeated calls have no effect as no templates
// remain after the first call.
func (rscs *Resources) expandVariants(registryPath string) {
	expanded := make(Resources, 0, len(*rscs))
	for _, tpl := range *rscs {
		if !tpl.IsDynamic() {
			expanded = append(expanded, tpl)
			continue
		}
		for _, variant := range tpl.Variants {
			merged := *tpl
			merged.syncedWithRegistry = ""
			merged.Variants = nil
			merged.ID = variant.ID
			if len(variant.FullName) > 0 {
				merged.FullName = variant.FullName
			}
			if len(variant.Description) > 0 {
				merged.Description = variant.Description
			}
			merged.Subcorpora = nil
			merged.Tagsets = nil
			merged.SrchKeywords = nil
			merged.SyncWithRegistry(registryPath)
			expanded = append(expanded, &merged)
		}
	}
	*rscs = expanded
}

func (rscs Resources) get(name string) *MQCorpusSetup {
	for _, v := range rscs {
		if v.ID == name {
			return v
		}
	}
	return nil
}
