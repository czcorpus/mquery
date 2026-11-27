package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMistral serves prepared responses (one per request; a response
// starting with "data:" is sent as an event stream) and records requests
type fakeMistral struct {
	status    []int
	responses []string
	requests  []map[string]any
}

func (f *fakeMistral) start(t *testing.T) *MistralBackend {
	return f.startWith(t, []string{"model-a", "model-b"})
}

func (f *fakeMistral) startWith(t *testing.T, models []string) *MistralBackend {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		i := len(f.requests)
		f.requests = append(f.requests, req)
		if i < len(f.status) && f.status[i] != 0 {
			w.WriteHeader(f.status[i])
		}
		w.Write([]byte(f.responses[i]))
	}))
	t.Cleanup(srv.Close)
	b, err := NewMistralBackend(srv.URL+"/v1/", "secret", models)
	require.NoError(t, err)
	return b
}

var testConversation = CompletionRequest{
	SystemPrompt: "be helpful",
	Messages: []Message{
		{Role: RoleUser, Content: "how frequent is pes?"},
		{Role: RoleAssistant, Content: "checking", ToolCalls: []ToolCall{
			{ID: "c1", Name: "freqs", Args: `{"q":"x"}`},
		}},
		{Role: RoleTool, ToolResult: ToolResult{ToolCallID: "c1", ToolName: "freqs", Content: "result_id: r1\n{}"}},
	},
	Tools:       []ToolDef{{Name: "freqs", Description: "frequencies", Parameters: json.RawMessage(`{"type":"object"}`)}},
	Temperature: 0.2,
}

func TestMistralRequest(t *testing.T) {
	f := &fakeMistral{responses: []string{
		`{"model":"model-a","choices":[{"message":{"content":"It is frequent [r1]."}}],"usage":{"prompt_tokens":50,"completion_tokens":7}}`,
	}}
	b := f.start(t)
	res, err := b.CreateCompletion(context.Background(), testConversation)
	require.NoError(t, err)
	assert.Equal(t, "It is frequent [r1].", res.Content)
	assert.Equal(t, "model-a", res.Model)
	assert.Equal(t, Usage{InputTokens: 50, OutputTokens: 7}, res.Usage)
	assert.Nil(t, res.Opaque)

	req := f.requests[0]
	assert.Equal(t, "model-a", req["model"])
	assert.Nil(t, req["stream"])
	assert.Nil(t, req["reasoning_effort"])
	msgs := req["messages"].([]any)
	require.Len(t, msgs, 4)
	assert.Equal(t, map[string]any{"role": "system", "content": "be helpful"}, msgs[0])
	asst := msgs[2].(map[string]any)
	call := asst["tool_calls"].([]any)[0].(map[string]any)
	assert.Equal(t, "c1", call["id"])
	assert.Equal(t, `{"q":"x"}`, call["function"].(map[string]any)["arguments"])
	assert.Equal(t, map[string]any{
		"role": "tool", "content": "result_id: r1\n{}", "tool_call_id": "c1", "name": "freqs",
	}, msgs[3])
	tool := req["tools"].([]any)[0].(map[string]any)
	assert.Equal(t, "function", tool["type"])
}

func TestMistralToolCallsAndChunks(t *testing.T) {
	f := &fakeMistral{responses: []string{
		`{"choices":[{"message":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"hmm"}]},{"type":"text","text":"Let me look."}],
		  "tool_calls":[{"id":"c2","function":{"name":"freqs","arguments":{"q":"y"}}}]}}]}`,
	}}
	b := f.start(t)
	res, err := b.CreateCompletion(context.Background(), CompletionRequest{EnableThinking: true})
	require.NoError(t, err)
	assert.Equal(t, "Let me look.", res.Content)
	assert.Equal(t, "model-a", res.Model) // not in the response
	require.Len(t, res.ToolCalls, 1)
	assert.Equal(t, ToolCall{ID: "c2", Name: "freqs", Args: `{"q":"y"}`}, res.ToolCalls[0])
	// the chunks are kept and replayed verbatim
	require.NotNil(t, res.Opaque)
	assert.Equal(t, "high", f.requests[0]["reasoning_effort"])
	msg := toMistralMessage(Message{Role: RoleAssistant, Content: res.Content, Opaque: res.Opaque})
	assert.Equal(t, json.RawMessage(res.Opaque), msg.Content)
}

func TestMistralRetryableError(t *testing.T) {
	f := &fakeMistral{
		status:    []int{http.StatusTooManyRequests, http.StatusBadRequest},
		responses: []string{`{"message":"rate limited"}`, `{"message":"invalid model"}`},
	}
	b := f.start(t)
	_, err := b.CreateCompletion(context.Background(), CompletionRequest{})
	require.Error(t, err)
	assert.True(t, IsRetryable(err))
	assert.Contains(t, err.Error(), "rate limited")
	assert.Equal(t, "model-b", b.ModelName()) // fallback

	_, err = b.CreateCompletion(context.Background(), CompletionRequest{})
	require.Error(t, err)
	assert.False(t, IsRetryable(err))
	assert.Equal(t, "model-b", f.requests[1]["model"])
}

func sseStream(events ...string) string {
	var sb strings.Builder
	for _, e := range events {
		// (multi-line events test joining of `data:` lines)
		for _, line := range strings.Split(e, "\n") {
			sb.WriteString("data: " + line + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

func TestMistralStreaming(t *testing.T) {
	f := &fakeMistral{responses: []string{sseStream(
		`{"model":"model-a-2511","choices":[{"delta":{"role":"assistant","content":""}}]}`,
		`{"choices":[{"delta":{"content":"Let me "}}]}`,
		`{"choices":[{"delta":{"content":"look."}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"id":"c1","index":0,"function":{"name":"freqs","arguments":"{\"q\":"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"","arguments":"\"x\"}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"id":"c2","index":1,"function":{"name":"corpus_list","arguments":"{}"}}]}}],
		  "usage":{"prompt_tokens":10,"completion_tokens":4}}`,
	)}}
	b := f.start(t)
	var texts []string
	res, err := b.StreamCompletion(context.Background(), testConversation, func(s string) {
		texts = append(texts, s)
	})
	require.NoError(t, err)
	assert.Equal(t, true, f.requests[0]["stream"])
	assert.Equal(t, []string{"Let me ", "Let me look."}, texts)
	assert.Equal(t, "Let me look.", res.Content)
	assert.Equal(t, "model-a-2511", res.Model)
	assert.Equal(t, Usage{InputTokens: 10, OutputTokens: 4}, res.Usage)
	assert.Equal(t, []ToolCall{
		{ID: "c1", Name: "freqs", Args: `{"q":"x"}`},
		{ID: "c2", Name: "corpus_list", Args: `{}`},
	}, res.ToolCalls)
	assert.Nil(t, res.Opaque)
}

func TestMistralStreamingChunks(t *testing.T) {
	f := &fakeMistral{responses: []string{sseStream(
		`{"choices":[{"delta":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"let me "}]}]}}]}`,
		`{"choices":[{"delta":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"think"}]}]}}]}`,
		`{"choices":[{"delta":{"content":[{"type":"text","text":"Answer."}]}}]}`,
	)}}
	b := f.start(t)
	var texts []string
	res, err := b.StreamCompletion(context.Background(), CompletionRequest{}, func(s string) {
		texts = append(texts, s)
	})
	require.NoError(t, err)
	// the thinking is not shown
	assert.Equal(t, []string{"Answer."}, texts)
	assert.Equal(t, "Answer.", res.Content)
	var chunks []mistralChunk
	require.NoError(t, json.Unmarshal(res.Opaque, &chunks))
	assert.Equal(t, []mistralChunk{
		{Type: "thinking", Thinking: []mistralChunk{{Type: "text", Text: "let me think"}}},
		{Type: "text", Text: "Answer."},
	}, chunks)
}

func TestMistralStreamError(t *testing.T) {
	f := &fakeMistral{
		status:    []int{http.StatusServiceUnavailable, 0},
		responses: []string{`{"message":"overloaded"}`, sseStream(`{"object":"error","message":"internal"}`)},
	}
	b := f.startWith(t, []string{"model-a"})
	_, err := b.StreamCompletion(context.Background(), CompletionRequest{}, func(string) {})
	require.Error(t, err)
	assert.True(t, IsRetryable(err))

	_, err = b.StreamCompletion(context.Background(), CompletionRequest{}, func(string) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal")
}
