package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/llm"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingOnceBackend struct {
	calls int
}

func (b *failingOnceBackend) CreateCompletion(context.Context, llm.CompletionRequest) (llm.CompletionResult, error) {
	b.calls++
	if b.calls == 1 {
		return llm.CompletionResult{}, errors.New("overloaded")
	}
	return llm.CompletionResult{Content: "answer"}, nil
}

func (b *failingOnceBackend) ModelName() string  { return "m" }
func (b *failingOnceBackend) VendorName() string { return "v" }

// runChat executes the commands of a running turn until it is done
func runChat(t *testing.T, m chatModel, cmd tea.Cmd) chatModel {
	for range 100 {
		require.NotNil(t, cmd)
		var msg tea.Msg
		switch c := cmd().(type) {
		case tea.BatchMsg:
			// spinner tick + waiting for the turn; only the latter matters here
			msg = c[1]()
		default:
			msg = c
		}
		m, cmd = m.Update(msg)
		if _, done := msg.(chatDoneMsg); done {
			return m
		}
	}
	t.Fatal("chat turn did not finish")
	return m
}

func TestChatRetry(t *testing.T) {
	env := newTestEnv(t)
	backend := &failingOnceBackend{}
	env.Chat = agent.NewSession(backend, &agent.Toolbox{}, agent.NewStore())
	m := newChat(env)
	m.SetSize(80, 30)

	// ctrl+r does nothing without a failed request
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	assert.Nil(t, cmd)

	m.input.SetValue("question")
	m = runChat(t, m, m.submit("", "", false))
	assert.True(t, m.canRetry)
	assert.Contains(t, m.Help(), "ctrl+r retry")
	assert.Equal(t, entryError, m.entries[len(m.entries)-1].kind)

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	require.NotNil(t, cmd)
	assert.Equal(t, chatSubmitMsg{retry: true}, cmd())

	m = runChat(t, m, m.submit("", "", true))
	assert.False(t, m.canRetry)
	last := m.entries[len(m.entries)-1]
	assert.Equal(t, entryAssistant, last.kind)
	assert.Equal(t, "answer", last.text)
	assert.Equal(t, 2, backend.calls)
}

// blockingBackend waits until the request is cancelled
type blockingBackend struct{}

func (blockingBackend) CreateCompletion(ctx context.Context, _ llm.CompletionRequest) (llm.CompletionResult, error) {
	<-ctx.Done()
	return llm.CompletionResult{}, ctx.Err()
}

func (blockingBackend) ModelName() string  { return "m" }
func (blockingBackend) VendorName() string { return "v" }

func TestChatCancelAlwaysFinishes(t *testing.T) {
	env := newTestEnv(t)
	env.Chat = agent.NewSession(blockingBackend{}, &agent.Toolbox{}, agent.NewStore())
	m := newChat(env)
	m.SetSize(80, 30)
	// a cancelled turn must always report back (it used to be dropped randomly)
	for range 20 {
		m.input.SetValue("question")
		cmd := m.submit("", "", false)
		require.True(t, m.running)
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = runChat(t, m, cmd)
		assert.False(t, m.running)
		assert.True(t, m.canRetry)
	}
}

func TestChatStreaming(t *testing.T) {
	m := newChat(newTestEnv(t))
	m.SetSize(80, 20)
	m.running, m.runID, m.cancel = true, 1, func() {}
	ch := make(chan tea.Msg)
	event := func(e agent.Event) {
		m, _ = m.Update(chatEventMsg{runID: 1, event: e, ch: ch})
	}

	event(agent.TextStreaming{Text: "Let me"})
	assert.Contains(t, m.View(), "Let me")
	assert.Contains(t, m.View(), "writing")
	assert.Empty(t, m.entries)

	// the text turns out to be an interim one (tool calls follow)
	event(agent.TextStreaming{Text: "Let me look."})
	event(agent.AssistantText{Text: "Let me look."})
	assert.Equal(t, 1, strings.Count(m.View(), "Let me look."))

	// a retry discards a partial text
	event(agent.TextStreaming{Text: "It is"})
	event(agent.TextStreaming{Text: ""})
	assert.NotContains(t, m.View(), "It is")

	event(agent.TextStreaming{Text: "It is frequent."})
	m, _ = m.Update(chatDoneMsg{runID: 1, ans: agent.TurnResult{Text: "It is frequent."}})
	assert.Equal(t, 1, strings.Count(m.View(), "It is frequent."))
	require.Len(t, m.entries, 2)
	assert.Equal(t, entryAssistant, m.entries[1].kind)
}
