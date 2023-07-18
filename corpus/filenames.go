// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

package corpus

import (
	"fmt"
	"path/filepath"
	"regexp"
)

var (
	icReg = regexp.MustCompile("intercorp_v([\\d]+)_\\w{2}")
)

// GenWSDefFilename returns word-sketch definition file (which is also
// a respective registry file value)
func GenWSDefFilename(basePath string, corpusID string) string {
	return filepath.Join(basePath, fmt.Sprintf("ws-%s.wsd", corpusID))
}

// GenWSBaseFilename returns a pair of WSBASE 'confirm existence' file
// and the actual registry value
func GenWSBaseFilename(basePath string, corpusID string, wsattr string) (string, string) {
	return filepath.Join(basePath, corpusID, fmt.Sprintf("%s-ws.lex.idx", wsattr)),
		filepath.Join(basePath, corpusID, fmt.Sprintf("%s-ws", wsattr))
}

// GenWSThesFilename returns a pair of WSTHES 'confirm existence' file
// and the actual registry value
func GenWSThesFilename(basePath string, corpusID string, wsattr string) (string, string) {
	return filepath.Join(basePath, corpusID, fmt.Sprintf("%s-thes.idx", wsattr)),
		filepath.Join(basePath, corpusID, fmt.Sprintf("%s-thes", wsattr))
}

// GenCorpusGroupName generates a proper name for corpus
// group name according to CNC's internal rules
// (e.g. intercorp_v11_en => intercorp_v11, foo => foo)
func GenCorpusGroupName(corpusID string) string {
	if v := icReg.FindStringSubmatch(corpusID); len(v) > 0 {
		return fmt.Sprintf("intercorp_v%s", v[1])
	}
	return corpusID
}

// IsIntercorpFilename tests whether the provided corpus identifier
// matches InterCorp naming patter.
func IsIntercorpFilename(corpusID string) bool {
	return icReg.MatchString(corpusID)
}
