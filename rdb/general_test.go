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

package rdb

import (
	"errors"
	"mquery/merror"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUserError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"input error", merror.InputError{Msg: "invalid rows range"}, true},
		{"input error pointer", &merror.InputError{Msg: "invalid rows range"}, true},
		{
			"syntax error",
			merror.InternalError{Msg: `query [lemma="pes": syntax error, unexpected end of file, expecting RBRACKET or BIN_OR near position 12`},
			true,
		},
		{
			"invalid regex",
			merror.InternalError{Msg: `query [lemma="pes("]: regexopt: regex optimization parsing error at position 4: syntax error, unexpected end of file near position 8`},
			true,
		},
		{
			"unknown attribute in query",
			merror.InternalError{Msg: `query [lemmma="pes"]: AttrNotFound (lemmma) near position 9`},
			true,
		},
		{
			"unknown structure in query",
			merror.InternalError{Msg: `query [lemma="pes"] within <foo/>: AttrNotFound (foo) near position 26`},
			true,
		},
		{
			"unknown attribute outside of query",
			merror.InternalError{Msg: `query [lemma="pes"]: AttrNotFound (sublemma)`},
			false,
		},
		{"system error", errors.New("FileAccessError (/var/lib/manatee/data/syn2020/word.lex)"), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, IsUserError(c.err), c.name)
	}
}
