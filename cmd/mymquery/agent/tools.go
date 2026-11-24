package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/llm"
	mqmcp "mquery/mcp"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CorpusListToolName is a builtin tool listing available corpora
// (the MQuery MCP server has no such tool).
const CorpusListToolName = "corpus_list"

// Toolbox provides tools of an in-process MQuery MCP server
// (sharing the HTTP client - incl. a session cookie - with the UI)
// along with builtin tools.
type Toolbox struct {
	apiClient *api.Client
	lang      string
	mcpClient *client.Client
	defs      []llm.ToolDef
}

// NewToolbox starts an in-process MQuery MCP server and connects to it.
// The lang is a locale for corpus descriptions ("" = server default).
func NewToolbox(ctx context.Context, apiClient *api.Client, lang, version string) (*Toolbox, error) {
	conf := &mqmcp.Conf{
		APIUrl:     apiClient.BaseURL(),
		APIHeaders: apiClient.Headers(),
		HTTPClient: apiClient.HTTPClient(),
	}
	srv := server.NewMCPServer("mquery-mcp", version)
	mqmcp.CreateCorpInfoTool(srv, conf)
	mqmcp.CreateTermSrchTool(srv, conf)
	mqmcp.CreateFreqsTool(srv, conf)
	mqmcp.CreateTextTypesTool(srv, conf)
	mqmcp.CreateTextTypesOverviewTool(srv, conf)
	mqmcp.CreateCollocationsTool(srv, conf)
	mqmcp.CreateConcordanceTool(srv, conf)
	mqmcp.CreateTextTypesAvailValuesTool(srv, conf)

	cl, err := client.NewInProcessClient(srv)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}
	if err := cl.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "mymquery", Version: version}
	if _, err := cl.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	list, err := cl.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools: %w", err)
	}
	tb := &Toolbox{apiClient: apiClient, lang: lang, mcpClient: cl}
	tb.defs = append(tb.defs, llm.ToolDef{
		Name:        CorpusListToolName,
		Description: "List available corpora (ID, name, language, description and subcorpora IDs).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	})
	for _, t := range list.Tools {
		schema, err := json.Marshal(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to encode schema for tool %q: %w", t.Name, err)
		}
		tb.defs = append(tb.defs, llm.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
		})
	}
	return tb, nil
}

// Defs returns definitions of all the available tools
func (tb *Toolbox) Defs() []llm.ToolDef {
	return tb.defs
}

// Call runs a tool by its name. Failures are reported the same way as
// results (i.e. as text for the model) with ok == false.
func (tb *Toolbox) Call(ctx context.Context, name string, args map[string]any) (result string, ok bool) {
	if name == CorpusListToolName {
		return tb.corpusList(ctx)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := tb.mcpClient.CallTool(ctx, req)
	if err != nil {
		return fmt.Sprintf("tool call failed: %v", err), false
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	if res.IsError {
		return "error: " + sb.String(), false
	}
	return sb.String(), true
}

type corpusListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Language    string   `json:"language,omitempty"`
	Description string   `json:"description,omitempty"`
	Subcorpora  []string `json:"subcorpora,omitempty"`
}

func (tb *Toolbox) corpusList(ctx context.Context) (string, bool) {
	corpora, err := tb.apiClient.Corplist(ctx, tb.lang)
	if err != nil {
		return "error: " + err.Error(), false
	}
	items := make([]corpusListItem, len(corpora))
	for i, c := range corpora {
		items[i] = corpusListItem{
			ID:          c.ID,
			Name:        c.FullName,
			Language:    c.Language,
			Description: c.Description,
		}
		for _, s := range c.Subcorpora {
			items[i].Subcorpora = append(items[i].Subcorpora, s.ID)
		}
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "error: " + err.Error(), false
	}
	return string(data), true
}

// Close disconnects from the MCP server
func (tb *Toolbox) Close() error {
	return tb.mcpClient.Close()
}
