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
	"mquery/corpus/baseinfo"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/rs/zerolog/log"
)

// Single corpus configuration types
// ----------------------------------------

type PosAttr struct {
	Name        string            `json:"name"`
	Description map[string]string `json:"description"`
}

func (p PosAttr) IsZero() bool {
	return p.Name == ""
}

func (p PosAttr) LocaleDescription(lang string) string {
	d := p.Description[lang]
	if d != "" {
		return d
	}
	return p.Description["en"]
}

type PosAttrList []PosAttr

func (pal PosAttrList) GetIDs() []string {
	ans := make([]string, len(pal))
	for i, v := range pal {
		ans[i] = v.Name
	}
	return ans
}

func (pal PosAttrList) Contains(ident string) bool {
	for _, v := range pal {
		if v.Name == ident {
			return true
		}
	}
	return false
}

// ----

type StructAttr struct {
	Name        string            `json:"name"`
	Description map[string]string `json:"description"`
}

func (s StructAttr) LocaleDescription(lang string) string {
	d := s.Description[lang]
	if d != "" {
		return d
	}
	return s.Description["en"]
}

func (s StructAttr) IsZero() bool {
	return s.Name == ""
}

// ----

type SyntaxConcordance struct {
	ParentAttr string `json:"parentAttr"`

	// ResultAttrs is a list of positional attributes
	// we need to provide all the required information about
	// syntax in for the "syntax-conc-examples" endpoint
	ResultAttrs []string `json:"resultAttrs"`
}

type TextTypes map[string][]string

type Subcorpus struct {
	ID          string            `json:"id"`
	TextTypes   TextTypes         `json:"textTypes"`
	Description map[string]string `json:"description"`
}

type CorpusVariant struct {
	ID          string            `json:"id"`
	FullName    map[string]string `json:"fullName"`
	Description map[string]string `json:"description"`
}

type TTPropertyConf struct {
	Name         string `json:"name"`
	IsInOverview bool   `json:"isInOverview"`
}

// TextTypeProperties maps between generalized text properties
// and specific corpus structural attributes.
type TextTypeProperties map[baseinfo.TextProperty]TTPropertyConf

// Prop returns a generalized property based on provided struct. attribute
// If nothing is found, empty TextProperty is returned
func (ttp TextTypeProperties) Prop(attr string) baseinfo.TextProperty {
	for k, v := range ttp {
		if v.Name == attr {
			return k
		}
	}
	return ""
}

func (ttp TextTypeProperties) List() []baseinfo.TextProperty {
	ans := make([]baseinfo.TextProperty, len(ttp))
	var i int
	for k := range ttp {
		ans[i] = k
	}
	return ans
}

func (ttp TextTypeProperties) ListOverviewProps() []baseinfo.TextProperty {
	ans := make([]baseinfo.TextProperty, 0, len(ttp))
	for _, v := range ttp {
		if v.IsInOverview {
			ans = append(ans, baseinfo.TextProperty(v.Name))
		}
	}
	return ans
}

// Attr returns a struct. attribute name based on generalized property.
// If nothing is found, empty string is returned.
func (ttp TextTypeProperties) Attr(prop baseinfo.TextProperty) string {
	return ttp[prop].Name
}

type ContextWindow int

// LeftAndRight converts the window into the left-right intervals.
// In case the value is an odd number, the reminder is added to the right.
func (cw ContextWindow) LeftAndRight() (lft int, rgt int) {
	tmp := int(cw) / 2
	lft = tmp
	rgt = tmp + (int(cw) - 2*tmp)
	return
}

type CorpusSetup struct {
	ID                   string             `json:"id"`
	FullName             map[string]string  `json:"fullName"`
	Description          map[string]string  `json:"description"`
	SyntaxConcordance    SyntaxConcordance  `json:"syntaxConcordance"`
	PosAttrs             PosAttrList        `json:"posAttrs"`
	ConcMarkupStructures []string           `json:"concMarkupStructures"`
	ConcTextPropsAttrs   []string           `json:"concTextPropsAttrs"`
	TextProperties       TextTypeProperties `json:"textProperties"`
	MaximumRecords       int                `json:"maximumRecords"`

	// MaximumTokenContextWindow specifies the total width of token's context
	// with the token in the middle. Odd numbers are applied in a way giving one
	// more token to the right.
	MaximumTokenContextWindow ContextWindow `json:"MaximumTokenContextWindow"`

	// Subcorpora defines named transient subcorpora created as part of the query.
	// MQuery also supports so called saved subcorpora which are files created via Manatee-open
	// (or in a more user-friendly way using KonText or NoSkE).
	Subcorpora map[string]Subcorpus `json:"subcorpora"`
	// ViewContextStruct is a structure used to specify "units"
	// for KWIC left and right context. Typically, this is
	// a structure representing a sentence or a speach.
	ViewContextStruct string                   `json:"viewContextStruct"`
	Variants          map[string]CorpusVariant `json:"variants"`
	SrchKeywords      []string                 `json:"srchKeywords"`
	WebURL            string                   `json:"webUrl"`
}

func (cs *CorpusSetup) LocaleDescription(lang string) string {
	d := cs.Description[lang]
	if d != "" {
		return d
	}
	return cs.Description["en"]
}

func (cs *CorpusSetup) IsDynamic() bool {
	return strings.Contains(cs.ID, "*")
}

func (cs *CorpusSetup) GetPosAttr(name string) PosAttr {
	for _, v := range cs.PosAttrs {
		if v.Name == name {
			return v
		}
	}
	return PosAttr{}
}

func (cs *CorpusSetup) KnownStructures() []string {
	ans := make([]string, 0, len(cs.ConcMarkupStructures)+len(cs.ConcTextPropsAttrs))
	ans = append(ans, cs.ConcMarkupStructures...)
	ans = append(ans, cs.ConcTextPropsAttrs...)
	return ans
}

func (cs *CorpusSetup) ValidateAndDefaults() error {
	if cs.IsDynamic() {
		for _, variant := range cs.Variants {
			if len(variant.FullName) == 0 || variant.FullName["en"] == "" {
				return fmt.Errorf("missing corpus variant `fullName`, at least `en` value must be set")
			}
		}

	} else {
		if len(cs.FullName) == 0 || cs.FullName["en"] == "" {
			return fmt.Errorf("missing corpus `fullName`, at least `en` value must be set")
		}
	}
	if len(cs.PosAttrs) == 0 {
		return fmt.Errorf("at least one positional attribute in `posAttrs` must be defined")
	}
	if cs.MaximumRecords == 0 {
		cs.MaximumRecords = DfltMaximumRecords
		log.Warn().
			Int("value", cs.MaximumRecords).
			Msg("missing or zero `maximumRecords`, using default")
	}
	if len(cs.TextProperties.ListOverviewProps()) == 0 {
		log.Warn().
			Msg("no `ttOverviewAttrs` defined, some freq. function will be disabled")
	}
	for prop := range cs.TextProperties {
		if !prop.Validate() {
			return fmt.Errorf("invalid text property %s", prop)
		}
	}
	if cs.MaximumTokenContextWindow == 0 {
		log.Warn().
			Int("value", DfltMaximumTokenContextWindow).
			Msg("`maximumTokenContextWindow` not specified, using default")
		cs.MaximumTokenContextWindow = DfltMaximumTokenContextWindow
	}
	return nil
}

// Multiple corpora configuration types
// -------------------------------------

type Resources []*CorpusSetup

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
		var conf CorpusSetup
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

func (rscs Resources) Get(name string) *CorpusSetup {
	for _, v := range rscs {
		if strings.Contains(v.ID, "*") {
			ptrn := regexp.MustCompile(strings.ReplaceAll(v.ID, "*", ".*"))
			if ptrn.MatchString(name) {
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
				return v
			}

		} else if v.ID == name {
			return v
		}
	}
	return nil
}

func (rscs Resources) GetAllCorpora() []*CorpusSetup {
	ans := make([]*CorpusSetup, 0, len(rscs)*3)
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
