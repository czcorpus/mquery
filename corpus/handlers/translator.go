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
	"io"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/httpclient"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

func (a *Actions) RemoteQueryTranslator(ctx *gin.Context) {
	req, err := http.NewRequest("GET", "https://hypertext.cz/translate", nil)
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
	transport.IdleConnTimeout = time.Duration(15) * time.Second
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout:   time.Duration(15) * time.Second,
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
