package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/llm"
	"mquery/cmd/mymquery/storage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChatWithDB(t *testing.T) (chatModel, *storage.DB) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	env := newTestEnv(t)
	env.DB = db
	env.Chat = agent.NewSession(&failingOnceBackend{}, &agent.Toolbox{}, agent.NewStore())
	env.Chat.Recorder = db
	m := newChat(env)
	m.SetSize(100, 30)
	return m, db
}

func keyStr(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestChatEntriesFromMessages(t *testing.T) {
	store := agent.NewStoreFrom([]*agent.Result{{ID: "r1", Tool: "freqs"}, {ID: "r2", Tool: "colls"}})
	entries := chatEntriesFromMessages([]llm.Message{
		{Role: llm.RoleUser, Content: "q"},
		{Role: llm.RoleAssistant, Content: "looking", ToolCalls: []llm.ToolCall{{Name: "freqs"}, {Name: "colls"}}},
		{Role: llm.RoleTool, ToolResult: llm.ToolResult{Content: "result_id: r1\n{}"}},
		{Role: llm.RoleTool, ToolResult: llm.ToolResult{Content: "result_id: r2\n{}"}},
		{Role: llm.RoleTool, ToolResult: llm.ToolResult{Content: "no ID"}},
		{Role: llm.RoleAssistant, Content: "answer [r1]"},
	}, store)
	kinds := make([]chatEntryKind, len(entries))
	for i, e := range entries {
		kinds[i] = e.kind
	}
	assert.Equal(t, []chatEntryKind{entryUser, entryInterim, entryResult, entryResult, entryAssistant}, kinds)
	assert.Equal(t, "r2", entries[3].result.ID)
}

func TestChatListOpenAndDelete(t *testing.T) {
	m, db := newChatWithDB(t)
	// an incomplete chat (the model did not answer the tool results yet)
	id1, err := db.SaveChat(0, agent.ChatUpdate{
		Info: agent.ChatInfo{Title: "first chat", Vendor: "v", Usage: llm.Usage{InputTokens: 7}},
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "question 1"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{Name: "freqs", Args: "{}"}}},
			{Role: llm.RoleTool, ToolResult: llm.ToolResult{ToolName: "freqs", Content: "result_id: r1\n{}"}},
		},
		Results: []*agent.Result{{ID: "r1", Tool: "freqs", OK: true, Time: time.Now()}},
	})
	require.NoError(t, err)
	_, err = db.SaveChat(0, agent.ChatUpdate{
		Info:     agent.ChatInfo{Title: "second chat", Vendor: "v"},
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "q2"}, {Role: llm.RoleAssistant, Content: "a2"}},
	})
	require.NoError(t, err)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	require.NotNil(t, m.list)
	view := m.View()
	assert.Contains(t, view, "second chat")
	assert.Contains(t, view, "first chat")

	// the newest one is first; open the older one
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, m.list)
	assert.Equal(t, id1, m.env.Chat.ChatID())
	assert.True(t, m.canRetry, "the incomplete turn can be retried")
	assert.Equal(t, 7, m.env.Chat.Usage().InputTokens)
	require.Len(t, m.entries, 4) // info, user, result, retry hint
	assert.Equal(t, "question 1", m.entries[1].text)
	assert.Equal(t, "r1", m.entries[2].result.ID)

	// retrying continues the saved chat
	m = runChat(t, m, m.submit("", true)) // fails once
	m = runChat(t, m, m.submit("", true))
	assert.Equal(t, "answer", m.entries[len(m.entries)-1].text)
	saved, err := db.LoadChat(id1)
	require.NoError(t, err)
	assert.Len(t, saved.Messages, 4)

	// deleting the open chat starts a new conversation
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	require.NotNil(t, m.list)
	cur, _ := m.list.selected()
	assert.Equal(t, id1, cur.ID, "the open chat is preselected")
	m, _ = m.Update(keyStr("d"))
	assert.Contains(t, m.View(), `delete "first chat"?`)
	m, _ = m.Update(keyStr("n")) // cancelled
	assert.Len(t, m.list.items, 2)
	m, _ = m.Update(keyStr("d"))
	m, _ = m.Update(keyStr("y"))
	assert.Len(t, m.list.items, 1)
	assert.Zero(t, m.env.Chat.ChatID())
	_, err = db.LoadChat(id1)
	assert.ErrorIs(t, err, storage.ErrChatNotFound)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, m.list)
	assert.False(t, m.canRetry)
}

func TestChatListWithoutDB(t *testing.T) {
	env := newTestEnv(t)
	env.Chat = agent.NewSession(&failingOnceBackend{}, &agent.Toolbox{}, agent.NewStore())
	m := newChat(env)
	m.SetSize(100, 30)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	assert.Nil(t, m.list)
	assert.Contains(t, m.entries[len(m.entries)-1].text, "not saved")
}

func TestChatListRowKeepsDetails(t *testing.T) {
	l := newChatList(nil, 0, nil)
	row := l.renderRow(agent.ChatInfo{
		Title:    strings.Repeat("long title ", 20),
		CorpusID: "syn2020", Model: "m1", Updated: time.Now(),
	}, false, 100)
	assert.Contains(t, row, "syn2020 · m1 · 0 tokens")
	assert.LessOrEqual(t, ansi.StringWidth(row), 100)
}
