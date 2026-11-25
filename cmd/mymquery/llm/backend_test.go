package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

// fakeStreamer streams `chunks`; the first call fails (retryably)
// after the first chunk if failFirst is set
type fakeStreamer struct {
	chunks     []string
	failFirst  bool
	calls      int
	plainCalls int
}

func (b *fakeStreamer) CreateCompletion(context.Context, CompletionRequest) (CompletionResult, error) {
	b.plainCalls++
	return CompletionResult{Content: "plain"}, nil
}

func (b *fakeStreamer) StreamCompletion(_ context.Context, _ CompletionRequest, onText TextFunc) (CompletionResult, error) {
	b.calls++
	var text string
	for i, c := range b.chunks {
		text += c
		onText(text)
		if b.failFirst && b.calls == 1 && i == 0 {
			return CompletionResult{}, &CompletionError{Err: errors.New("overloaded"), ShouldRetry: true}
		}
	}
	return CompletionResult{Content: text}, nil
}

func (b *fakeStreamer) ModelName() string  { return "m" }
func (b *fakeStreamer) VendorName() string { return "v" }

func TestStreamingWithRetry(t *testing.T) {
	completionRetryBaseDelay = time.Millisecond
	t.Cleanup(func() { completionRetryBaseDelay = 2 * time.Second })

	b := &fakeStreamer{chunks: []string{"Hel", "lo"}, failFirst: true}
	var texts []string
	resp, err := CreateCompletionWithRetry(context.Background(), b, CompletionRequest{},
		func(text string) { texts = append(texts, text) })
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Content)
	// the partial text of the failed attempt is discarded via ""
	assert.Equal(t, []string{"Hel", "", "Hel", "Hello"}, texts)
	assert.Equal(t, 0, b.plainCalls)
}

func TestNoStreamingWithoutCallback(t *testing.T) {
	b := &fakeStreamer{chunks: []string{"x"}}
	resp, err := CreateCompletionWithRetry(context.Background(), b, CompletionRequest{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "plain", resp.Content)
	assert.Equal(t, 0, b.calls)
}

func TestGenAIAccumulator(t *testing.T) {
	chunk := func(usage int32, parts ...*genai.Part) *genai.GenerateContentResponse {
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{Content: &genai.Content{Parts: parts}}},
		}
		if usage > 0 {
			resp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount: 100, CandidatesTokenCount: usage}
		}
		return resp
	}
	var acc genAIAccumulator
	require.NoError(t, acc.add(chunk(0, &genai.Part{Text: "thinking", Thought: true})))
	require.NoError(t, acc.add(chunk(5, &genai.Part{Text: "Let me "})))
	require.NoError(t, acc.add(chunk(9,
		&genai.Part{Text: "check."},
		&genai.Part{
			FunctionCall:     &genai.FunctionCall{Name: "freqs", Args: map[string]any{"q": "x"}},
			ThoughtSignature: []byte("sig"),
		})))
	require.NoError(t, acc.add(&genai.GenerateContentResponse{})) // e.g. a final empty chunk

	res := acc.result("model-x")
	assert.Equal(t, "Let me check.", res.Content)
	assert.Equal(t, "model-x", res.Model)
	require.Len(t, res.ToolCalls, 1)
	assert.Equal(t, `{"q":"x"}`, res.ToolCalls[0].Args)
	assert.Equal(t, []byte("sig"), res.ToolCalls[0].Opaque)
	// the usage is cumulative => the last one counts
	assert.Equal(t, Usage{InputTokens: 100, OutputTokens: 9}, res.Usage)
}
