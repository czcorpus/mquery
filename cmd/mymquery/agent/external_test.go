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
func startFakeCQLizer(t *testing.T) (url string, stop func()) {
	srv := server.NewMCPServer("cqlizer-mcp", "test")
	srv.AddTool(
		mcp.NewTool("cql_validate",
			mcp.WithDescription("Validate CQL"),
			mcp.WithString("corpus_id", mcp.Required()),
			mcp.WithString("q", mcp.Required()),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			q := req.GetString("q", "")
			switch {
			case strings.Contains(q, "{}"):
				return mcp.NewToolResultError("failed to parse query"), nil
			case strings.Contains(q, "NOUN"):
				return mcp.NewToolResultError(
					"The query is syntactically valid, but some attribute values cannot match as intended:\n" +
						"- tag: NOUN is not a valid value"), nil
			case strings.Contains(q, "warn"):
				return mcp.NewToolResultText("query OK, but please check the following: tag value"), nil
			}
			return mcp.NewToolResultText("query OK"), nil
		},
	)
	srv.AddTool(
		mcp.NewTool("tagset_reference",
			mcp.WithDescription("Tagset"),
			mcp.WithString("corpus_id", mcp.Required()),
			mcp.WithString("attr", mcp.Required()),
		),
		func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("tag: positional tagset"), nil
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
	return hs.URL + "/mcp", hs.Close
}

func connectFakeCQLizer(t *testing.T, tb *Toolbox) []string {
	added, _ := connectFakeCQLizerStoppable(t, tb)
	return added
}

func connectFakeCQLizerStoppable(t *testing.T, tb *Toolbox) ([]string, func()) {
	url, stop := startFakeCQLizer(t)
	ext, err := ConnectMCP(context.Background(), url, "test", nil)
	require.NoError(t, err)
	t.Cleanup(func() { ext.Close() })
	added, err := tb.AddHelperTools(context.Background(), ext, append(CQLizerTools, "freqs"))
	require.NoError(t, err)
	return added, stop
}

func TestAddHelperTools(t *testing.T) {
	tb, _ := newTestToolbox(t)
	added := connectFakeCQLizer(t, tb)
	assert.Equal(t, []string{"cql_validate", "tagset_reference"}, added)
	assert.True(t, tb.Has("cql_validate"))
	assert.False(t, tb.Has("get_corpus_structure"))
	assert.True(t, tb.IsHelper("cql_validate"))
	assert.False(t, tb.IsHelper("freqs"))
	assert.Equal(t, []string{"cql_validate", "tagset_reference"}, tb.HelperNames())
	// cql_validate is used automatically, the model does not get it
	assert.True(t, tb.AutoValidates())
	var offered []string
	for _, d := range tb.ModelDefs() {
		offered = append(offered, d.Name)
	}
	assert.NotContains(t, offered, "cql_validate")
	assert.Contains(t, offered, "tagset_reference")

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

// runScripted runs a turn and collects helper calls and results
func runScripted(t *testing.T, tb *Toolbox, responses []llm.CompletionResult) (
	*Session, *scriptedBackend, []HelperCall, []*Result,
) {
	backend := &scriptedBackend{responses: responses}
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
	return sess, backend, helperCalls, results
}

// lastToolContent returns the last tool output sent to the model in a request
func lastToolContent(req llm.CompletionRequest) string {
	return req.Messages[len(req.Messages)-1].ToolResult.Content
}

func TestSessionAutoValidation(t *testing.T) {
	tb, paths := newTestToolbox(t)
	connectFakeCQLizer(t, tb)
	badQuery := `{"corpus_id":"syn","q":"[]{} 0,2","attr":"word"}`
	sess, backend, helperCalls, results := runScripted(t, tb, []llm.CompletionResult{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "freqs", Args: badQuery}}},
		// the model insists on the same query => it is run without validation
		{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "freqs", Args: badQuery}}},
		{ToolCalls: []llm.ToolCall{
			{ID: "c3", Name: "freqs", Args: `{"corpus_id":"syn","q":"[tag=\"warn\"]","attr":"word"}`},
		}},
		{Content: "Done [r1, r2]."},
	})

	// the rejected call is not run nor stored
	require.Len(t, helperCalls, 1)
	assert.Equal(t, "freqs", helperCalls[0].Tool)
	assert.False(t, helperCalls[0].OK)
	assert.Equal(t, "[]{} 0,2", helperCalls[0].Args["q"])
	rejection := lastToolContent(backend.requests[1])
	assert.True(t, strings.HasPrefix(rejection,
		"error: rejected by CQL validation (`q`): failed to parse query\nThe query was not run."), rejection)

	// multi-line reasons are passed to the model whole
	_, backend2, _, _ := runScripted(t, tb, []llm.CompletionResult{
		{ToolCalls: []llm.ToolCall{
			{ID: "c1", Name: "concordance", Args: `{"corpus_id":"syn","q":"[lemma=\"pes\"]","coll":"[tag=\"NOUN\"]"}`},
		}},
		{Content: "Done."},
	})
	assert.Contains(t, lastToolContent(backend2.requests[1]),
		"rejected by CQL validation (`coll`): The query is syntactically valid, but some attribute "+
			"values cannot match as intended:\n- tag: NOUN is not a valid value\nThe query was not run.")

	// the repeated and the valid query were run (incl. the warning for the model)
	require.Len(t, results, 2)
	assert.Equal(t, "r1", results[0].ID)
	assert.Len(t, sess.Store().All(), 2)
	assert.Len(t, *paths, 2)
	withWarning := lastToolContent(backend.requests[3])
	assert.True(t, strings.HasPrefix(withWarning,
		"result_id: r2\nCQL validation of `q`: query OK, but please check the following: tag value\n"), withWarning)

	// the model is told about the validation, it does not get the tool
	prompt := backend.requests[0].SystemPrompt
	assert.Contains(t, prompt, "validated automatically")
	assert.Contains(t, prompt, "tagset_reference explains")
	for _, d := range backend.requests[0].Tools {
		assert.NotEqual(t, ValidateToolName, d.Name)
	}
}

func TestSessionAutoValidationUnavailable(t *testing.T) {
	tb, paths := newTestToolbox(t)
	_, stop := connectFakeCQLizerStoppable(t, tb)
	stop() // CQLizer is gone => queries are run without validation
	_, _, helperCalls, results := runScripted(t, tb, []llm.CompletionResult{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "freqs", Args: `{"corpus_id":"syn","q":"[]{} 0,2"}`}}},
		{Content: "Done."},
	})
	assert.Empty(t, helperCalls)
	assert.Len(t, results, 1)
	assert.Len(t, *paths, 1)
}

func TestSessionHelperTool(t *testing.T) {
	tb, _ := newTestToolbox(t)
	connectFakeCQLizer(t, tb)
	sess, backend, helperCalls, results := runScripted(t, tb, []llm.CompletionResult{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "tagset_reference", Args: `{"corpus_id":"syn","attr":"tag"}`}}},
		{Content: "Done."},
	})
	assert.Equal(t, []HelperCall{{
		Tool: "tagset_reference", Args: map[string]any{"corpus_id": "syn", "attr": "tag"},
		Content: "tag: positional tagset", OK: true,
	}}, helperCalls)
	assert.Empty(t, results)
	assert.Empty(t, sess.Store().All())
	// helper outputs go to the model without a result ID
	assert.Equal(t, "tag: positional tagset", lastToolContent(backend.requests[1]))
}

func TestSystemPromptWithoutHelpers(t *testing.T) {
	prompt := SystemPrompt(PromptOptions{})
	assert.NotContains(t, prompt, "Helper tools")
}
