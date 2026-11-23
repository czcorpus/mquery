package api

import (
	"context"
	"net/url"
	"strconv"

	"github.com/czcorpus/mquery-common/concordance"
)

// ConcordanceArgs mirrors query arguments of GET /concordance/:corpusId
type ConcordanceArgs struct {
	Query         string
	Subcorpus     string
	RowsOffset    int
	MaxRows       int
	ContextWidth  int
	ContextStruct string
	// ShowTextProps: 0 = none, 1 = basic, 2 = all
	ShowTextProps int
	NoShuffle     bool
}

func (a ConcordanceArgs) values() url.Values {
	v := url.Values{}
	v.Set("q", a.Query)
	if a.Subcorpus != "" {
		v.Set("subcorpus", a.Subcorpus)
	}
	if a.RowsOffset > 0 {
		v.Set("rowsOffset", strconv.Itoa(a.RowsOffset))
	}
	if a.MaxRows > 0 {
		v.Set("maxRows", strconv.Itoa(a.MaxRows))
	}
	if a.ContextWidth > 0 {
		v.Set("contextWidth", strconv.Itoa(a.ContextWidth))
	}
	if a.ContextStruct != "" {
		v.Set("contextStruct", a.ContextStruct)
	}
	if a.ShowTextProps > 0 {
		v.Set("showTextProps", strconv.Itoa(a.ShowTextProps))
	}
	if a.NoShuffle {
		v.Set("noShuffle", "1")
	}
	return v
}

type ConcordanceResponse struct {
	Lines      []concordance.Line `json:"lines"`
	ConcSize   int                `json:"concSize"`
	Corpname   string             `json:"corpname"`
	CorpusSize int                `json:"corpusSize"`
	IPM        float64            `json:"ipm"`
}

func (c *Client) Concordance(ctx context.Context, corpusID string, args ConcordanceArgs) (*ConcordanceResponse, error) {
	var resp ConcordanceResponse
	if err := c.get(ctx, &resp, args.values(), "concordance", corpusID); err != nil {
		return nil, err
	}
	return &resp, nil
}
