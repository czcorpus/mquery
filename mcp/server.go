package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// argToString converts a query argument value to its string representation.
// Bools are encoded as "1"/"0" rather than "true"/"false".
func argToString(v any) string {
	switch tv := v.(type) {
	case bool:
		if tv {
			return "1"
		}
		return "0"
	case string:
		return tv
	default:
		return fmt.Sprint(tv)
	}
}

// JoinURL appends chunks as path segments to base, e.g. JoinURL("http://acme.localhost",
// "subdir1", "subdir1.a") produces "http://acme.localhost/subdir1/subdir1.a". Each chunk
// is validated to be a single non-empty path segment (no "/", "?" or "#").
func JoinURL(base string, chunks ...string) (string, error) {
	parsedURL, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("failed to join URL: %w", err)
	}
	segments := make([]string, 0, len(chunks)+1)
	segments = append(segments, strings.TrimRight(parsedURL.Path, "/"))
	for _, c := range chunks {
		if c == "" {
			return "", fmt.Errorf("failed to join URL: empty path chunk")
		}
		if strings.ContainsAny(c, "/?#") {
			return "", fmt.Errorf("failed to join URL: invalid path chunk %q", c)
		}
		segments = append(segments, c)
	}
	parsedURL.Path = strings.Join(segments, "/")
	return parsedURL.String(), nil
}

// httpRequest performs an HTTP request with the given method against rawURL,
// passing args as URL query parameters, and returns the response body as a string.
func httpRequest(ctx context.Context, method, rawURL string, args map[string]any) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := make(url.Values, len(args))
	for k, v := range args {
		if v == nil {
			continue
		}
		query.Set(k, argToString(v))
	}
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
