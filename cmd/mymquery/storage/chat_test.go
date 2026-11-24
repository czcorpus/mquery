package storage

import (
	"path/filepath"
	"testing"
	"time"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatSaveAndLoad(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "test.db"))
	sig := []byte{0, 1, 2, 255}
	resTime := time.Unix(1700000000, 0)
	info := agent.ChatInfo{
		Title: "How frequent is pes?", Vendor: "google", Model: "m1", CorpusID: "syn",
		Usage: llm.Usage{InputTokens: 10, CachedInputTokens: 1, OutputTokens: 5, ReasoningTokens: 2},
	}
	id, err := db.SaveChat(0, agent.ChatUpdate{
		Info: info,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "How frequent is pes?"},
			{Role: llm.RoleAssistant, Content: "let me look", Opaque: []byte("msg"), ToolCalls: []llm.ToolCall{
				{ID: "c1", Name: "freqs", Args: `{"q":"x"}`, Opaque: sig},
			}},
			{Role: llm.RoleTool, ToolResult: llm.ToolResult{ToolCallID: "c1", ToolName: "freqs", Content: "result_id: r1\n{}"}},
		},
		Results: []*agent.Result{
			{ID: "r1", Tool: "freqs", Args: map[string]any{"q": "x", "max_items": float64(5)},
				Content: "{}", OK: true, Time: resTime},
		},
	})
	require.NoError(t, err)
	assert.NotZero(t, id)

	// a failed step is replaced; a new result is added
	info.Usage.InputTokens = 30
	_, err = db.SaveChat(id, agent.ChatUpdate{
		Info:         info,
		KeepMessages: 3,
		Messages:     []llm.Message{{Role: llm.RoleAssistant, Content: "wrong"}},
		Results:      []*agent.Result{{ID: "r2", Tool: "corpus_info", Content: "error", Time: resTime}},
	})
	require.NoError(t, err)
	_, err = db.SaveChat(id, agent.ChatUpdate{
		Info:         info,
		KeepMessages: 3,
		Messages:     []llm.Message{{Role: llm.RoleAssistant, Content: "It is frequent [r1]."}},
	})
	require.NoError(t, err)

	chat, err := db.LoadChat(id)
	require.NoError(t, err)
	assert.Equal(t, id, chat.Info.ID)
	assert.Equal(t, info.Title, chat.Info.Title)
	assert.Equal(t, "google", chat.Info.Vendor)
	assert.Equal(t, info.Usage, chat.Info.Usage)
	assert.False(t, chat.Info.Created.IsZero())

	require.Len(t, chat.Messages, 4)
	assert.Equal(t, llm.RoleUser, chat.Messages[0].Role)
	assert.Equal(t, []byte("msg"), chat.Messages[1].Opaque)
	assert.Equal(t, []llm.ToolCall{{ID: "c1", Name: "freqs", Args: `{"q":"x"}`, Opaque: sig}}, chat.Messages[1].ToolCalls)
	assert.Equal(t, "c1", chat.Messages[2].ToolResult.ToolCallID)
	assert.Nil(t, chat.Messages[2].ToolCalls)
	assert.Empty(t, chat.Messages[3].Opaque)
	assert.Equal(t, "It is frequent [r1].", chat.Messages[3].Content)

	require.Len(t, chat.Results, 2)
	assert.Equal(t, &agent.Result{ID: "r1", Tool: "freqs", Args: map[string]any{"q": "x", "max_items": float64(5)},
		Content: "{}", OK: true, Time: resTime}, chat.Results[0])
	assert.Equal(t, "r2", chat.Results[1].ID)
	assert.Nil(t, chat.Results[1].Args)
	assert.False(t, chat.Results[1].OK)
}

func TestChatListAndDelete(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "test.db"))
	save := func(id int64, title string) int64 {
		id, err := db.SaveChat(id, agent.ChatUpdate{
			Info:     agent.ChatInfo{Title: title},
			Messages: []llm.Message{{Role: llm.RoleUser, Content: title}},
			Results:  []*agent.Result{{ID: "r1", Tool: "x", Time: time.Now()}},
		})
		require.NoError(t, err)
		return id
	}
	id1 := save(0, "first")
	id2 := save(0, "second")

	chats, err := db.ListChats(0)
	require.NoError(t, err)
	require.Len(t, chats, 2)
	assert.Equal(t, "second", chats[0].Title) // same second => higher ID first

	require.NoError(t, db.DeleteChat(id2))
	chats, err = db.ListChats(10)
	require.NoError(t, err)
	require.Len(t, chats, 1)
	assert.Equal(t, id1, chats[0].ID)

	_, err = db.LoadChat(id2)
	assert.ErrorIs(t, err, ErrChatNotFound)
	var n int
	require.NoError(t, db.db.QueryRow("SELECT COUNT(*) FROM chat_messages").Scan(&n))
	assert.Equal(t, 1, n, "messages are deleted along with the chat")
	require.NoError(t, db.db.QueryRow("SELECT COUNT(*) FROM chat_results").Scan(&n))
	assert.Equal(t, 1, n, "results are deleted along with the chat")

	_, err = db.SaveChat(id2, agent.ChatUpdate{})
	assert.Error(t, err, "updating a deleted chat fails")
}
