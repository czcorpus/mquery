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

package mango

import "errors"

var (
	collFunc = map[string]byte{
		"absFreq":        'f',
		"logLikelihood":  'l',
		"logDice":        'd',
		"minSensitivity": 's',
		"mutualInfo":     'm',
		"mutualInfo3":    '3',
		"mutualInfoLogF": 'p',
		"relFreq":        'r',
		"tScore":         't',
	}

	ErrUnsupportedValue = errors.New("unsupported value")
)

// ImportCollMeasure transforms API defined collocation names
// (absFreq, logLikelihood, logDice, minSensitivity, mutualInfo, mutualInfo3,
// mutualInfoLogF, relFreq, tScore) into Manatee internal codes (f, l, d, s, m, 3, ...).
// The function also accepts the Manatee codes directly in which case it behaves
// like an identity (or rather string to byte conversion). This is mostly for old time
// users of Manatee library for easier API adoption.
func ImportCollMeasure(v string) (byte, error) {
	imp, ok := collFunc[v]
	if !ok {
		for _, fnCode := range collFunc {
			if v == string(fnCode) {
				return fnCode, nil
			}
		}
		return 0, ErrUnsupportedValue
	}
	return imp, nil
}

func NormalizeCollMeasureName(v string) (string, error) {
	_, ok := collFunc[v]
	if ok {
		return v, nil
	}
	for k, v2 := range collFunc {
		if len(v) == 1 && v[0] == v2 {
			return k, nil
		}
	}
	return "", ErrUnsupportedValue
}
