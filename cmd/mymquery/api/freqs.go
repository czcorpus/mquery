package api

import (
	"context"
	"net/url"
	"strconv"
)

// FreqsArgs mirrors query arguments of GET /freqs/:corpusId
type FreqsArgs struct {
	Query     string
	Subcorpus string
	// Attr is a positional attribute (server default is `lemma`)
	Attr      string
	MatchCase bool
	MaxItems  int
	// FLimit is a minimum frequency of an item (server default is 1)
	FLimit int
}

func (a FreqsArgs) values() url.Values {
	v := url.Values{}
	v.Set("q", a.Query)
	if a.Subcorpus != "" {
		v.Set("subcorpus", a.Subcorpus)
	}
	if a.Attr != "" {
		v.Set("attr", a.Attr)
	}
	if a.MatchCase {
		v.Set("matchCase", "1")
	}
	if a.MaxItems > 0 {
		v.Set("maxItems", strconv.Itoa(a.MaxItems))
	}
	if a.FLimit > 0 {
		v.Set("flimit", strconv.Itoa(a.FLimit))
	}
	return v
}

type FreqItem struct {
	// Value is the attribute value; for multi-token KWICs,
	// values of individual tokens are separated by a space
	Value string  `json:"word"`
	Freq  int64   `json:"freq"`
	Base  int64   `json:"base"`
	IPM   float64 `json:"ipm"`
}

type FreqsResponse struct {
	ConcSize   int64      `json:"concSize"`
	CorpusSize int64      `json:"corpusSize"`
	SubcSize   int64      `json:"subcSize"`
	Freqs      []FreqItem `json:"freqs"`
	Fcrit      string     `json:"fcrit"`
}

func (c *Client) Freqs(ctx context.Context, corpusID string, args FreqsArgs) (*FreqsResponse, error) {
	var resp FreqsResponse
	if err := c.get(ctx, &resp, args.values(), "freqs", corpusID); err != nil {
		return nil, err
	}
	return &resp, nil
}
