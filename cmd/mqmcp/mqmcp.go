package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mquery/general"
	"mquery/mcp"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/mcptools"
	"github.com/rs/zerolog/log"

	"github.com/mark3labs/mcp-go/server"
)

const (
	// logPathStderr is a LOG_PATH sentinel value requesting logging
	// to stderr instead of the default log file. An unset/empty
	// LOG_PATH means "use the default log file path", so we need a
	// distinct value to explicitly request stderr.
	logPathStderr = "stderr"

	// apiHeaderEnvPrefix marks environment variables that configure
	// individual HTTP headers to send to the mquery API, e.g.
	// MQUERY_API_HEADER_X_API_KEY=secret becomes header "X-Api-Key: secret".
	// A single env var cannot hold a whole map, and stdio-mode MCP clients
	// typically inject config as a flat env var map, so headers are
	// expressed as one env var per header instead of e.g. a JSON blob.
	apiHeaderEnvPrefix = "MQUERY_API_HEADER_"

	// defaultStdioAPIUrl is used in stdio mode when neither the conf file
	// nor MQUERY_API_URL specify one, so a locally installed server works
	// out of the box against a locally running mquery API.
	defaultStdioAPIUrl = "https://www.korpus.cz/mquery"
)

var (
	version   string
	buildDate string
	gitCommit string
)

func loadConf(path string) mcp.Conf {
	if path == "" {
		log.Fatal().Msg("Cannot load config - path not specified")
	}
	rawData, err := os.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	var conf mcp.Conf
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	return conf
}

func normVersionInfo(v string) string {
	return strings.TrimLeft(strings.Trim(v, "'"), "v")
}

// headersFromEnv extracts HTTP headers to forward to the mquery API from
// environment variables of the form MQUERY_API_HEADER_<NAME>=<value>, e.g.
// MQUERY_API_HEADER_X_API_KEY=secret becomes the header "X-Api-Key: secret".
func headersFromEnv(environ []string) map[string]string {
	headers := make(map[string]string)
	for _, kv := range environ {
		name, value, found := strings.Cut(kv, "=")
		if !found || !strings.HasPrefix(name, apiHeaderEnvPrefix) {
			continue
		}
		headerName := strings.ReplaceAll(strings.TrimPrefix(name, apiHeaderEnvPrefix), "_", "-")
		if headerName == "" {
			continue
		}
		headers[http.CanonicalHeaderKey(headerName)] = value
	}
	return headers
}

func main() {
	version := general.VersionInfo{
		Version:   normVersionInfo(version),
		BuildDate: normVersionInfo(buildDate),
		GitCommit: normVersionInfo(gitCommit),
	}

	srv := server.NewMCPServer("mquery-mcp", version.Version, server.WithHooks(mcptools.DefaultLoggingHooks()))
	var conf mcp.Conf
	confPath := os.Getenv("CONF_PATH")
	if confPath != "" {
		conf = loadConf(confPath)
	}

	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "stdio"
	}
	if mode != "http" && mode != "stdio" {
		log.Fatal().Str("mode", mode).Err(fmt.Errorf("invalid running mode")).Send()
		return
	}

	// apiUrl: env overrides conf file; stdio mode falls back to a local
	// default so a freshly installed server works with zero config, while
	// http mode (a shared, network-facing service) must be explicit.
	if apiURL := os.Getenv("MQUERY_API_URL"); apiURL != "" {
		conf.APIUrl = apiURL
	}
	if conf.APIUrl == "" {
		if mode == "stdio" {
			conf.APIUrl = defaultStdioAPIUrl
		} else {
			log.Fatal().Msg("apiUrl must be configured (conf file 'apiUrl' or MQUERY_API_URL) in http mode")
		}
	}

	if envHeaders := headersFromEnv(os.Environ()); len(envHeaders) > 0 {
		if conf.APIHeaders == nil {
			conf.APIHeaders = make(map[string]string, len(envHeaders))
		}
		maps.Copy(conf.APIHeaders, envHeaders)
	}

	if addr := os.Getenv("LISTEN_ADDRESS"); addr != "" {
		conf.Server.ListenAddress = addr
	}
	if mode == "http" && conf.Server.ListenAddress == "" {
		log.Fatal().Msg("server.listenAddress must be configured (conf file or LISTEN_ADDRESS) in http mode")
	}

	logPath := os.Getenv("LOG_PATH")
	switch logPath {
	case logPathStderr:
		conf.Logging.Path = ""
	case "":
		// keep whatever the conf file set, if anything
	default:
		conf.Logging.Path = logPath
	}
	if conf.Logging.Path == "" && mode == "stdio" && logPath != logPathStderr {
		stateHome := os.Getenv("XDG_STATE_HOME")
		if stateHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to determine default log path: %s\n", err)
				os.Exit(1)
			}
			stateHome = filepath.Join(homeDir, ".local", "state")
		}
		conf.Logging.Path = filepath.Join(stateHome, "mqmcp", "mqmcp.log")
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		conf.Logging.Level = logging.LogLevel(logLevel)
	}
	if conf.Logging.Level == "" {
		conf.Logging.Level = "info"
	}

	logging.SetupLogging(conf.Logging)

	mcp.CreateCorpInfoTool(srv, &conf)
	mcp.CreateTermSrchTool(srv, &conf)
	mcp.CreateFreqsTool(srv, &conf)
	mcp.CreateTextTypesTool(srv, &conf)
	mcp.CreateTextTypesOverviewTool(srv, &conf)
	mcp.CreateCollocationsTool(srv, &conf)
	mcp.CreateConcordanceTool(srv, &conf)
	mcp.CreateTextTypesAvailValuesTool(srv, &conf)

	switch mode {
	case "stdio":
		// Start the stdio server
		if err := server.ServeStdio(srv); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}

	case "http":
		httpServer := server.NewStreamableHTTPServer(
			srv,
			server.WithEndpointPath("/mcp"),
			server.WithHTTPContextFunc(mcptools.WithClientIPContext),
		)

		mux := http.NewServeMux()
		mux.Handle("/mcp", httpServer)

		log.Info().Msgf("MQuery MCP (Streamable HTTP) listening on %s", conf.Server.ListenAddress)
		if err := http.ListenAndServe(conf.Server.ListenAddress, mux); err != nil {
			log.Fatal().Err(err).Send()
		}
	}

}
