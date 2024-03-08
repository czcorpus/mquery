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

package handlers

import (
	"errors"
	"fmt"
	"mquery/corpus"
	"net/http"

	"github.com/gin-gonic/gin"
)

type queryProps struct {
	corpus     string
	query      string
	err        error
	corpusConf *corpus.CorpusSetup
	status     int
}

func (qp queryProps) hasError() bool {
	return qp.err != nil
}

func DetermineQueryProps(ctx *gin.Context, cConf *corpus.CorporaSetup) queryProps {
	var ans queryProps
	ans.corpus = ctx.Param("corpusId")
	corpusConf := cConf.Resources.Get(ans.corpus)
	if corpusConf == nil {
		ans.err = fmt.Errorf("corpus %s not found", ans.corpus)
		ans.status = http.StatusNotFound
		return ans
	}
	ans.corpusConf = corpusConf

	var ttCQL string
	userQuery := ctx.Query("q")
	if userQuery == "" {
		ans.err = errors.New("empty query")
		ans.status = http.StatusUnprocessableEntity
		return ans
	}
	subc := ctx.Query("subcorpus")
	if subc != "" {
		ttCQL = corpus.SubcorpusToCQL(corpusConf.Subcorpora[subc].TextTypes)
		if ttCQL == "" {
			ans.err = errors.New("invalid subcorpus specification")
			ans.status = http.StatusUnprocessableEntity
			return ans
		}
	}
	ans.query = userQuery + ttCQL
	return ans
}
