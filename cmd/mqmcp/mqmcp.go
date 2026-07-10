package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"mquery/general"
	"mquery/mcp"

	"github.com/rs/zerolog/log"

	"github.com/mark3labs/mcp-go/server"
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

func main() {
	version := general.VersionInfo{
		Version:   normVersionInfo(version),
		BuildDate: normVersionInfo(buildDate),
		GitCommit: normVersionInfo(gitCommit),
	}

	srv := server.NewMCPServer("mquery-mcp", version.Version)
	confPath := os.Getenv("CONF_PATH")
	if confPath == "" {
		confPath = "./mqmcp.json"
	}
	conf := loadConf(confPath)
	mcp.CreateCorpInfoTool(srv, &conf)
	mcp.CreateTermSrchTool(srv, &conf)
	mcp.CreateFreqsTool(srv, &conf)
	mcp.CreateTextTypesTool(srv, &conf)
	mcp.CreateTextTypesOverviewTool(srv, &conf)
	mcp.CreateCollocationsTool(srv, &conf)
	mcp.CreateConcordanceTool(srv, &conf)

	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "stdio"
	}
	if mode != "http" && mode != "stdio" {
		log.Fatal().Str("mode", mode).Err(fmt.Errorf("invalid running mode")).Send()
		return
	}

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
		)

		mux := http.NewServeMux()
		mux.Handle("/mcp", httpServer)

		log.Info().Msgf("MQuery MCP (Streamable HTTP) listening on %s", conf.Server.ListenAddress)
		if err := http.ListenAndServe(fmt.Sprintf("%s", conf.Server.ListenAddress), mux); err != nil {
			log.Fatal().Err(err).Send()
		}
	}

}
