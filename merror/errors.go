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

package merror

import (
	"encoding/json"
	"fmt"
)

type InputError struct {
	Msg string
}

func (err InputError) Error() string {
	return err.Msg
}

func (err InputError) MarshalJSON() ([]byte, error) {
	if err.Msg != "" {
		return json.Marshal(err.Msg)
	}
	return json.Marshal(nil)
}

// ----------------------------

type InternalError struct {
	Msg string
}

func (err InternalError) Error() string {
	return err.Msg
}

func (err InternalError) MarshalJSON() ([]byte, error) {
	if err.Msg != "" {
		return json.Marshal(err.Msg)
	}
	return json.Marshal(nil)
}

// ---------------------------

type RecoveredError struct {
	Msg string
}

func (err RecoveredError) Error() string {
	return err.Msg
}

func (err RecoveredError) MarshalJSON() ([]byte, error) {
	if err.Msg != "" {
		return json.Marshal(err.Msg)
	}
	return json.Marshal(nil)
}

// ---------------------------

type TimeoutError struct {
	Msg string
}

func (err TimeoutError) Error() string {
	return err.Msg
}

func (err TimeoutError) MarshalJSON() ([]byte, error) {
	if err.Msg != "" {
		return json.Marshal(err.Msg)
	}
	return json.Marshal(nil)
}

// -----------------

func PanicValueToErr(v any) (err error) {
	switch tr := v.(type) {
	case error:
		err = fmt.Errorf("recovered panic: %w", tr)
	case string:
		err = fmt.Errorf("recovered panic: %s", tr)
	default:
		err = fmt.Errorf("recovered panic from an error of type %T", v)
	}
	return
}
