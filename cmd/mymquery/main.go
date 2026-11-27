// mymquery is a terminal UI client for the MQuery HTTP API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/llm"
	"mquery/cmd/mymquery/storage"
	"mquery/cmd/mymquery/ui"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultAPIURL   = "http://localhost:8989"
	defaultLoginURL = "https://korpus.cz/login"
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
// They must be known before the other flags are defined, because the env
// files provide their defaults. The path is "" if `-env-file` is not set.
func envFileArgs(args []string) (path string, skip bool) {
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
	cfgLoc := defaultConfigLocations(envPath)
	dotenv := map[string]string{}
	var configWarnings []string
	if !skipEnvFile {
		var err error
		dotenv, configWarnings, err = loadConfig(cfgLoc)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
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
	cqlizerSpec := envLookup(dotenv, "CQLIZER_MCP")
	llmCnf := llmConf{
		backend:    envLookup(dotenv, "LLM_BACKEND"),
		models:     envLookup(dotenv, "LLM_MODEL"),
		geminiKey:  envLookup(dotenv, "GEMINI_API_KEY"),
		mistralKey: envLookup(dotenv, "MISTRAL_API_KEY"),
		openaiURL:  envLookup(dotenv, "OPENAI_API_URL"),
		openaiKey:  envLookup(dotenv, "OPENAI_API_KEY"),
	}
	headers := make(headerFlags)
	flag.String("env-file", envPath, "an env file with configuration (API_URL, API_TOKEN, LOGIN_URL, "+
		"LLM_BACKEND, LLM_MODEL, GEMINI_API_KEY, MISTRAL_API_KEY, OPENAI_API_URL, OPENAI_API_KEY, CQLIZER_MCP); by default, "+
		"$XDG_CONFIG_DIRS/mymquery/env (e.g. /etc/xdg/mymquery/env) and then "+
		"$XDG_CONFIG_HOME/mymquery/env (e.g. ~/.config/mymquery/env) are read, plus ./.env if "+
		"the binary is run from the current directory (a development setup); a later file "+
		"overrides an earlier one, real environment variables override the files. "+
		"With this option, it is read instead of ./.env")
	flag.Bool("no-env-file", skipEnvFile, "do not read any env files (real environment variables still apply)")
	flag.StringVar(&apiURL, "api", apiURL, "MQuery API URL (env: MQUERY_API_URL or API_URL)")
	flag.StringVar(&loginURL, "login-url", loginURL, "login URL used when API_TOKEN is set (env: LOGIN_URL)")
	flag.Var(headers, "H", "additional HTTP header 'Name: value' (can be repeated)")
	flag.StringVar(&llmCnf.backend, "llm-backend", llmCnf.backend, fmt.Sprintf(
		"AI chat backend: %s (needs GEMINI_API_KEY), %s (needs MISTRAL_API_KEY) or %s "+
			"(an OpenAI-compatible server, e.g. llama-server, at -llm-url; optional OPENAI_API_KEY); "+
			"empty = the first one with an API key, then %s if -llm-url is set (env: LLM_BACKEND)",
		llmBackendGemini, llmBackendMistral, llmBackendOpenAI, llmBackendOpenAI))
	flag.StringVar(&llmCnf.openaiURL, "llm-url", llmCnf.openaiURL, fmt.Sprintf(
		"URL of the OpenAI-compatible server of the %s backend; empty = %s (env: OPENAI_API_URL)",
		llmBackendOpenAI, llm.DefaultOpenAIAPIURL))
	flag.StringVar(&llmCnf.models, "llm-model", llmCnf.models, fmt.Sprintf(
		"AI chat model(s); comma separated, the first one is primary, the others are fallbacks; "+
			"empty = %q for %s, %q for %s, the server's model for %s (env: LLM_MODEL)",
		defaultLLMModels[llmBackendGemini], llmBackendGemini,
		defaultLLMModels[llmBackendMistral], llmBackendMistral, llmBackendOpenAI))
	flag.StringVar(&cqlizerSpec, "cqlizer", cqlizerSpec,
		"CQLizer MCP server providing CQL helper tools to the AI chat: an URL (Streamable HTTP, "+
			"e.g. http://localhost:8080/mcp) or a command line of a stdio server (e.g. \"cqlizer mcp-server\"; "+
			"it gets the environment incl. the env file, e.g. CQLIZER_CONF_PATH, REGISTRY_PATH); "+
			"empty = no helper tools (env: CQLIZER_MCP)")
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
	env.StartupNotice = strings.Join(configWarnings, "; ")
	if *dbPath != "" {
		db, err := storage.Open(*dbPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer db.Close()
		env.DB = db
	}
	llmSetup, chatUnavailable, err := llmCnf.resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	chats := &chatManager{
		client: client, lang: *lang, db: env.DB, llmConf: llmCnf, cqlizerSpec: cqlizerSpec,
		helperEnv: helperServerEnv(dotenv), userConfig: cfgLoc.userConfigPath(),
	}
	defer chats.Close()
	env.ChatBackends = chatBackends()
	env.ChatConfigure = chats.configure
	env.ChatConfigPath = chats.userConfig
	if chatUnavailable != "" {
		// (the user can configure the chat in the UI)
		env.ChatUnavailable = chatUnavailable
		env.ChatCurrent = ui.ChatSetup{
			Backend: llmCnf.backend, Models: llmCnf.models, URL: llmCnf.openaiURL}

	} else {
		sess, notice, err := chats.connect(llmSetup)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to initialize AI chat:", err)
			os.Exit(1)
		}
		env.Chat = sess
		env.ChatNotice = notice
		env.ChatCurrent = chats.current()
	}
	p := tea.NewProgram(ui.NewApp(env), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// mymquerySecrets are env file variables not passed to helper servers
var mymquerySecrets = []string{"API_TOKEN", "GEMINI_API_KEY", "MISTRAL_API_KEY", "OPENAI_API_KEY"}

// helperServerEnv returns env file variables for a stdio helper server
// (e.g. CQLIZER_CONF_PATH, REGISTRY_PATH; real environment variables take
// precedence, mymquery's secrets are not passed)
func helperServerEnv(dotenv map[string]string) []string {
	var ans []string
	for k, v := range dotenv {
		if _, ok := os.LookupEnv(k); ok || slices.Contains(mymquerySecrets, k) {
			continue
		}
		ans = append(ans, k+"="+v)
	}
	slices.Sort(ans)
	return ans
}
