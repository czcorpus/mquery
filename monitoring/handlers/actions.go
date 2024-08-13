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

package handlers

import (
	"fmt"
	"mquery/monitoring"
	"net/http"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type timeSpan string

func (ts timeSpan) Validate() error {
	if ts != spanTypeRecent && ts != spanTypeTotal {
		return fmt.Errorf("unknown time span `%s`", ts)
	}
	return nil
}

const (
	spanTypeRecent timeSpan = "recent"
	spanTypeTotal  timeSpan = "total"
)

type Actions struct {
	logger   *monitoring.WorkerJobLogger
	location *time.Location
}

func (a *Actions) WorkersLoad(ctx *gin.Context) {

	span := timeSpan(ctx.DefaultQuery("span", "recent"))
	if err := span.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	var ans monitoring.WorkerLoad
	if span == spanTypeRecent {
		ans = a.logger.RecentLoad()

	} else if span == spanTypeTotal {
		ans = a.logger.TotalLoad()
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) SingleWorkerLoad(ctx *gin.Context) {

	span := timeSpan(ctx.DefaultQuery("span", "recent"))
	if err := span.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	workerID := ctx.Param("workerId")

	var ans monitoring.WorkerLoad
	var srchErr error
	if span == spanTypeRecent {
		ans, srchErr = a.logger.RecentWorkerLoad(workerID)

	} else if span == spanTypeTotal {
		ans, srchErr = a.logger.TotalWorkerLoad(workerID)
	}
	if srchErr == monitoring.ErrWorkerNotFound {
		uniresp.RespondWithErrorJSON(ctx, srchErr, http.StatusNotFound)
		return

	} else if srchErr != nil {
		uniresp.RespondWithErrorJSON(ctx, srchErr, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) RecentRecords(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, a.logger.RecentRecords())
}

func NewActions(
	logger *monitoring.WorkerJobLogger,
	location *time.Location,
) *Actions {
	ans := &Actions{
		logger:   logger,
		location: location,
	}
	return ans
}
