// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openapi

import (
	"fmt"
	"mquery/cnf"
	"net/http"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

var (
	supportedSubscribers = []string{
		"corpus-linguist",
		"slovo-v-kostce",
		"",
	}
)

func MkHandleRequest(conf *cnf.Conf, ver string) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		subscr := ctx.Query("subscriber")
		if !collections.SliceContains(supportedSubscribers, subscr) {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("unknown subscriber"),
				http.StatusNotFound,
			)
			return
		}
		ans := NewResponse(ver, conf.PublicURL, ctx.Query("subscriber"))
		uniresp.WriteJSONResponse(ctx.Writer, &ans)
	}
}
