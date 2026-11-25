package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedBackend returns prepared responses (one per call) and records requests
type scriptedBackend struct {
	responses []llm.CompletionResult
	errs      []error
	requests  []llm.CompletionRequest
}

func (b *scriptedBackend) CreateCompletion(_ context.Context, req llm.CompletionRequest) (llm.CompletionResult, error) {
	i := len(b.requests)
	// messages slice is reused by the session, so we must copy it
	req.Messages = append([]llm.Message(nil), req.Messages...)
	b.requests = append(b.requests, req)
	if i < len(b.errs) && b.errs[i] != nil {
		return llm.CompletionResult{}, b.errs[i]
	}
	return b.responses[i], nil
}

func (b *scriptedBackend) ModelName() string  { return "test-model" }
func (b *scriptedBackend) VendorName() string { return "test" }

func newTestToolbox(t *testing.T) (*Toolbox, *[]string) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/freqs/syn":
			w.Write([]byte(`{"concSize":10,"freqs":[{"word":"pes","freq":10}]}`))
		case "/corplist":
			w.Write([]byte(`{"corpora":[{"id":"syn","fullName":"SYN","subcorpora":[{"id":"sub1"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(srv.URL, map[string]string{"X-Test": "1"}, 5*time.Second)
	require.NoError(t, err)
	tb, err := NewToolbox(context.Background(), client, "", "test")
	require.NoError(t, err)
	t.Cleanup(func() { tb.Close() })
	return tb, &paths
}

func TestSessionToolLoop(t *testing.T) {
	tb, paths := newTestToolbox(t)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{
				Content: "Let me check.",
				ToolCalls: []llm.ToolCall{
					{Name: "freqs", Args: `{"corpus_id":"syn","q":"[lemma=\"pes\"]","attr":"word"}`},
					{Name: CorpusListToolName, Args: `{}`},
				},
				Usage: llm.Usage{InputTokens: 100, OutputTokens: 10},
			},
			{Content: "It is frequent [r1].", Model: "test-model", Usage: llm.Usage{InputTokens: 200, OutputTokens: 5}},
		},
	}
	sess := NewSession(backend, tb, NewStore())
	var events []Event
	ans, err := sess.Run(context.Background(), "How frequent is pes?", PromptOptions{CorpusID: "syn"},
		func(e Event) { events = append(events, e) })
	require.NoError(t, err)

	assert.Equal(t, "It is frequent [r1].", ans.Text)
	assert.Equal(t, llm.Usage{InputTokens: 300, OutputTokens: 15}, ans.Usage)
	assert.Equal(t, ans.Usage, sess.Usage())
	assert.Contains(t, backend.requests[0].SystemPrompt, "`syn`")

	results := sess.Store().All()
	require.Len(t, results, 2)
	assert.Equal(t, "r1", results[0].ID)
	assert.Equal(t, "freqs", results[0].Tool)
	assert.True(t, results[0].OK)
	assert.Equal(t, "word", results[0].StringArg("attr"))
	assert.Contains(t, results[0].Content, `"pes"`)
	assert.Equal(t, "r2", results[1].ID)
	assert.Contains(t, results[1].Content, `"subcorpora":["sub1"]`)
	require.Len(t, *paths, 2)
	assert.Contains(t, (*paths)[0], "attr=word")

	// the second request carries tool results with IDs
	msgs := backend.requests[1].Messages
	require.Len(t, msgs, 4)
	assert.Equal(t, llm.RoleTool, msgs[2].Role)
	assert.True(t, strings.HasPrefix(msgs[2].ToolResult.Content, "result_id: r1\n"))
	assert.True(t, strings.HasPrefix(msgs[3].ToolResult.Content, "result_id: r2\n"))

	require.Len(t, events, 5)
	assert.Equal(t, AssistantText{Text: "Let me check."}, events[0])
	assert.Equal(t, "freqs", events[1].(ToolCallStarted).Tool)
	assert.Equal(t, "r1", events[2].(ToolCallDone).Result.ID)
}

func TestSessionToolError(t *testing.T) {
	tb, _ := newTestToolbox(t)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{ToolCalls: []llm.ToolCall{{Name: "collocations", Args: `{"corpus_id":"syn","q":"x"}`}}},
			{Content: "failed"},
		},
	}
	sess := NewSession(backend, tb, NewStore())
	_, err := sess.Run(context.Background(), "q", PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	res, ok := sess.Store().Get("r1")
	require.True(t, ok)
	assert.False(t, res.OK)
	assert.Contains(t, backend.requests[1].Messages[2].ToolResult.Content, "not found")
}

func TestSessionRetry(t *testing.T) {
	tb, paths := newTestToolbox(t)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{Content: "hello", Usage: llm.Usage{InputTokens: 1}},
			{ToolCalls: []llm.ToolCall{{Name: "freqs", Args: `{"corpus_id":"syn","q":"x"}`}}, Usage: llm.Usage{InputTokens: 10}},
			{},
			{Content: "done [r1]", Usage: llm.Usage{InputTokens: 100}},
		},
		errs: []error{nil, nil, errors.New("boom")},
	}
	sess := NewSession(backend, tb, NewStore())
	_, err := sess.Run(context.Background(), "hi", PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	assert.False(t, sess.CanRetry())
	_, err = sess.Retry(context.Background(), PromptOptions{}, func(Event) {})
	assert.Error(t, err)

	_, err = sess.Run(context.Background(), "frequencies", PromptOptions{}, func(Event) {})
	assert.ErrorContains(t, err, "boom")
	assert.True(t, sess.CanRetry())
	assert.Equal(t, "frequencies", sess.messages[sess.turnStart].Content)
	// progress up to the last completed step (tool results) is kept
	require.Len(t, sess.messages, 5)
	assert.Equal(t, llm.RoleTool, sess.messages[4].Role)
	assert.Len(t, sess.Store().All(), 1)
	// usage of the failed turn is counted
	assert.Equal(t, 11, sess.Usage().InputTokens)

	ans, err := sess.Retry(context.Background(), PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	assert.Equal(t, "done [r1]", ans.Text)
	assert.False(t, sess.CanRetry())
	// the tool was not called again
	assert.Len(t, *paths, 1)
	assert.Len(t, sess.Store().All(), 1)
	// the retried request continued from the tool results
	assert.Len(t, backend.requests[3].Messages, 5)
	assert.Len(t, sess.messages, 6)
}

func TestSessionNewMessageDiscardsFailedTurn(t *testing.T) {
	tb, _ := newTestToolbox(t)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{Content: "hello"},
			{},
			{Content: "second answer"},
		},
		errs: []error{nil, context.Canceled},
	}
	sess := NewSession(backend, tb, NewStore())
	_, err := sess.Run(context.Background(), "hi", PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	_, err = sess.Run(context.Background(), "failing", PromptOptions{}, func(Event) {})
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, sess.CanRetry())
	assert.Len(t, sess.messages, 3)

	_, err = sess.Run(context.Background(), "another", PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	assert.False(t, sess.CanRetry())
	msgs := backend.requests[2].Messages
	require.Len(t, msgs, 3)
	assert.Equal(t, "another", msgs[2].Content)
}

func TestContentForModelTruncates(t *testing.T) {
	sess := NewSession(&scriptedBackend{}, nil, NewStore())
	sess.MaxResultChars = 4
	res := sess.Store().Add("freqs", nil, "ab→cdefgh", true)
	content := sess.contentForModel(res)
	// "→" takes bytes 2-4, so the cut (at 4) must be moved before it
	assert.True(t, strings.HasPrefix(content, "result_id: r1\nab\n[... truncated, 2 of 11 bytes"))
}

func TestSystemPromptSubcorpus(t *testing.T) {
	p := SystemPrompt(PromptOptions{CorpusID: "syn", Subcorpus: "ab12"})
	assert.Contains(t, p, "`syn`")
	assert.Contains(t, p, "`ab12`")
	assert.NotContains(t, SystemPrompt(PromptOptions{CorpusID: "syn"}), "subcorpus `")
	// a subcorpus without a corpus makes no sense
	assert.NotContains(t, SystemPrompt(PromptOptions{Subcorpus: "ab12"}), "ab12")
}

// streamingBackend streams the scripted responses' text in two halves
type streamingBackend struct {
	scriptedBackend
}

func (b *streamingBackend) StreamCompletion(
	ctx context.Context, req llm.CompletionRequest, onText llm.TextFunc,
) (llm.CompletionResult, error) {
	resp, err := b.CreateCompletion(ctx, req)
	if err != nil {
		return resp, err
	}
	if half := len(resp.Content) / 2; half > 0 {
		onText(resp.Content[:half])
		onText(resp.Content)
	}
	return resp, nil
}

func TestSessionStreaming(t *testing.T) {
	tb, _ := newTestToolbox(t)
	backend := &streamingBackend{scriptedBackend{responses: []llm.CompletionResult{
		{Content: "Let me look.", ToolCalls: []llm.ToolCall{
			{Name: "freqs", Args: `{"corpus_id":"syn","q":"[lemma=\"pes\"]"}`},
		}},
		{Content: "It occurs 10 times [r1]."},
	}}}
	sess := NewSession(backend, tb, NewStore())
	var events []string
	ans, err := sess.Run(context.Background(), "How frequent is pes?", PromptOptions{},
		func(e Event) {
			switch e := e.(type) {
			case TextStreaming:
				events = append(events, "stream:"+e.Text)
			case AssistantText:
				events = append(events, "text:"+e.Text)
			case ToolCallStarted:
				events = append(events, "call:"+e.Tool)
			}
		})
	require.NoError(t, err)
	assert.Equal(t, "It occurs 10 times [r1].", ans.Text)
	assert.Equal(t, []string{
		"stream:Let me", "stream:Let me look.", "text:Let me look.", "call:freqs",
		"stream:It occurs 10", "stream:It occurs 10 times [r1].",
	}, events)

	// streaming can be disabled
	backend.requests = nil
	backend.responses = []llm.CompletionResult{{Content: "No stream."}}
	sess.Stream = false
	events = nil
	_, err = sess.Run(context.Background(), "q", PromptOptions{}, func(e Event) {
		if _, ok := e.(TextStreaming); ok {
			events = append(events, "stream")
		}
	})
	require.NoError(t, err)
	assert.Empty(t, events)
}
