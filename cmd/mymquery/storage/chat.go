package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/llm"
)

// DB implements agent.Recorder
var _ agent.Recorder = (*DB)(nil)

// SaveChat applies an incremental update of a conversation (id 0 = a new one)
// and returns the conversation ID
func (d *DB) SaveChat(id int64, upd agent.ChatUpdate) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // no-op after a commit
	id, err = saveChat(tx, id, upd)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func saveChat(tx *sql.Tx, id int64, upd agent.ChatUpdate) (int64, error) {
	now := time.Now().Unix()
	info := upd.Info
	if id == 0 {
		res, err := tx.Exec(
			`INSERT INTO chats (created_at, updated_at, title, vendor, model, corpus_id,
				input_tokens, cached_input_tokens, output_tokens, reasoning_tokens)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			now, now, info.Title, info.Vendor, info.Model, info.CorpusID,
			info.Usage.InputTokens, info.Usage.CachedInputTokens,
			info.Usage.OutputTokens, info.Usage.ReasoningTokens,
		)
		if err != nil {
			return 0, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}

	} else {
		res, err := tx.Exec(
			`UPDATE chats SET updated_at = ?, title = ?, vendor = ?, model = ?, corpus_id = ?,
				input_tokens = ?, cached_input_tokens = ?, output_tokens = ?, reasoning_tokens = ?
			WHERE id = ?`,
			now, info.Title, info.Vendor, info.Model, info.CorpusID,
			info.Usage.InputTokens, info.Usage.CachedInputTokens,
			info.Usage.OutputTokens, info.Usage.ReasoningTokens, id,
		)
		if err != nil {
			return 0, err
		}
		if n, err := res.RowsAffected(); err != nil {
			return 0, err
		} else if n == 0 {
			return 0, fmt.Errorf("chat %d not found", id)
		}
		if _, err := tx.Exec(
			"DELETE FROM chat_messages WHERE chat_id = ? AND seq >= ?", id, upd.KeepMessages,
		); err != nil {
			return 0, err
		}
	}

	for i, m := range upd.Messages {
		var toolCalls, toolResult []byte
		var err error
		if len(m.ToolCalls) > 0 {
			if toolCalls, err = json.Marshal(m.ToolCalls); err != nil {
				return 0, err
			}
		}
		if m.Role == llm.RoleTool {
			if toolResult, err = json.Marshal(m.ToolResult); err != nil {
				return 0, err
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO chat_messages (chat_id, seq, role, content, tool_calls, tool_result, opaque)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, upd.KeepMessages+i, string(m.Role), m.Content,
			nullString(toolCalls), nullString(toolResult), m.Opaque,
		); err != nil {
			return 0, err
		}
	}

	if len(upd.Results) > 0 {
		var nextSeq int
		if err := tx.QueryRow(
			"SELECT COALESCE(MAX(seq) + 1, 0) FROM chat_results WHERE chat_id = ?", id,
		).Scan(&nextSeq); err != nil {
			return 0, err
		}
		for i, r := range upd.Results {
			var args []byte
			if r.Args != nil {
				var err error
				if args, err = json.Marshal(r.Args); err != nil {
					return 0, err
				}
			}
			if _, err := tx.Exec(
				`INSERT INTO chat_results (chat_id, seq, result_id, tool, args, content, ok, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id, nextSeq+i, r.ID, r.Tool, nullString(args), r.Content, r.OK, r.Time.Unix(),
			); err != nil {
				return 0, err
			}
		}
	}
	return id, nil
}

func nullString(data []byte) sql.NullString {
	return sql.NullString{String: string(data), Valid: data != nil}
}

const chatInfoColumns = `id, created_at, updated_at, title, vendor, model, corpus_id,
	input_tokens, cached_input_tokens, output_tokens, reasoning_tokens`

func scanChatInfo(row interface{ Scan(...any) error }) (agent.ChatInfo, error) {
	var info agent.ChatInfo
	var created, updated int64
	err := row.Scan(
		&info.ID, &created, &updated, &info.Title, &info.Vendor, &info.Model, &info.CorpusID,
		&info.Usage.InputTokens, &info.Usage.CachedInputTokens,
		&info.Usage.OutputTokens, &info.Usage.ReasoningTokens,
	)
	info.Created = time.Unix(created, 0)
	info.Updated = time.Unix(updated, 0)
	return info, err
}

// ListChats returns saved conversations, the most recently updated first
// (limit <= 0 = all)
func (d *DB) ListChats(limit int) ([]agent.ChatInfo, error) {
	if limit <= 0 {
		limit = -1 // no limit in SQLite
	}
	rows, err := d.db.Query(
		"SELECT "+chatInfoColumns+" FROM chats ORDER BY updated_at DESC, id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ans []agent.ChatInfo
	for rows.Next() {
		info, err := scanChatInfo(rows)
		if err != nil {
			return nil, err
		}
		ans = append(ans, info)
	}
	return ans, rows.Err()
}

// ChatTexts returns texts of chats for searching: the title, the user's
// messages and the model's texts (tool calls and their outputs are not
// included) joined by newlines, mapped by chat IDs
func (d *DB) ChatTexts() (map[int64]string, error) {
	texts := make(map[int64]*strings.Builder)
	add := func(query string, args ...any) error {
		rows, err := d.db.Query(query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var text string
			if err := rows.Scan(&id, &text); err != nil {
				return err
			}
			sb, ok := texts[id]
			if !ok {
				sb = &strings.Builder{}
				texts[id] = sb
			} else {
				sb.WriteString("\n")
			}
			sb.WriteString(text)
		}
		return rows.Err()
	}
	if err := add("SELECT id, title FROM chats"); err != nil {
		return nil, err
	}
	if err := add(
		`SELECT chat_id, content FROM chat_messages
		WHERE role IN (?, ?) AND content != '' ORDER BY chat_id, seq`,
		string(llm.RoleUser), string(llm.RoleAssistant),
	); err != nil {
		return nil, err
	}
	ans := make(map[int64]string, len(texts))
	for id, sb := range texts {
		ans[id] = sb.String()
	}
	return ans, nil
}

// ErrChatNotFound is returned by LoadChat for an unknown ID
var ErrChatNotFound = errors.New("chat not found")

// LoadChat loads a complete conversation
func (d *DB) LoadChat(id int64) (agent.SavedChat, error) {
	var chat agent.SavedChat
	info, err := scanChatInfo(d.db.QueryRow("SELECT "+chatInfoColumns+" FROM chats WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return chat, ErrChatNotFound
	} else if err != nil {
		return chat, err
	}
	chat.Info = info

	rows, err := d.db.Query(
		`SELECT role, content, tool_calls, tool_result, opaque FROM chat_messages
		WHERE chat_id = ? ORDER BY seq`, id)
	if err != nil {
		return chat, err
	}
	defer rows.Close()
	for rows.Next() {
		var m llm.Message
		var role string
		var toolCalls, toolResult sql.NullString
		if err := rows.Scan(&role, &m.Content, &toolCalls, &toolResult, &m.Opaque); err != nil {
			return chat, err
		}
		m.Role = llm.Role(role)
		if toolCalls.Valid {
			if err := json.Unmarshal([]byte(toolCalls.String), &m.ToolCalls); err != nil {
				return chat, fmt.Errorf("invalid tool calls of a message: %w", err)
			}
		}
		if toolResult.Valid {
			if err := json.Unmarshal([]byte(toolResult.String), &m.ToolResult); err != nil {
				return chat, fmt.Errorf("invalid tool result of a message: %w", err)
			}
		}
		chat.Messages = append(chat.Messages, m)
	}
	if err := rows.Err(); err != nil {
		return chat, err
	}
	rows.Close()

	rows, err = d.db.Query(
		`SELECT result_id, tool, args, content, ok, created_at FROM chat_results
		WHERE chat_id = ? ORDER BY seq`, id)
	if err != nil {
		return chat, err
	}
	defer rows.Close()
	for rows.Next() {
		r := &agent.Result{}
		var args sql.NullString
		var created int64
		if err := rows.Scan(&r.ID, &r.Tool, &args, &r.Content, &r.OK, &created); err != nil {
			return chat, err
		}
		r.Time = time.Unix(created, 0)
		if args.Valid {
			if err := json.Unmarshal([]byte(args.String), &r.Args); err != nil {
				return chat, fmt.Errorf("invalid arguments of result %s: %w", r.ID, err)
			}
		}
		chat.Results = append(chat.Results, r)
	}
	return chat, rows.Err()
}

// DeleteChat removes a conversation including its messages and results
func (d *DB) DeleteChat(id int64) error {
	_, err := d.db.Exec("DELETE FROM chats WHERE id = ?", id)
	return err
}
