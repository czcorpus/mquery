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

package baseinfo

const (
	TextPropertyAuthor      = "author"
	TextPropertyTitle       = "title"
	TextPropertyPubYear     = "publication-year"
	TextPropertyTranslator  = "translator"
	TextPropertyOriginaLang = "original-language"
	TextPropertyTextType    = "text-type"
)

type TextProperty string

func (tp TextProperty) Validate() bool {
	return tp == TextPropertyAuthor || tp == TextPropertyTitle ||
		tp == TextPropertyPubYear || tp == TextPropertyTextType ||
		tp == TextPropertyTranslator || tp == TextPropertyOriginaLang
}

func (tp TextProperty) String() string {
	return string(tp)
}

func (tp TextProperty) IsZero() bool {
	return tp == ""
}
