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
	"net/http/cookiejar"
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
	baseURL       *url.URL
	headers       map[string]string
	http          *http.Client
	authenticated bool
}

func NewClient(baseURL string, headers map[string]string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid API URL %q: scheme must be http or https", baseURL)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: u,
		headers: headers,
		http:    &http.Client{Timeout: timeout, Jar: jar},
	}, nil
}

func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// Authenticated reports whether a successful Login was performed.
func (c *Client) Authenticated() bool {
	return c.authenticated
}

// Login authenticates using a personal access token as described in
// https://wiki.korpus.cz/doku.php/en:manualy:api – the token is POSTed
// as the `personal_access_token` form field to `loginURL` and the returned
// session cookie is kept in the client's cookie jar and sent along with
// all subsequent API requests.
func (c *Client) Login(ctx context.Context, loginURL, token string) error {
	lu, err := url.Parse(loginURL)
	if err != nil || (lu.Scheme != "http" && lu.Scheme != "https") {
		return fmt.Errorf("invalid login URL %q", loginURL)
	}
	form := url.Values{"personal_access_token": {token}}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, lu.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("login failed: %w", decodeError(resp.StatusCode, body))
	}
	if len(c.http.Jar.Cookies(c.baseURL)) == 0 {
		return fmt.Errorf(
			"login to %s returned no session cookie usable for %s (invalid token or a different domain?)",
			lu.Host, c.baseURL.Host)
	}
	c.authenticated = true
	return nil
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

// ------ / (server info)

type VersionInfo struct {
	Version    string `json:"version"`
	BuildDate  string `json:"buildDate"`
	LastCommit string `json:"lastCommit"`
}

type ServerInfo struct {
	Name        string      `json:"name"`
	About       string      `json:"about"`
	VersionInfo VersionInfo `json:"versionInfo"`
}

// ServerInfo fetches basic information about the server (including
// its version) from the API root path.
func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) {
	var resp ServerInfo
	if err := c.get(ctx, &resp, nil); err != nil {
		return ServerInfo{}, err
	}
	return resp, nil
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
