// mymquery is a terminal UI client for the MQuery HTTP API.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/ui"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultAPIURL = "http://localhost:8989"

type headerFlags map[string]string

func (h headerFlags) String() string {
	parts := make([]string, 0, len(h))
	for k, v := range h {
		parts = append(parts, k+": "+v)
	}
	return strings.Join(parts, ", ")
}

func (h headerFlags) Set(s string) error {
	k, v, ok := strings.Cut(s, ":")
	if !ok || strings.TrimSpace(k) == "" {
		return fmt.Errorf("invalid header %q (expected 'Name: value')", s)
	}
	h[strings.TrimSpace(k)] = strings.TrimSpace(v)
	return nil
}

func main() {
	apiURL := os.Getenv("MQUERY_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	headers := make(headerFlags)
	flag.StringVar(&apiURL, "api", apiURL, "MQuery API URL (env: MQUERY_API_URL)")
	flag.Var(headers, "H", "additional HTTP header 'Name: value' (can be repeated)")
	lang := flag.String("lang", "", "locale for corpus names and descriptions (e.g. en, cs)")
	pageSize := flag.Int("page-size", 50, "number of concordance lines per page")
	contextWidth := flag.Int("context-width", 20, "concordance context width in tokens (max. 50)")
	timeout := flag.Duration("timeout", 90*time.Second, "HTTP request timeout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n\nA terminal UI for MQuery.\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *pageSize < 1 {
		fmt.Fprintln(os.Stderr, "page-size must be >= 1")
		os.Exit(2)
	}
	if *contextWidth < 0 || *contextWidth > 50 {
		fmt.Fprintln(os.Stderr, "context-width must be between 0 and 50")
		os.Exit(2)
	}
	client, err := api.NewClient(apiURL, headers, *timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	env := ui.NewEnv(client, *lang, *pageSize, *contextWidth)
	p := tea.NewProgram(ui.NewApp(env), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
