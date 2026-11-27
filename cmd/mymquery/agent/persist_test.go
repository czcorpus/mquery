package agent

import (
	"context"
	"errors"
	"testing"

	"mquery/cmd/mymquery/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memRecorder keeps saved chats in memory (with the semantics of storage.DB)
type memRecorder struct {
	chats map[int64]*SavedChat
	saves int
	err   error
}

func newMemRecorder() *memRecorder {
	return &memRecorder{chats: make(map[int64]*SavedChat)}
}

func (r *memRecorder) SaveChat(id int64, upd ChatUpdate) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	r.saves++
	if id == 0 {
		id = int64(len(r.chats) + 1)
		r.chats[id] = &SavedChat{}
	}
	c := r.chats[id]
	c.Info = upd.Info
	c.Info.ID = id
	c.Messages = append(c.Messages[:min(upd.KeepMessages, len(c.Messages))], upd.Messages...)
	c.Results = append(c.Results, upd.Results...)
	return id, nil
}

// load returns a deep enough copy for Session.Load
func (r *memRecorder) load(id int64) SavedChat {
	c := *r.chats[id]
	c.Messages = append([]llm.Message(nil), c.Messages...)
	c.Results = append([]*Result(nil), c.Results...)
	return c
}

func TestSessionPersistsTurns(t *testing.T) {
	tb, _ := newTestToolbox(t)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{ToolCalls: []llm.ToolCall{{Name: "freqs", Args: `{"corpus_id":"syn","q":"x"}`, Opaque: []byte("sig")}},
				Model: "m-fallback", Usage: llm.Usage{InputTokens: 10}},
			{Content: "done [r1]", Model: "m-fallback", Usage: llm.Usage{InputTokens: 20}},
		},
	}
	rec := newMemRecorder()
	sess := NewSession(backend, tb, NewStore())
	sess.Recorder = rec
	_, err := sess.Run(context.Background(), "  How   frequent\nis x?", PromptOptions{CorpusID: "syn"}, func(Event) {})
	require.NoError(t, err)

	require.NotZero(t, sess.ChatID())
	saved := rec.chats[sess.ChatID()]
	assert.Equal(t, sess.Messages(), saved.Messages)
	assert.Equal(t, sess.Store().All(), saved.Results)
	assert.Equal(t, "How frequent is x?", saved.Info.Title)
	assert.Equal(t, "syn", saved.Info.CorpusID)
	assert.Equal(t, "test", saved.Info.Vendor)
	assert.Equal(t, "m-fallback", saved.Info.Model)
	assert.Equal(t, 30, saved.Info.Usage.InputTokens)
	// user message, tool results, final answer
	assert.Equal(t, 3, rec.saves)

	// a new conversation gets a new ID and a new result store
	sess.Reset()
	assert.Zero(t, sess.ChatID())
	assert.Empty(t, sess.Store().All())
}

func TestSessionPersistsFailedTurnAndLoads(t *testing.T) {
	tb, paths := newTestToolbox(t)
	backend := &scriptedBackend{
		responses: []llm.CompletionResult{
			{ToolCalls: []llm.ToolCall{{Name: "freqs", Args: `{"corpus_id":"syn","q":"x"}`, Opaque: []byte("sig")}}},
			{},
			{Content: "done [r1]"},
			{Content: "second"},
		},
		errs: []error{nil, errors.New("boom")},
	}
	rec := newMemRecorder()
	sess := NewSession(backend, tb, NewStore())
	sess.Recorder = rec
	_, err := sess.Run(context.Background(), "q1", PromptOptions{}, func(Event) {})
	require.Error(t, err)
	id := sess.ChatID()
	require.Len(t, rec.chats[id].Messages, 3) // up to the tool results

	// "restart" the app: a new session loads the chat and retries the turn
	sess2 := NewSession(backend, tb, NewStore())
	sess2.Recorder = rec
	sess2.Load(rec.load(id))
	assert.True(t, sess2.CanRetry())
	assert.Equal(t, id, sess2.ChatID())
	ans, err := sess2.Retry(context.Background(), PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	assert.Equal(t, "done [r1]", ans.Text)
	assert.Len(t, *paths, 1, "the tool is not called again")
	// the thought signature survived (same vendor)
	assert.Equal(t, []byte("sig"), backend.requests[2].Messages[1].ToolCalls[0].Opaque)
	assert.Len(t, rec.chats[id].Messages, 4)

	// continuing the conversation; new results continue the numbering
	_, err = sess2.Run(context.Background(), "q2", PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	assert.Len(t, rec.chats, 1)
	assert.Len(t, rec.chats[id].Messages, 6)
	res := sess2.Store().Add("x", nil, "", true)
	assert.Equal(t, "r2", res.ID)
}

func TestSessionLoadOtherVendor(t *testing.T) {
	sess := NewSession(&scriptedBackend{}, nil, NewStore())
	sess.Load(SavedChat{
		Info: ChatInfo{ID: 7, Vendor: "other"},
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "q"},
			{Role: llm.RoleAssistant, Opaque: []byte("m"), ToolCalls: []llm.ToolCall{{Name: "a", Opaque: []byte("s")}}},
			{Role: llm.RoleTool, ToolResult: llm.ToolResult{ToolName: "a"}},
			{Role: llm.RoleAssistant, Content: "answer"},
		},
	})
	assert.False(t, sess.CanRetry())
	msgs := sess.Messages()
	assert.Nil(t, msgs[1].Opaque)
	assert.Nil(t, msgs[1].ToolCalls[0].Opaque)

	// an unanswered tool call is dropped (and the turn can be retried)
	sess.Load(SavedChat{
		Info: ChatInfo{Vendor: "test"},
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "q"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{Name: "a", Opaque: []byte("s")}}},
		},
	})
	assert.True(t, sess.CanRetry())
	assert.Len(t, sess.Messages(), 1)
}

func TestSessionPersistFailureDoesNotFailTurn(t *testing.T) {
	rec := newMemRecorder()
	rec.err = errors.New("disk full")
	backend := &scriptedBackend{responses: []llm.CompletionResult{{Content: "hi"}}}
	tb, _ := newTestToolbox(t)
	sess := NewSession(backend, tb, NewStore())
	sess.Recorder = rec
	var failures []PersistFailed
	ans, err := sess.Run(context.Background(), "q", PromptOptions{}, func(e Event) {
		if f, ok := e.(PersistFailed); ok {
			failures = append(failures, f)
		}
	})
	require.NoError(t, err)
	assert.Equal(t, "hi", ans.Text)
	assert.NotEmpty(t, failures)

	// once the recorder works again, everything is saved
	rec.err = nil
	backend.responses = append(backend.responses, llm.CompletionResult{Content: "hi2"})
	_, err = sess.Run(context.Background(), "q2", PromptOptions{}, func(Event) {})
	require.NoError(t, err)
	assert.Len(t, rec.chats[sess.ChatID()].Messages, 4)
}
