package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOpenAI serves prepared chat responses (one per request; a response
// starting with "data:" is sent as an event stream), lists `models` and
// records requests
type fakeOpenAI struct {
	mu         sync.Mutex
	status     []int
	responses  []string
	requests   []map[string]any
	auth       []string
	models     string
	listCalled int
}

func (f *fakeOpenAI) start(t *testing.T, apiKey string, models []string) *OpenAIBackend {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.URL.Path == "/v1/models" {
			f.listCalled++
			w.Write([]byte(f.models))
			return
		}
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		i := len(f.requests)
		f.requests = append(f.requests, req)
		if strings.HasPrefix(f.responses[i], "data:") {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		if i < len(f.status) && f.status[i] != 0 {
			w.WriteHeader(f.status[i])
		}
		w.Write([]byte(f.responses[i]))
	}))
	t.Cleanup(srv.Close)
	b, err := NewOpenAIBackend(srv.URL+"/v1/", apiKey, models)
	require.NoError(t, err)
	return b
}

func TestOpenAIRequest(t *testing.T) {
	f := &fakeOpenAI{responses: []string{
		`{"model":"qwen3","choices":[{"message":{"role":"assistant","content":"",` +
			`"tool_calls":[{"id":"c2","type":"function","function":{"name":"conc","arguments":"{\"q\":\"y\"}"}}]}}],` +
			`"usage":{"prompt_tokens":50,"completion_tokens":7,"total_tokens":57}}`,
	}}
	b := f.start(t, "secret", []string{"qwen3"})
	res, err := b.CreateCompletion(context.Background(), testConversation)
	require.NoError(t, err)
	assert.Equal(t, "qwen3", res.Model)
	assert.Equal(t, Usage{InputTokens: 50, OutputTokens: 7}, res.Usage)
	assert.Equal(t, []ToolCall{{ID: "c2", Name: "conc", Args: `{"q":"y"}`}}, res.ToolCalls)

	assert.Equal(t, []string{"Bearer secret"}, f.auth)
	req := f.requests[0]
	assert.Equal(t, "qwen3", req["model"])
	assert.Equal(t, map[string]any{"enable_thinking": false}, req["chat_template_kwargs"])
	msgs := req["messages"].([]any)
	require.Len(t, msgs, 4)
	assert.Equal(t, map[string]any{"role": "system", "content": "be helpful"}, msgs[0])
	asst := msgs[2].(map[string]any)
	assert.Equal(t, "checking", asst["content"])
	call := asst["tool_calls"].([]any)[0].(map[string]any)
	assert.Equal(t, "c1", call["id"])
	assert.Equal(t, map[string]any{"name": "freqs", "arguments": `{"q":"x"}`}, call["function"])
	tool := msgs[3].(map[string]any)
	assert.Equal(t, "tool", tool["role"])
	assert.Equal(t, "c1", tool["tool_call_id"])
	tools := req["tools"].([]any)
	require.Len(t, tools, 1)
	assert.Equal(t, "freqs", tools[0].(map[string]any)["function"].(map[string]any)["name"])
}

func TestOpenAIDiscoversModelAndNoKey(t *testing.T) {
	f := &fakeOpenAI{
		models:    `{"object":"list","data":[{"id":"gemma-3-27b.gguf","object":"model"}]}`,
		responses: []string{`{"choices":[{"message":{"content":"hi"}}]}`, `{"choices":[{"message":{"content":"hi"}}]}`},
	}
	b := f.start(t, "", nil)
	assert.Equal(t, "(the server's model)", b.ModelName())
	res, err := b.CreateCompletion(context.Background(), testConversation)
	require.NoError(t, err)
	assert.Equal(t, "hi", res.Content)
	assert.Equal(t, "gemma-3-27b.gguf", res.Model)
	assert.Equal(t, "gemma-3-27b.gguf", b.ModelName())
	assert.Equal(t, "gemma-3-27b.gguf", f.requests[0]["model"])
	_, err = b.CreateCompletion(context.Background(), testConversation)
	require.NoError(t, err)
	assert.Equal(t, 1, f.listCalled) // just once
	assert.Equal(t, []string{"", ""}, f.auth)
}

func TestOpenAIRetryableErrorSwitchesModel(t *testing.T) {
	f := &fakeOpenAI{
		status:    []int{http.StatusServiceUnavailable, http.StatusBadRequest},
		responses: []string{`{"error":{"message":"loading model"}}`, `{"error":{"message":"bad"}}`},
	}
	b := f.start(t, "secret", []string{"model-a", "model-b"})
	_, err := b.CreateCompletion(context.Background(), testConversation)
	require.Error(t, err)
	assert.True(t, IsRetryable(err))
	assert.Equal(t, "model-b", b.ModelName())

	_, err = b.CreateCompletion(context.Background(), testConversation)
	require.Error(t, err)
	assert.False(t, IsRetryable(err))
	assert.Equal(t, "model-b", f.requests[1]["model"])
}

func TestOpenAIStream(t *testing.T) {
	events := []string{
		`{"model":"qwen3","choices":[{"delta":{"role":"assistant","reasoning_content":"hmm"}}]}`,
		`{"choices":[{"delta":{"content":"It is "}}]}`,
		`{"choices":[{"delta":{"content":"frequent."}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"freqs","arguments":"{\"q\":"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"c2","type":"function","function":{"name":"conc","arguments":"{}"}}]}}]}`,
		`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}
	f := &fakeOpenAI{responses: []string{"data: " + strings.Join(events, "\n\ndata: ") + "\n\ndata: [DONE]\n\n"}}
	b := f.start(t, "secret", []string{"qwen3"})
	var texts []string
	res, err := b.StreamCompletion(context.Background(), testConversation, func(s string) { texts = append(texts, s) })
	require.NoError(t, err)
	assert.Equal(t, []string{"It is ", "It is frequent."}, texts)
	assert.Equal(t, "It is frequent.", res.Content)
	assert.Equal(t, "qwen3", res.Model)
	assert.Equal(t, Usage{InputTokens: 10, OutputTokens: 5}, res.Usage)
	assert.Equal(t, []ToolCall{
		{ID: "c1", Name: "freqs", Args: `{"q":"x"}`},
		{ID: "c2", Name: "conc", Args: `{}`},
	}, res.ToolCalls)
	req := f.requests[0]
	assert.Equal(t, true, req["stream"])
	assert.Equal(t, map[string]any{"include_usage": true}, req["stream_options"])
}

func TestOpenAIStreamHTTPError(t *testing.T) {
	f := &fakeOpenAI{status: []int{http.StatusTooManyRequests}, responses: []string{`{"error":{"message":"slow down"}}`}}
	b := f.start(t, "secret", []string{"qwen3"})
	_, err := b.StreamCompletion(context.Background(), testConversation, func(string) {})
	require.Error(t, err)
	assert.True(t, IsRetryable(err))
	assert.Contains(t, err.Error(), "slow down")
}

func TestOpenAIBackendURL(t *testing.T) {
	b, err := NewOpenAIBackend("", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "OpenAI-compatible (localhost:8080)", b.VendorName())
	b, err = NewOpenAIBackend("https://api.openai.com/v1", "k", []string{"gpt-5"})
	require.NoError(t, err)
	assert.Equal(t, "OpenAI", b.VendorName())
	assert.Nil(t, b.newChatRequest("gpt-5", testConversation).ChatTemplateKwargs)
	_, err = NewOpenAIBackend("localhost:8080", "", nil)
	assert.Error(t, err)
}
