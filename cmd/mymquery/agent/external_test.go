package agent

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"mquery/cmd/mymquery/llm"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startFakeCQLizer serves an MCP server (Streamable HTTP) with CQLizer-like tools
func startFakeCQLizer(t *testing.T) string {
	srv := server.NewMCPServer("cqlizer-mcp", "test")
	srv.AddTool(
		mcp.NewTool("cql_validate",
			mcp.WithDescription("Validate CQL"),
			mcp.WithString("corpus_id", mcp.Required()),
			mcp.WithString("q", mcp.Required()),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if strings.Contains(req.GetString("q", ""), "{}") {
				return mcp.NewToolResultError("failed to parse query"), nil
			}
			return mcp.NewToolResultText("query OK"), nil
		},
	)
	// not in CQLizerTools => not offered to the model
	srv.AddTool(mcp.NewTool("get_corpus_structure", mcp.WithDescription("structure")),
		func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("{}"), nil
		})
	// a name already used by MQuery => skipped
	srv.AddTool(mcp.NewTool("freqs", mcp.WithDescription("duplicate")),
		func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("wrong server"), nil
		})
	hs := httptest.NewServer(server.NewStreamableHTTPServer(srv))
	t.Cleanup(hs.Close)
	return hs.URL + "/mcp"
}

func connectFakeCQLizer(t *testing.T, tb *Toolbox) []string {
	ext, err := ConnectMCP(context.Background(), startFakeCQLizer(t), "test", nil)
	require.NoError(t, err)
	t.Cleanup(func() { ext.Close() })
	added, err := tb.AddHelperTools(context.Background(), ext, append(CQLizerTools, "freqs"))
	require.NoError(t, err)
	return added
}

func TestAddHelperTools(t *testing.T) {
	tb, _ := newTestToolbox(t)
	added := connectFakeCQLizer(t, tb)
	assert.Equal(t, []string{"cql_validate"}, added)
	assert.True(t, tb.Has("cql_validate"))
	assert.False(t, tb.Has("get_corpus_structure"))
	assert.True(t, tb.IsHelper("cql_validate"))
	assert.False(t, tb.IsHelper("freqs"))
	assert.Equal(t, []string{"cql_validate"}, tb.HelperNames())

	out, ok := tb.Call(context.Background(), "cql_validate", map[string]any{"corpus_id": "syn", "q": `[lemma="pes"]`})
	assert.True(t, ok)
	assert.Equal(t, "query OK", out)
	out, ok = tb.Call(context.Background(), "cql_validate", map[string]any{"corpus_id": "syn", "q": `[]{} 0,2`})
	assert.False(t, ok)
	assert.Equal(t, "error: failed to parse query", out)
}

func TestConnectMCPFailures(t *testing.T) {
	_, err := ConnectMCP(context.Background(), "/nonexistent/cqlizer mcp-server", "test", nil)
	assert.Error(t, err)
	_, err = ConnectMCP(context.Background(), "   ", "test", nil)
	assert.Error(t, err)
	// a process which is not an MCP server (its stderr is reported)
	_, err = ConnectMCP(context.Background(), "sh -c echo-oops-not-mcp", "test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "echo-oops-not-mcp")
}

func TestSessionHelperTools(t *testing.T) {
	tb, _ := newTestToolbox(t)
	connectFakeCQLizer(t, tb)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{ToolCalls: []llm.ToolCall{
				{ID: "c1", Name: "cql_validate", Args: `{"corpus_id":"syn","q":"[]{} 0,2"}`},
			}},
			{ToolCalls: []llm.ToolCall{
				{ID: "c2", Name: "cql_validate", Args: `{"corpus_id":"syn","q":"[lemma=\"pes\"]"}`},
				{ID: "c3", Name: "freqs", Args: `{"corpus_id":"syn","q":"[lemma=\"pes\"]","attr":"word"}`},
			}},
			{Content: "Pes is frequent [r1]."},
		},
	}
	sess := NewSession(backend, tb, NewStore())
	var helperCalls []HelperCall
	var results []*Result
	_, err := sess.Run(context.Background(), "pes?", PromptOptions{CorpusID: "syn"}, func(e Event) {
		switch e := e.(type) {
		case HelperCallDone:
			helperCalls = append(helperCalls, e.Call)
		case ToolCallDone:
			results = append(results, e.Result)
		}
	})
	require.NoError(t, err)

	// helper calls are reported, but not stored as results
	require.Len(t, helperCalls, 2)
	assert.Equal(t, HelperCall{
		Tool: "cql_validate", Args: map[string]any{"corpus_id": "syn", "q": "[]{} 0,2"},
		Content: "error: failed to parse query", OK: false,
	}, helperCalls[0])
	assert.True(t, helperCalls[1].OK)
	require.Len(t, results, 1)
	assert.Equal(t, "r1", results[0].ID) // the numbering is not affected
	assert.Len(t, sess.Store().All(), 1)

	// the model gets helper outputs without a result ID
	msgs := backend.requests[1].Messages
	assert.Equal(t, "error: failed to parse query", msgs[len(msgs)-1].ToolResult.Content)
	assert.True(t, strings.HasPrefix(backend.requests[2].Messages[len(backend.requests[2].Messages)-1].ToolResult.Content, "result_id: r1"))

	// the system prompt mentions the available helper tools only
	prompt := backend.requests[0].SystemPrompt
	assert.Contains(t, prompt, "cql_validate (syntax, attributes, tagset values)")
	assert.NotContains(t, prompt, "build_tag_condition")
}

func TestSystemPromptWithoutHelpers(t *testing.T) {
	prompt := SystemPrompt(PromptOptions{})
	assert.NotContains(t, prompt, "Helper tools")
}
