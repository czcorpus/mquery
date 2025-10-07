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

package transform

import (
	"fmt"
	"mquery/corpus"
	"mquery/rdb/results"
	"strings"

	"github.com/czcorpus/mquery-common/concordance"
)

func expandKWIC(tk *concordance.Token, conf *corpus.MQCorpusSetup) string {
	var tmp strings.Builder
	if tk.Strong {
		tmp.WriteString(fmt.Sprintf("**%s** *{", tk.Word))
		var i int
		for _, v := range conf.PosAttrs {
			if v.Name == "word" {
				continue
			}
			if i > 0 {
				tmp.WriteString(", ")
			}
			tmp.WriteString(fmt.Sprintf("%s=%s", v.Name, tk.Attrs[v.Name]))
			i++
		}
		tmp.WriteString("}*")
		return tmp.String()
	}
	return tk.Word
}

func getAttrs(tk *concordance.Token, conf *corpus.MQCorpusSetup) string {
	ans := make([]string, 0, len(conf.PosAttrs)-1)
	for _, v := range conf.PosAttrs {
		if v.Name != "word" {
			ans = append(ans, fmt.Sprintf("*%s*=&quot;%s&quot;", v.Name, tk.Attrs[v.Name]))
		}
	}
	return strings.Join(ans, " &amp; ")
}

func exportToken(tk *concordance.Token) string {
	if tk.Strong {
		return fmt.Sprintf("**%s**", tk.Word)
	}
	return tk.Word
}

func exportTextProps(props map[string]string, buff *strings.Builder) {
	var i int
	for k, v := range props {
		if i > 0 {
			buff.WriteString(", ")
		}
		buff.WriteString(fmt.Sprintf("**%s**: %s", k, v))
		i++
	}
}

func ConcToMarkdown(data *results.Concordance, conf *corpus.MQCorpusSetup, textProps bool) string {
	var ans strings.Builder
	if textProps {
		ans.WriteString("|left context | KWIC | right context | text properties |\n")
		ans.WriteString("|-------:|:----:|:-------|--------|\n")

	} else {
		ans.WriteString("|left context | KWIC | right context |\n")
		ans.WriteString("|-------:|:----:|:-------|\n")
	}
	for _, line := range data.Lines {
		var state int
		ans.WriteString("| \u2026 ")
		metadataBuff := make([]string, 0, 5)
		for _, ch := range line.Text {
			switch tLineElem := ch.(type) {
			case *concordance.Token:
				if state == 0 && tLineElem.Strong {
					state = 1
					ans.WriteString(" | ")

				} else if !tLineElem.Strong && state == 1 {
					ans.WriteString(" |")
					state = 2
				}
				if tLineElem.Strong {
					metadataBuff = append(metadataBuff, "["+getAttrs(tLineElem, conf)+"]")
				}
				ans.WriteString(" " + exportToken(tLineElem))
			case *concordance.Struct:
				if tLineElem.IsSelfClose {
					ans.WriteString(fmt.Sprintf(" *&lt;%s /&gt;*", tLineElem.Name))

				} else {
					ans.WriteString(fmt.Sprintf(" *&lt;%s&gt;*", tLineElem.Name))
				}
			case *concordance.CloseStruct:
				ans.WriteString(fmt.Sprintf(" *&lt;/%s&gt;*", tLineElem.Name))
			}
		}
		ans.WriteString(" \u2026")
		if textProps {
			ans.WriteString(" | ")
			exportTextProps(line.Props, &ans)
		}
		ans.WriteString("|\n")
		if len(metadataBuff) > 0 {
			ans.WriteString("|| " + strings.Join(metadataBuff, " ") + " ||\n")
		}
	}
	ans.WriteString("\n\n")
	return ans.String()
}
