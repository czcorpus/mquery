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

package monitoring

import (
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	logger   *WorkerJobLogger
	location *time.Location
}

func (a *Actions) WorkersLoad(ctx *gin.Context) {
	now := time.Now().In(a.location)
	dur, err := datetime.ParseDuration(ctx.Request.URL.Query().Get("ago"))
	fromDT := now.Add(-dur)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return
	}
	load, err := a.logger.WorkersLoad(fromDT, now)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return
	}
	for k, v := range load {
		load[k] = v * 100
	}
	uniresp.WriteJSONResponse(ctx.Writer, load)
}

func (a *Actions) WorkersLoadTotal(ctx *gin.Context) {
	now := time.Now().In(a.location)
	dur, err := datetime.ParseDuration(ctx.Request.URL.Query().Get("ago"))
	fromDT := now.Add(-dur)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return
	}
	load, err := a.logger.TotalLoad(fromDT, now)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return
	}

	uniresp.WriteJSONResponse(ctx.Writer, map[string]any{"loadPercent": 100 * load})

}

func NewActions(
	logger *WorkerJobLogger,
	location *time.Location,
) *Actions {
	ans := &Actions{
		logger:   logger,
		location: location,
	}
	return ans
}
