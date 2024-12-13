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

package proxied

import (
	"io"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	translatorURL       string
	idleConnTimeoutSecs int
	requestTimeoutSecs  int
}

// RemoteQueryTranslator godoc
// @Summary      Translate
// @Description  Translate a query to CQL.
// @Produce      plain
// @Param        q query string true "the raw query"
// @Param        lang query string true "language in which the raw query is (eng or ces)" enums(ces, eng)
// @Success      200 {string} string
// @Router       /translate [get]
func (a *Actions) RemoteQueryTranslator(ctx *gin.Context) {
	req, err := http.NewRequest("GET", a.translatorURL, nil)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	q := req.URL.Query()
	q.Add("q", ctx.Query("q"))
	q.Add("lang", ctx.Query("lang"))
	req.URL.RawQuery = q.Encode()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = httpclient.TransportMaxIdleConns
	transport.MaxConnsPerHost = httpclient.TransportMaxConnsPerHost
	transport.MaxIdleConnsPerHost = httpclient.TransportMaxIdleConnsPerHost
	transport.IdleConnTimeout = time.Duration(a.idleConnTimeoutSecs) * time.Second
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout:   time.Duration(a.requestTimeoutSecs) * time.Second,
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, resp.StatusCode)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	ctx.Header("content-type", "text/plain; charset=utf-8")
	ctx.Writer.Write(body)
}

func NewActions(translatorURL string) *Actions {
	return &Actions{
		translatorURL:       translatorURL,
		idleConnTimeoutSecs: 60,
		requestTimeoutSecs:  10,
	}
}
