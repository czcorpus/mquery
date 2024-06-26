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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

const (
	DfltSplitChunkSize = 100000000
	DfltMaximumRecords = 50
)

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

type CorpusSetup struct {
	ID                string               `json:"id"`
	FullName          map[string]string    `json:"fullName"`
	Description       map[string]string    `json:"description"`
	SyntaxConcordance SyntaxConcordance    `json:"syntaxConcordance"`
	PosAttrs          PosAttrList          `json:"posAttrs"`
	StructAttrs       []StructAttr         `json:"structAttrs"`
	MaximumRecords    int                  `json:"maximumRecords"`
	Subcorpora        map[string]Subcorpus `json:"subcorpora"`
	// ViewContextStruct is a structure used to specify "units"
	// for KWIC left and right context. Typically, this is
	// a structure representing a sentence or a speach.
	ViewContextStruct string                   `json:"viewContextStruct"`
	Variants          map[string]CorpusVariant `json:"variants"`
	SrchKeywords      []string                 `json:"srchKeywords"`
	WebURL            string                   `json:"webUrl"`
	TextProperties    TextTypeProperties       `json:"textProperties"`
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

func (cs *CorpusSetup) GetStruct(name string) StructAttr {
	for _, v := range cs.StructAttrs {
		if v.Name == name {
			return v
		}
	}
	return StructAttr{}
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
	return nil
}

type Resources []*CorpusSetup

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

// CorporaSetup defines mquery application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDir     string `json:"registryDir"`
	SplitCorporaDir string `json:"splitCorporaDir"`

	// MultiprocChunkSize defines a subcorpus size for large
	// corpora when processed in a parallel way.
	// Please note that once created, the subcorpora will be
	// applied in their original way until explicitly removed.
	// I.e. the value only affects newly created splits.
	MultiprocChunkSize int64 `json:"multiprocChunkSize"`

	MktokencovPath string `json:"mktokencovPath"`

	Resources Resources `json:"resources"`
}

func (cs *CorporaSetup) GetRegistryPath(corpusID string) string {
	return filepath.Join(cs.RegistryDir, corpusID)
}

func (cs *CorporaSetup) ValidateAndDefaults(confContext string) error {
	if cs == nil {
		return fmt.Errorf("missing configuration section `%s`", confContext)
	}
	if cs.RegistryDir == "" {
		return fmt.Errorf("missing `%s.registryDir`", confContext)
	}
	isDir, err := fs.IsDir(cs.RegistryDir)
	if err != nil {
		return fmt.Errorf("failed to test `%s.registryDir`: %w", confContext, err)
	}
	if !isDir {
		return fmt.Errorf("`%s.registryDir` is not a directory", confContext)
	}
	if cs.SplitCorporaDir == "" {
		return fmt.Errorf("missing `%s.splitCorporaDir`", confContext)
	}
	isDir, err = fs.IsDir(cs.SplitCorporaDir)
	if err != nil {
		return fmt.Errorf("failed to test `%s.splitCorporaDir`: %w", confContext, err)
	}
	if !isDir {
		return fmt.Errorf("`%s.splitCorporaDir` is not a directory", confContext)
	}

	if cs.MultiprocChunkSize == 0 {
		log.Warn().
			Int("value", DfltSplitChunkSize).
			Msgf("`%s.multiprocChunkSize` not set, using default", confContext)
	}

	isFile, err := fs.IsFile(cs.MktokencovPath)
	if err != nil {
		return fmt.Errorf("failed to test `%s.mktokencovPath` file %w", confContext, err)
	}
	if !isFile {
		return fmt.Errorf("the `%s.mktokencovPath` does not point to a file", confContext)
	}
	for _, v := range cs.Resources {
		if err := v.ValidateAndDefaults(); err != nil {
			return err
		}
	}
	return nil
}
