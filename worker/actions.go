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

package worker

import (
	"fmt"
	"mquery/corpus/baseinfo"
	"mquery/corpus/infoload"
	"mquery/mango"
	"mquery/merror"
	"mquery/rdb"
	"mquery/rdb/results"
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/rs/zerolog/log"
)

func (w *Worker) corpusInfo(args rdb.CorpusInfoArgs) results.CorpusInfo {
	var ans results.CorpusInfo
	ans.Data = baseinfo.Corpus{Corpname: filepath.Base(args.CorpusPath)}
	t, err := fs.IsFile(args.CorpusPath)
	if err != nil {
		ans.Error = err
		return ans
	}
	if !t {
		ans.Error = merror.InputError{
			Msg: fmt.Sprintf("Invalid corpus path: %s", args.CorpusPath)}
		return ans
	}
	err = infoload.FillStructAndAttrs(args.CorpusPath, &ans.Data)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.Data.Size, err = mango.GetCorpusSize(args.CorpusPath)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.Data.Description, err = mango.GetCorpusConf(args.CorpusPath, "INFO")
	if err != nil {
		ans.Error = err
		return ans
	}
	return ans
}

func (w *Worker) freqDistrib(args rdb.FreqDistribArgs) results.FreqDistrib {
	ans := results.FreqDistrib{Freqs: []*results.FreqDistribItem{}}
	if args.MaxItems <= 0 {
		ans.Error = merror.InputError{
			Msg: "maxItems must be a positive number"}
		return ans
	}
	freqs, err := mango.CalcFreqDist(
		args.CorpusPath, args.SubcPath, args.Query, args.Crit, args.FreqLimit)
	if err != nil {
		ans.Error = err
		return ans
	}

	var norms map[string]int64
	if args.IsTextTypes {
		attr := extractAttrFromTTCrit(args.Crit)

		var ok bool
		norms, ok = w.normsCache.Get(args.CorpusPath, attr)
		if ok {
			log.Debug().
				Str("corp", args.CorpusPath).
				Str("attr", attr).
				Msg("norms cache hit")
		} else {
			var err error
			norms, err = mango.GetTextTypesNorms(args.CorpusPath, attr)
			if err != nil {
				ans.Error = err
			}
		}
	}
	mergedFreqs, err := CompileFreqResult(
		freqs, freqs.SubcSize, args.MaxItems, norms)
	ans.Freqs = mergedFreqs
	ans.ConcSize = freqs.ConcSize
	ans.CorpusSize = freqs.CorpusSize
	ans.Fcrit = args.Crit
	return ans
}

func (w *Worker) collocations(args rdb.CollocationsArgs) results.Collocations {
	var ans results.Collocations
	msr, err := mango.ImportCollMeasure(args.Measure)
	if err != nil {
		ans.Error = err
		return ans
	}
	colls, err := mango.GetCollcations(
		args.CorpusPath,
		args.SubcPath,
		args.Query,
		args.Attr,
		msr,
		args.SrchRange,
		args.MinFreq,
		args.MaxItems,
	)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.Colls = colls.Colls
	ans.ConcSize = colls.ConcSize
	ans.CorpusSize = colls.CorpusSize
	ans.SubcSize = colls.SubcSize
	ans.Measure = args.Measure
	ans.SrchRange = args.SrchRange
	return ans
}

func (w *Worker) concSize(args rdb.ConcordanceArgs) results.ConcSize {
	var ans results.ConcSize
	concSizeInfo, err := mango.GetConcSize(args.CorpusPath, args.Query)
	if err != nil {
		ans.Error = err
		return ans
	}
	ans.Total = concSizeInfo.Value
	ans.CorpusSize = concSizeInfo.CorpusSize
	ans.ARF = concSizeInfo.ARF
	return ans
}

func (w *Worker) concordance(args rdb.ConcordanceArgs) results.Concordance {
	ans := results.Concordance{
		Lines: []concordance.Line{},
	}
	if len(args.Attrs) == 0 {
		ans.Error = merror.InputError{Msg: "No positional attributes selected for the concordance"}
		return ans
	}
	var concEx mango.GoConcordance
	var err error

	if args.CollQuery != "" {
		concEx, err = mango.GetConcordanceWithCollPhrase(
			args.CorpusPath,
			args.Query,
			args.CollQuery,
			args.CollLftCtx,
			args.CollRgtCtx,
			args.Attrs,
			args.ShowStructs,
			args.ShowRefs,
			args.StartLine,
			args.MaxItems,
			args.MaxContext,
			args.ViewContextStruct,
		)

	} else {
		concEx, err = mango.GetConcordance(
			args.CorpusPath,
			args.Query,
			args.Attrs,
			args.ShowStructs,
			args.ShowRefs,
			args.StartLine,
			args.MaxItems,
			args.MaxContext,
			args.ViewContextStruct,
		)
	}
	if err != nil {
		ans.Error = err
		return ans
	}
	parser := concordance.NewLineParser(args.Attrs)
	ans.Lines = parser.Parse(concEx.Lines)
	ans.ConcSize = concEx.ConcSize
	ans.CorpusSize = concEx.CorpusSize
	return ans
}

func (w *Worker) calcCollFreqData(args rdb.CalcCollFreqDataArgs) results.CollFreqData {
	for _, attr := range args.Attrs {
		err := mango.CompileSubcFreqs(args.CorpusPath, args.SubcPath, attr)
		if err != nil {
			return results.CollFreqData{Error: err}
		}
	}
	for _, strct := range args.Structs {
		err := w.tokenCoverage(args.MktokencovPath, args.SubcPath, args.CorpusPath, strct)
		if err != nil {
			return results.CollFreqData{Error: err}
		}
	}
	return results.CollFreqData{}
}

func (w *Worker) textTypeNorms(args rdb.TextTypeNormsArgs) results.TextTypeNorms {
	var ans results.TextTypeNorms
	norms, ok := w.normsCache.Get(args.CorpusPath, args.StructAttr)
	if ok {
		log.Debug().
			Str("corp", args.CorpusPath).
			Str("attr", args.StructAttr).
			Msg("norms cache hit")
	} else {
		var err error
		norms, err = mango.GetTextTypesNorms(args.CorpusPath, args.StructAttr)
		if err != nil {
			ans.Error = err
			return ans
		}
		w.normsCache.Set(args.CorpusPath, args.StructAttr, norms)
	}
	ans.Sizes = norms
	return ans
}

func (w *Worker) tokenContext(args rdb.TokenContextArgs) results.TokenContext {
	var ans results.TokenContext
	res1, err := mango.GetCorpRegion(
		args.CorpusPath,
		int64(max(0, args.Idx-args.LeftCtx)),
		int64(max(0, args.Idx)),
		args.Structs,
		args.Attrs,
	)
	if err != nil {
		ans.Error = err
		return ans
	}
	parser := concordance.NewLineParser(args.Attrs)
	tmp := parser.Parse([]string{res1.Text})
	if len(tmp) > 0 {
		ans.Context = tmp[0]
	}

	res2, err := mango.GetCorpRegion(
		args.CorpusPath,
		int64(args.Idx),
		int64(args.Idx+args.KWICLen),
		args.Structs,
		args.Attrs,
	)
	if err != nil {
		ans.Error = err
		return ans
	}
	tmp = parser.Parse([]string{res2.Text})
	for _, v := range tmp[0].Text {
		if vt, ok := v.(*concordance.Token); ok {
			vt.Strong = true
		}
	}
	if len(tmp) > 0 {
		ans.Context.Text = append(ans.Context.Text, tmp[0].Text...)
	}

	res3, err := mango.GetCorpRegion(
		args.CorpusPath,
		int64(args.Idx+args.KWICLen+1),
		int64(args.Idx+args.RightCtx+args.KWICLen),
		args.Structs,
		args.Attrs,
	)
	if err != nil {
		ans.Error = err
		return ans
	}
	tmp = parser.Parse([]string{res3.Text})
	if len(tmp) > 0 {
		ans.Context.Text = append(ans.Context.Text, tmp[0].Text...)
	}

	ans.Context.Ref = fmt.Sprintf("#%d", args.Idx)
	return ans
}
