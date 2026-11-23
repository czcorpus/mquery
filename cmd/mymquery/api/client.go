// Package api is a thin HTTP client for the public (GET) part
// of the MQuery API. It intentionally does not depend on any
// MQuery server package (to stay CGO-free).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/mquery-common/corp"
)

const maxErrBodyPreview = 300

// Error represents a non-2xx response from MQuery
type Error struct {
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("MQuery error (HTTP %d): %s", e.StatusCode, e.Message)
}

type Client struct {
	baseURL *url.URL
	headers map[string]string
	http    *http.Client
}

func NewClient(baseURL string, headers map[string]string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid API URL %q: scheme must be http or https", baseURL)
	}
	return &Client{
		baseURL: u,
		headers: headers,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// get performs a GET request on base URL + path elements
// and decodes JSON response into `out`.
func (c *Client) get(ctx context.Context, out any, args url.Values, path ...string) error {
	u := c.baseURL.JoinPath(path...)
	u.RawQuery = args.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeError(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

func decodeError(status int, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return &Error{StatusCode: status, Message: errResp.Error}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > maxErrBodyPreview {
		msg = msg[:maxErrBodyPreview] + "…"
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &Error{StatusCode: status, Message: msg}
}

// ------ /corplist

type Subcorpus struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type CorpusItem struct {
	ID          string      `json:"id"`
	FullName    string      `json:"fullName"`
	Description string      `json:"description"`
	Language    string      `json:"language"`
	Flags       []string    `json:"flags"`
	Subcorpora  []Subcorpus `json:"subcorpora"`
}

type corplistResponse struct {
	Corpora []CorpusItem `json:"corpora"`
	Locale  string       `json:"locale"`
}

func (c *Client) Corplist(ctx context.Context, lang string) ([]CorpusItem, error) {
	args := url.Values{}
	if lang != "" {
		args.Set("lang", lang)
	}
	var resp corplistResponse
	if err := c.get(ctx, &resp, args, "corplist"); err != nil {
		return nil, err
	}
	return resp.Corpora, nil
}

// ------ /info/:corpusId

type corpusInfoResponse struct {
	Corpus struct {
		Data corp.Overview `json:"data"`
	} `json:"corpus"`
}

func (c *Client) CorpusInfo(ctx context.Context, corpusID, lang string) (corp.Overview, error) {
	args := url.Values{}
	if lang != "" {
		args.Set("lang", lang)
	}
	var resp corpusInfoResponse
	if err := c.get(ctx, &resp, args, "info", corpusID); err != nil {
		return corp.Overview{}, err
	}
	return resp.Corpus.Data, nil
}
