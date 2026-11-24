package agent

import (
	"strings"
	"time"
	"unicode/utf8"

	"mquery/cmd/mymquery/llm"
)

const maxTitleChars = 80

// ChatInfo describes a saved conversation
type ChatInfo struct {
	// ID is assigned by a Recorder (0 = not saved yet)
	ID int64
	// Title is derived from the first user message
	Title  string
	Vendor string
	// Model is the last model which served the conversation
	Model string
	// CorpusID is the last corpus the user had selected (may be empty)
	CorpusID string
	Usage    llm.Usage
	// Created and Updated are set by a Recorder
	Created time.Time
	Updated time.Time
}

// ChatUpdate is an incremental change of a conversation
type ChatUpdate struct {
	Info ChatInfo
	// KeepMessages is the number of already saved messages which are
	// kept; the following ones are deleted and replaced by Messages
	KeepMessages int
	// Messages are appended after the kept ones (their indices are
	// KeepMessages, KeepMessages+1, ...)
	Messages []llm.Message
	// Results are new tool call results
	Results []*Result
}

// SavedChat is a complete saved conversation
type SavedChat struct {
	Info     ChatInfo
	Messages []llm.Message
	Results  []*Result
}

// Recorder persists conversations. It is called synchronously from
// Session.Run and Session.Retry.
type Recorder interface {
	// SaveChat applies an update to the conversation `id`; id 0 means
	// a new conversation. It returns the conversation's ID.
	SaveChat(id int64, upd ChatUpdate) (int64, error)
}

// PersistFailed is emitted when a conversation cannot be saved
// (the turn itself continues)
type PersistFailed struct {
	Err error
}

func chatTitle(userInput string) string {
	title := strings.Join(strings.Fields(userInput), " ")
	if utf8.RuneCountInString(title) > maxTitleChars {
		title = string([]rune(title)[:maxTitleChars-1]) + "…"
	}
	return title
}

// isFinalAnswer tells whether a conversation ending with the message is
// complete (i.e. not waiting for a model's response)
func isFinalAnswer(m llm.Message) bool {
	return m.Role == llm.RoleAssistant && len(m.ToolCalls) == 0
}

// persist saves changes of the conversation (if there is a Recorder)
func (s *Session) persist(emit func(Event)) {
	if s.Recorder == nil || len(s.messages) == 0 {
		return
	}
	keep := min(s.savedMsgs, len(s.messages))
	results := s.store.All()
	results = results[min(s.savedResults, len(results)):]
	id, err := s.Recorder.SaveChat(s.info.ID, ChatUpdate{
		Info:         s.info,
		KeepMessages: keep,
		Messages:     s.messages[keep:],
		Results:      results,
	})
	if err != nil {
		emit(PersistFailed{Err: err})
		return
	}
	s.info.ID = id
	s.savedMsgs = len(s.messages)
	s.savedResults += len(results)
}

// truncate drops messages from index n on
func (s *Session) truncate(n int) {
	s.messages = s.messages[:n]
	s.savedMsgs = min(s.savedMsgs, n)
}

// ChatID returns the ID of the saved conversation (0 = not saved)
func (s *Session) ChatID() int64 {
	return s.info.ID
}

// Messages returns a copy of the conversation messages
func (s *Session) Messages() []llm.Message {
	return append([]llm.Message(nil), s.messages...)
}

// Load replaces the current conversation with a saved one. If the last turn
// is incomplete (failed or interrupted), it can be continued via Retry.
// Backend-specific state is dropped if the chat was run by another vendor.
func (s *Session) Load(chat SavedChat) {
	s.Reset()
	msgs := append([]llm.Message(nil), chat.Messages...)
	if chat.Info.Vendor != s.backend.VendorName() {
		for i, m := range msgs {
			m.Opaque = nil
			if len(m.ToolCalls) > 0 {
				calls := append([]llm.ToolCall(nil), m.ToolCalls...)
				for j := range calls {
					calls[j].Opaque = nil
				}
				m.ToolCalls = calls
			}
			msgs[i] = m
		}
	}
	// tool calls without results cannot be continued (we never save such
	// a state, but better safe than sorry)
	if n := len(msgs); n > 0 && msgs[n-1].Role == llm.RoleAssistant && len(msgs[n-1].ToolCalls) > 0 {
		msgs = msgs[:n-1]
	}
	s.messages = msgs
	s.store = NewStoreFrom(chat.Results)
	s.info = chat.Info
	s.info.Vendor = s.backend.VendorName()
	s.usage = chat.Info.Usage
	s.savedMsgs = min(len(msgs), len(chat.Messages))
	s.savedResults = len(chat.Results)
	for i, m := range msgs {
		if m.Role == llm.RoleUser {
			s.turnStart = i
		}
	}
	s.checkpoint = len(msgs)
	s.failed = len(msgs) > 0 && !isFinalAnswer(msgs[len(msgs)-1])
}
