// mymquery is a terminal UI client for the MQuery HTTP API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/llm"
	"mquery/cmd/mymquery/storage"
	"mquery/cmd/mymquery/ui"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultAPIURL   = "http://localhost:8989"
	defaultLoginURL = "https://korpus.cz/login"
	// defaultLLMModels is a comma separated list of Gemini models
	// (the first one is the primary one, the others are fallbacks)
	defaultLLMModels = "gemini-3.5-flash-lite,gemini-3.1-flash-lite"
)

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

// envFileArgs pre-scans the command line for `-env-file` and `-no-env-file`.
// They must be known before the other flags are defined, because the .env
// file provides their defaults.
func envFileArgs(args []string) (path string, skip bool) {
	path = ".env"
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") {
			continue // a value of another flag (we don't know which ones take values)
		}
		name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		switch name {
		case "env-file":
			if hasVal {
				path = val
			} else if i+1 < len(args) {
				i++
				path = args[i]
			}
		case "no-env-file":
			skip = !hasVal || val == "true" || val == "1"
		}
	}
	return
}

func main() {
	envPath, skipEnvFile := envFileArgs(os.Args[1:])
	dotenv := map[string]string{}
	if !skipEnvFile {
		var err error
		dotenv, err = loadDotEnv(envPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read %s: %s\n", envPath, err)
			os.Exit(2)
		}
	}
	apiURL := envLookup(dotenv, "MQUERY_API_URL")
	if apiURL == "" {
		apiURL = envLookup(dotenv, "API_URL")
	}
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	loginURL := envLookup(dotenv, "LOGIN_URL")
	if loginURL == "" {
		loginURL = defaultLoginURL
	}
	apiToken := envLookup(dotenv, "API_TOKEN")
	geminiAPIKey := envLookup(dotenv, "GEMINI_API_KEY")
	llmModels := envLookup(dotenv, "LLM_MODEL")
	if llmModels == "" {
		llmModels = defaultLLMModels
	}
	headers := make(headerFlags)
	flag.String("env-file", envPath, "file with environment variables (API_URL, API_TOKEN, LOGIN_URL, GEMINI_API_KEY, LLM_MODEL)")
	flag.Bool("no-env-file", skipEnvFile, "do not read the env file (real environment variables still apply)")
	flag.StringVar(&apiURL, "api", apiURL, "MQuery API URL (env: MQUERY_API_URL or API_URL)")
	flag.StringVar(&loginURL, "login-url", loginURL, "login URL used when API_TOKEN is set (env: LOGIN_URL)")
	flag.Var(headers, "H", "additional HTTP header 'Name: value' (can be repeated)")
	flag.StringVar(&llmModels, "llm-model", llmModels,
		"AI chat model(s); comma separated, the first one is primary, the others are fallbacks (env: LLM_MODEL)")
	lang := flag.String("lang", "", "locale for corpus names and descriptions (e.g. en, cs)")
	pageSize := flag.Int("page-size", 50, "number of concordance lines per page")
	contextWidth := flag.Int("context-width", 20, "concordance context width in tokens (max. 50)")
	dbPath := flag.String("db", storage.DefaultPath(), "SQLite database for query history and AI chats (empty = do not store anything)")
	historySize := flag.Int("history-size", ui.DefaultHistorySize, "max. number of stored queries")
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
	if apiToken != "" {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		err := client.Login(ctx, loginURL, apiToken)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	env := ui.NewEnv(client, *lang, *pageSize, *contextWidth)
	env.HistorySize = *historySize
	if *dbPath != "" {
		db, err := storage.Open(*dbPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer db.Close()
		env.DB = db
	}
	if geminiAPIKey == "" {
		env.ChatUnavailable = "AI chat requires GEMINI_API_KEY (environment or the .env file)"

	} else {
		sess, tools, err := newChatSession(client, *lang, geminiAPIKey, llmModels)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to initialize AI chat:", err)
			os.Exit(1)
		}
		defer tools.Close()
		if env.DB != nil {
			sess.Recorder = env.DB
		}
		env.Chat = sess
	}
	p := tea.NewProgram(ui.NewApp(env), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newChatSession(client *api.Client, lang, apiKey, models string) (*agent.Session, *agent.Toolbox, error) {
	var modelList []string
	for _, m := range strings.Split(models, ",") {
		if m = strings.TrimSpace(m); m != "" {
			modelList = append(modelList, m)
		}
	}
	ctx := context.Background()
	backend, err := llm.NewGenAIBackend(ctx, apiKey, modelList)
	if err != nil {
		return nil, nil, err
	}
	tools, err := agent.NewToolbox(ctx, client, lang, "mymquery")
	if err != nil {
		return nil, nil, err
	}
	return agent.NewSession(backend, tools, agent.NewStore()), tools, nil
}
