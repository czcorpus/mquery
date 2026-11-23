package api

import (
	"context"
	"net/url"
	"strconv"
)

// TextTypesArgs mirrors query arguments of GET /text-types/:corpusId
type TextTypesArgs struct {
	Query     string
	Subcorpus string
	// Attr is a structural attribute in the `struct.attr` form
	Attr     string
	MaxItems int
	FLimit   int
}

func (a TextTypesArgs) values() url.Values {
	v := url.Values{}
	v.Set("q", a.Query)
	v.Set("attr", a.Attr)
	if a.Subcorpus != "" {
		v.Set("subcorpus", a.Subcorpus)
	}
	if a.MaxItems > 0 {
		v.Set("maxItems", strconv.Itoa(a.MaxItems))
	}
	if a.FLimit > 0 {
		v.Set("flimit", strconv.Itoa(a.FLimit))
	}
	return v
}

// TextTypes returns frequencies of structural attribute values. Note that
// for each item, `Base` is the size (in tokens) of the respective text type
// and `IPM` is relative to it.
func (c *Client) TextTypes(ctx context.Context, corpusID string, args TextTypesArgs) (*FreqsResponse, error) {
	var resp FreqsResponse
	if err := c.get(ctx, &resp, args.values(), "text-types", corpusID); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StructAttrValues is an item of GET /text-types-avail-values/:corpusId.
// Note: attributes with too many values are omitted by the server.
type StructAttrValues struct {
	Struct      string   `json:"struct"`
	Attr        string   `json:"attr"`
	Values      []string `json:"values"`
	IsTruncated bool     `json:"isTruncated"`
}

func (s StructAttrValues) FullName() string {
	return s.Struct + "." + s.Attr
}

func (c *Client) TextTypesAvailValues(ctx context.Context, corpusID string) ([]StructAttrValues, error) {
	var resp struct {
		Attributes []StructAttrValues `json:"attributes"`
	}
	if err := c.get(ctx, &resp, url.Values{}, "text-types-avail-values", corpusID); err != nil {
		return nil, err
	}
	return resp.Attributes, nil
}

// TextProperty is a configured "abstract" text property (e.g. `author`)
// mapped to a concrete structural attribute (e.g. `doc.author`)
type TextProperty struct {
	ID           string
	Attr         string
	IsInOverview bool
}

// CorpusTextProperties returns text properties as configured in MQuery
// for the corpus (via /info/:corpusId?attachConf=1)
func (c *Client) CorpusTextProperties(ctx context.Context, corpusID string) ([]TextProperty, error) {
	var resp struct {
		Conf struct {
			TextProperties map[string]struct {
				Name         string `json:"name"`
				IsInOverview bool   `json:"isInOverview"`
			} `json:"textProperties"`
		} `json:"conf"`
	}
	if err := c.get(ctx, &resp, url.Values{"attachConf": {"1"}}, "info", corpusID); err != nil {
		return nil, err
	}
	ans := make([]TextProperty, 0, len(resp.Conf.TextProperties))
	for id, p := range resp.Conf.TextProperties {
		if p.Name == "" {
			continue
		}
		ans = append(ans, TextProperty{ID: id, Attr: p.Name, IsInOverview: p.IsInOverview})
	}
	return ans, nil
}
