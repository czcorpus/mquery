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
	"net/url"
	"slices"
	"strings"

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

func findHTTPProtocol(req *http.Request) string {
	if prot := req.Header.Get("x-forwarded-proto"); prot != "" {
		return prot
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

func findHTTPServer(req *http.Request) string {
	if serv := req.Header.Get("x-forwarded-host"); serv != "" {
		return serv
	}
	return req.Host
}

func findPath(req *http.Request) string {
	if path := req.Header.Get("x-original-path"); path != "" {
		return path
	}
	return req.URL.Path
}

func findCurrentPublicURL(conf *cnf.Conf, req *http.Request) string {
	proto := findHTTPProtocol(req)
	host := findHTTPServer(req)
	path := findPath(req)
	curr, err := url.JoinPath(fmt.Sprintf("%s://%s", proto, host), path)
	if err != nil {
		panic(fmt.Errorf("cannot find current public url: %w", err))
	}
	publicURLs := make([]string, len(conf.PublicURLs))
	copy(publicURLs, conf.PublicURLs)
	slices.Sort(publicURLs)
	slices.Reverse(publicURLs)
	for _, addr := range publicURLs {
		if strings.HasPrefix(curr, addr) {
			return addr
		}
	}
	return ""
}

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
		publicURL := findCurrentPublicURL(conf, ctx.Request)
		ans := NewResponse(ver, publicURL, ctx.Query("subscriber"))
		uniresp.WriteJSONResponse(ctx.Writer, &ans)
	}
}
