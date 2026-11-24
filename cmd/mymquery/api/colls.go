package api

import (
	"context"
	"net/url"
	"strconv"
)

// CollocationsArgs mirrors query arguments of GET /collocations/:corpusId
type CollocationsArgs struct {
	Query     string
	Subcorpus string
	Measure   string
	// SrchLeft and SrchRight define the window (both are >= 0, not both 0)
	SrchLeft  int
	SrchRight int
	// SrchAttr is a positional attribute collocates are calculated for
	SrchAttr    string
	MinCollFreq int
	MaxItems    int
}

func (a CollocationsArgs) values() url.Values {
	v := url.Values{}
	v.Set("q", a.Query)
	if a.Subcorpus != "" {
		v.Set("subcorpus", a.Subcorpus)
	}
	if a.Measure != "" {
		v.Set("measure", a.Measure)
	}
	// always sent - zero is a valid value different from the server default
	v.Set("srchLeft", strconv.Itoa(a.SrchLeft))
	v.Set("srchRight", strconv.Itoa(a.SrchRight))
	if a.SrchAttr != "" {
		v.Set("srchAttr", a.SrchAttr)
	}
	if a.MinCollFreq > 0 {
		v.Set("minCollFreq", strconv.Itoa(a.MinCollFreq))
	}
	if a.MaxItems > 0 {
		v.Set("maxItems", strconv.Itoa(a.MaxItems))
	}
	return v
}

type CollItem struct {
	Value string  `json:"word"`
	Score float64 `json:"score"`
	Freq  int64   `json:"freq"`
}

type CollocationsResponse struct {
	ConcSize   int64      `json:"concSize"`
	CorpusSize int64      `json:"corpusSize"`
	SubcSize   int64      `json:"subcSize"`
	Colls      []CollItem `json:"colls"`
	Measure    string     `json:"measure"`
	// SrchRange is [left, right], both as non-negative numbers
	SrchRange [2]int `json:"srchRange"`
}

func (c *Client) Collocations(ctx context.Context, corpusID string, args CollocationsArgs) (*CollocationsResponse, error) {
	var resp CollocationsResponse
	if err := c.get(ctx, &resp, args.values(), "collocations", corpusID); err != nil {
		return nil, err
	}
	return &resp, nil
}
