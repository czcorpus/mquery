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

package qgen

import (
	"mquery/corpus"
	"mquery/db"
	"mquery/rdb"
	"mquery/results"

	"github.com/rs/zerolog/log"
)

type QueryExecutor struct {
	backend  *db.Backend
	radapter *rdb.Adapter
}

func (qe *QueryExecutor) FxQuery(
	gen QueryGenerator, corpusPath, word string,
) (<-chan *rdb.WorkerResult, error) {

	sql, args := gen.FxQuerySelectSQL(word)
	ans, err := qe.backend.Select(sql, args)
	ch := make(chan *rdb.WorkerResult)

	if err != nil {
		go func() {
			close(ch)
		}()
		return ch, err
	}
	if ans != nil {
		go func() {
			ch <- ans
			close(ch)
		}()
		return ch, nil

	} else {
		ch2, err := qe.radapter.PublishQuery(
			rdb.Query{
				ResultType: results.ResultTypeFx,
				Func:       "freqDistrib",
				Args:       []any{corpusPath, gen.FxQuery(word), gen.FxCrit(), 1},
			},
		)
		if err != nil {
			go func() {
				close(ch)
			}()
			return ch, err
		}
		go func() {
			res := <-ch2
			ch <- res
			close(ch)
			// now let's store data to db
			query, args := gen.FxQueryInsertSQL(word, &rdb.WorkerResult{ResultType: res.ResultType})
			newID, err := qe.backend.Insert(query, args)
			if err != nil {
				log.Error().Err(err).Msg("Failed to insert cache data (Fx)")
				return
			}
			query, args = gen.FxCritInsertSQL(newID, res)
			_, err = qe.backend.Insert(query, args)
			if err != nil {
				log.Error().Err(err).Msg("Failed to insert cache data (Fx)")
				return
			}
		}()
		return ch, nil
	}
}

func (qe *QueryExecutor) FyQuery(
	gen QueryGenerator, corpusPath, word string,
) (<-chan *rdb.WorkerResult, error) {
	sql, args := gen.FyQuerySelectSQL(word)
	ans, err := qe.backend.Select(sql, args)
	ch := make(chan *rdb.WorkerResult)

	if err != nil {
		go func() {
			close(ch)
		}()
		return ch, err
	}
	if ans != nil {
		go func() {
			ch <- ans
			close(ch)
		}()
		return ch, nil

	} else {
		ch2, err := qe.radapter.PublishQuery(
			rdb.Query{
				ResultType: results.ResultTypeFy,
				Func:       "concSize",
				Args:       []any{corpusPath, gen.FyQuery(word)},
			},
		)
		if err != nil {
			go func() {
				close(ch)
			}()
			return ch, err
		}
		go func() {
			res := <-ch2
			ch <- res
			close(ch)
			// now let's store data to db
			query, args := gen.FyQueryInsertSQL(word, res)
			_, err := qe.backend.Insert(query, args)
			if err != nil {
				log.Error().Err(err).Msg("failed to insert cache data for FyQuery")
			}
		}()
		return ch, nil
	}
}

func (qe *QueryExecutor) FxyQuery(
	gen QueryGenerator, corpusPath, word, collCandidate string,
) (<-chan *rdb.WorkerResult, error) {
	sql, args := gen.FxyQuerySelectSQL(word, collCandidate)
	ans, err := qe.backend.Select(sql, args)
	ch := make(chan *rdb.WorkerResult)

	if err != nil {
		go func() {
			close(ch)
		}()
		return ch, err
	}
	if ans != nil {
		go func() {
			ch <- ans
			close(ch)
		}()
		return ch, nil

	} else {
		ch2, err := qe.radapter.PublishQuery(
			rdb.Query{
				ResultType: results.ResultTypeFxy,
				Func:       "concSize",
				Args:       []any{corpusPath, gen.FxyQuery(word, collCandidate)},
			},
		)
		if err != nil {
			go func() {
				close(ch)
			}()
			return ch, err
		}
		go func() {
			res := <-ch2
			ch <- res
			close(ch)
			// now let's store data to db
			query, args := gen.FxyQueryInsertSQL(word, collCandidate, res)
			_, err := qe.backend.Insert(query, args)
			if err != nil {
				log.Error().Err(err).Msg("failed to insert cache data for FxyQuery")
			}
		}()
		return ch, nil
	}
}

func (qe *QueryExecutor) NewReorderCalculator(
	corpConf *corpus.CorporaSetup,
	corpusPath string,
	qGen QueryGenerator,
) *ReorderCalculator {

	return &ReorderCalculator{
		corpConf:   corpConf,
		corpusPath: corpusPath,
		qGen:       qGen,
		radapter:   qe.radapter,
		executor:   qe,
	}
}

func NewQueryExecutor(
	backend *db.Backend,
	radapter *rdb.Adapter,
) *QueryExecutor {
	return &QueryExecutor{
		backend:  backend,
		radapter: radapter,
	}
}
