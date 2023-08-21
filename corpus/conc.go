// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
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
	"mquery/mango"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

var (
	splitPatt = regexp.MustCompile(`\s+`)
)

type Token struct {
	Word   string            `json:"word"`
	Strong bool              `json:"strong"`
	Attrs  map[string]string `json:"attrs"`
}

type ConcordanceLine struct {
	Text []*Token `json:"text"`
}

type ConcExamples struct {
	Lines []ConcordanceLine `json:"lines"`
}

type LineParser struct {
	attrs []string
}

func (lp *LineParser) parseTokenQuadruple(s []string) *Token {
	mAttrs := make(map[string]string)
	for i, attr := range strings.Split(s[2], "/")[1:] {
		mAttrs[lp.attrs[i]] = attr
	}
	return &Token{
		Word:   s[0],
		Strong: len(s[1]) > 2,
		Attrs:  mAttrs,
	}
}

func (lp *LineParser) normalizeTokens(tokens []string) []string {
	ans := make([]string, 0, len(tokens))
	var parTok strings.Builder
	for _, tok := range tokens {
		if tok == "" {
			continue

		} else if tok[0] == '{' {
			if tok[len(tok)-1] != '}' {
				parTok.WriteString(tok)

			} else {
				ans = append(ans, tok)
			}

		} else if tok[len(tok)-1] == '}' {
			parTok.WriteString(tok)
			ans = append(ans, parTok.String())
			parTok.Reset()

		} else {
			ans = append(ans, tok)
		}
	}
	return ans
}

func (lp *LineParser) parseRawLine(line string) ConcordanceLine {
	items := lp.normalizeTokens(splitPatt.Split(line, -1))
	if len(items)%4 != 0 {
		log.Error().
			Str("origLine", line).
			Msg("unparseable Manatee KWIC line")
		return ConcordanceLine{Text: []*Token{{Word: "---- ERROR (unparseable) ----"}}}
	}
	tokens := make([]*Token, 0, len(items)/4)
	for i := 0; i < len(items); i += 4 {
		tokens = append(tokens, lp.parseTokenQuadruple(items[i:i+4]))
	}
	return ConcordanceLine{Text: tokens}
}

func (lp *LineParser) Parse(data mango.GoConcExamples) *ConcExamples {
	pLines := make([]ConcordanceLine, len(data.Lines))
	for i, line := range data.Lines {
		pLines[i] = lp.parseRawLine(line)
	}
	return &ConcExamples{
		Lines: pLines,
	}
}

func NewLineParser(attrs []string) *LineParser {
	return &LineParser{attrs: attrs}
}
