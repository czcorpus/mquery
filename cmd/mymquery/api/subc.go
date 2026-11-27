package api

import (
	"context"
	"net/url"
)

// subcorpus types as returned by /subcorpus
const (
	SubcTypeAlias = "alias"
	SubcTypeSaved = "saved"
)

// SubcorpusInfo is a response of /subcorpus/:corpusId/:subcorpusId
type SubcorpusInfo struct {
	ID       string `json:"id"`
	CorpusID string `json:"corpusId"`
	// Type is SubcTypeAlias (configured via text types) or SubcTypeSaved
	Type        string `json:"type"`
	Description string `json:"description"`
	// Size is the subcorpus size in tokens (saved subcorpora only)
	Size       int64 `json:"size"`
	NumRanges  int64 `json:"numRanges"`
	CorpusSize int64 `json:"corpusSize"`
}

// SubcorpusInfo checks whether a subcorpus can be used with a corpus
// and returns its properties. An unknown subcorpus results in HTTP 404,
// a saved subcorpus which cannot belong to the corpus in HTTP 422.
func (c *Client) SubcorpusInfo(ctx context.Context, corpusID, subcID, lang string) (SubcorpusInfo, error) {
	args := url.Values{}
	if lang != "" {
		args.Set("lang", lang)
	}
	var resp SubcorpusInfo
	if err := c.get(ctx, &resp, args, "subcorpus", corpusID, subcID); err != nil {
		return SubcorpusInfo{}, err
	}
	return resp, nil
}
