// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
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

package rdb

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

func (a *Adapter) CacheResult(fn func(Query) (<-chan *WorkerResult, error), query Query) (<-chan *WorkerResult, error) {
	if len(a.cachePath) == 0 {
		return fn(query)
	}

	argKey := ""
	for _, v := range query.Args {
		argKey += fmt.Sprintf("%s", v)
	}
	hashKey := sha1.Sum([]byte(argKey))
	path := a.cachePath + "/" + query.Func + hex.EncodeToString(hashKey[:])

	pe := fs.PathExists(path)
	isf, _ := fs.IsFile(path)
	ans := make(chan *WorkerResult)
	if pe && isf {
		go func() {
			result := new(WorkerResult)
			content, err := os.ReadFile(path)
			if err != nil {
				log.Err(err).Msgf("Error while reading cache file %s", path)
			}
			split := strings.Split(string(content), "\n")
			result.ResultType = split[0]
			result.Value = json.RawMessage(split[1])
			ans <- result
		}()
		return ans, nil
	}

	wr, err := fn(query)
	go func(wr <-chan *WorkerResult) {
		rawResult := <-wr
		f, err := os.Create(path)
		if err != nil {
			log.Err(err).Msgf("Error while creating cache file %s", path)
		}
		defer f.Close()
		_, err = f.WriteString(rawResult.ResultType + "\n")
		if err != nil {
			log.Err(err).Msgf("Error while writing cache file %s", path)
		}
		_, err = f.Write(rawResult.Value)
		if err != nil {
			log.Err(err).Msgf("Error while writing cache file %s", path)
		}
		ans <- rawResult
	}(wr)
	return ans, err
}
