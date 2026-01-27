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

package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/czcorpus/mquery-common/corp"
	"github.com/czcorpus/rexplorer/parser"
	"github.com/rs/zerolog/log"
)

var (
	keyPosattrs   = []string{"word", "sword", "lemma", "sublemma", "tag"}
	keyUDPosattrs = []string{"word", "sword", "lemma", "upos", "feats", "deprel", "parent"}
	markupStructs = []string{
		"p", "s", "hi", "lb", "note", "table", "ref", "email", "graphic", "geo", "g", "head", "bold",
		"em", "emph", "strong", "br", "quote", "seg",
	}
	textSegmentStructs = []string{
		"doc", "text", "file", "sp", "speech",
	}
)

func isLikelyUDCorpus(posattrs []*parser.Attr) bool {
	numMatches := 0
	for _, attr := range posattrs {
		if attr.Name == "upos" {
			numMatches++

		} else if attr.Name == "feats" {
			numMatches++
		}
		if numMatches >= 2 {
			return true
		}
	}
	return false
}

func getSelAttrSet(reg *parser.Document) []string {
	if isLikelyUDCorpus(reg.PosAttrs) {
		return keyUDPosattrs
	}
	return keyPosattrs
}

func extractPosAttrs(reg *parser.Document, selAttrs []string) []corp.PosAttr {
	ans := make([]corp.PosAttr, 0, len(reg.PosAttrs))
	for _, p := range reg.PosAttrs {
		if slices.Contains(selAttrs, p.Name) {
			ans = append(
				ans,
				corp.PosAttr{
					Name:        p.Name,
					Description: map[string]string{"en": p.Entries.Get("LABEL").Value()},
				},
			)
		}
	}
	return ans
}

func extractTextPropsStrucattrs(reg *parser.Document) corp.TextTypeProperties {
	commonProps := make(corp.TextTypeProperties)
	for _, strct := range reg.Structures {
		if slices.Contains(textSegmentStructs, strct.Name) {
			for _, attr := range strct.Attrs {
				var key string
				if attr.Name == "author" {
					key = corp.TextPropertyAuthor

				} else if attr.Name == "year" || attr.Name == "pubyear" {
					key = corp.TextPropertyPubYear

				} else if attr.Name == "title" {
					key = corp.TextPropertyTitle

				} else if attr.Name == "srclang" {
					key = corp.TextPropertyOriginaLang

				} else if attr.Name == "txtype" {
					key = corp.TextPropertyTextType

				} else if attr.Name == "medium" || attr.Name == "media_type" {
					key = corp.TextPropertyMedium
				}

				if key != "" {
					commonProps[corp.TextProperty(key)] = corp.TTPropertyConf{
						Name:         fmt.Sprintf("%s.%s", strct.Name, attr.Name),
						IsInOverview: true,
					}
				}
			}
		}
	}
	return commonProps
}

func extractMarkupStructs(reg *parser.Document) []string {
	ans := make([]string, 0, 20)
	for _, strct := range reg.Structures {
		if slices.Contains(markupStructs, strct.Name) {
			ans = append(ans, strct.Name)
		}
	}
	return ans
}

// AutogenerateConf generates a MQuery configuration based
// on a registry file found based on the provided arguments.
func AutogenerateConf(registryDir, corpusID string) *MQCorpusSetup {
	regPath := filepath.Join(registryDir, corpusID)
	tmp, err := os.ReadFile(regPath)
	if err != nil {
		log.Error().Err(err).Str("corpus", corpusID).Msg("failed to autogenerate corpus config")
		return nil
	}
	reg, err := parser.ParseRegistryBytes(corpusID, tmp)
	if err != nil {
		log.Error().Err(err).Str("corpus", corpusID).Msg("failed to autogenerate corpus config")
		return nil
	}
	if err := checkRegistryFile(regPath); err != nil {
		log.Error().Err(err).Str("corpus", corpusID).Msg("failed to validate corpus autoconfiguration; corpus won't be available")
		return nil
	}
	newConf := &MQCorpusSetup{
		CorpusSetup: corp.CorpusSetup{
			ID:                   corpusID,
			FullName:             map[string]string{"en": reg.Entries.Get("NAME").Value()},
			PosAttrs:             extractPosAttrs(reg, getSelAttrSet(reg)),
			ConcMarkupStructures: extractMarkupStructs(reg),
			TextProperties:       extractTextPropsStrucattrs(reg),
		},
	}
	if err := newConf.ValidateAndDefaults(); err != nil {
		log.Error().Err(err).Str("corpus", corpusID).Msg("failed to validate corpus autoconfiguration; corpus won't be available")
		return nil
	}
	return newConf
}
