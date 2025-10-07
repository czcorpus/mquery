// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Institute of the Czech National Corpus,
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
	"strings"

	"github.com/czcorpus/mquery-common/corp"
)

func SubcorpusToCQL(tt corp.TextTypes) string {
	var buff strings.Builder
	for attr, values := range tt {
		pAttr := strings.Split(attr, ".")
		buff.WriteString(
			fmt.Sprintf(
				` within <%s %s="%s" />`,
				pAttr[0],
				pAttr[1],
				strings.Join(values, "|"),
			),
		)
	}
	return buff.String()
}
