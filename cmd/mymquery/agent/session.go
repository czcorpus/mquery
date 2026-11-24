// Package agent runs a chat with an LLM which can query MQuery via
// an in-process MQuery MCP server. All tool results are kept in a Store
// so the UI can show them.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"mquery/cmd/mymquery/llm"
)

const (
	DefaultMaxIterations  = 15
	DefaultMaxResultChars = 40000
)

// Event is a progress notification of a running turn. It is one of
// ToolCallStarted, ToolCallDone, AssistantText, PersistFailed.
type Event any

// ToolCallStarted is emitted right before a tool is called
type ToolCallStarted struct {
	Tool string
	Args map[string]any
}

// ToolCallDone is emitted once a tool call finishes (also on a failure)
type ToolCallDone struct {
	Result *Result
}

// (PersistFailed is defined in persist.go)

// AssistantText is a text the model produced along with tool calls
// (i.e. not the final answer of a turn)
type AssistantText struct {
	Text string
}

// TurnResult is the final outcome of a turn
type TurnResult struct {
	Text  string
	Model string
	// Usage is the token usage of the whole turn (all iterations)
	Usage llm.Usage
}

type Session struct {
	backend        llm.Backend
	tools          *Toolbox
	store          *Store
	MaxIterations  int
	MaxResultChars int
	Temperature    float32
	EnableThinking bool
	// Recorder saves the conversation after each completed step
	// (nil = no persistence)
	Recorder Recorder

	messages []llm.Message
	// usage is the token usage of the whole session
	usage llm.Usage

	// turnStart is the index of the current/last turn's user message
	turnStart int
	// checkpoint is the length of `messages` after the last completed step
	// of the current turn (the user message, a complete set of tool results);
	// it is a consistent state the loop can continue from
	checkpoint int
	// failed is true if the last turn failed (then `messages` end at
	// the checkpoint, and the turn can be retried)
	failed bool

	// info describes the conversation for the Recorder
	info ChatInfo
	// savedMsgs and savedResults are numbers of already saved
	// messages and results
	savedMsgs    int
	savedResults int
}

func NewSession(backend llm.Backend, tools *Toolbox, store *Store) *Session {
	return &Session{
		backend:        backend,
		tools:          tools,
		store:          store,
		MaxIterations:  DefaultMaxIterations,
		MaxResultChars: DefaultMaxResultChars,
		Temperature:    0.2,
	}
}

func (s *Session) Store() *Store { return s.store }

func (s *Session) Backend() llm.Backend { return s.backend }

// Usage returns the token usage of the whole session (incl. failed turns)
func (s *Session) Usage() llm.Usage { return s.usage }

// Reset starts a new conversation (with a new result store)
func (s *Session) Reset() {
	s.messages = nil
	s.usage = llm.Usage{}
	s.turnStart, s.checkpoint, s.failed = 0, 0, false
	s.store = NewStore()
	s.info = ChatInfo{}
	s.savedMsgs, s.savedResults = 0, 0
}

// CanRetry tells whether the last turn failed and can be retried
func (s *Session) CanRetry() bool {
	return s.failed
}

// Run processes a user's message: it iterates (calling tools as requested
// by the model) until the model provides a final answer. The emit function
// is called (synchronously) with progress events.
//
// On an error (incl. a cancellation), the turn's progress is kept up to its
// last completed step so it can be continued via Retry. If Run is called
// instead, the failed turn is discarded.
//
// Run and Retry must not be called concurrently.
func (s *Session) Run(
	ctx context.Context, userInput string, opts PromptOptions, emit func(Event),
) (TurnResult, error) {
	if s.failed {
		s.truncate(s.turnStart)
		s.failed = false
	}
	s.turnStart = len(s.messages)
	s.messages = append(s.messages, llm.Message{Role: llm.RoleUser, Content: userInput})
	if s.info.Title == "" {
		s.info.Title = chatTitle(userInput)
	}
	s.info.CorpusID = opts.CorpusID
	ans, err := s.loop(ctx, opts, emit)
	return s.finishTurn(ans, err, emit)
}

// Retry continues the last (failed) turn from its last completed step
// (i.e. successful tool calls are not repeated). The turn gets a new
// iteration budget.
func (s *Session) Retry(ctx context.Context, opts PromptOptions, emit func(Event)) (TurnResult, error) {
	if !s.failed {
		return TurnResult{}, fmt.Errorf("there is no failed request to retry")
	}
	s.failed = false
	s.info.CorpusID = opts.CorpusID
	ans, err := s.loop(ctx, opts, emit)
	return s.finishTurn(ans, err, emit)
}

func (s *Session) finishTurn(ans TurnResult, err error, emit func(Event)) (TurnResult, error) {
	// tokens are billed even if the turn fails
	s.usage = s.usage.Add(ans.Usage)
	s.info.Usage = s.usage
	if err != nil {
		s.truncate(s.checkpoint)
		s.failed = true
	}
	s.persist(emit)
	if err != nil {
		return TurnResult{}, err
	}
	return ans, nil
}

// loop runs completions and tool calls until a final answer is obtained.
// It expects s.messages to end with a user message or tool results.
func (s *Session) loop(ctx context.Context, opts PromptOptions, emit func(Event)) (TurnResult, error) {
	s.info.Vendor = s.backend.VendorName()
	s.checkpoint = len(s.messages)
	s.persist(emit)
	var turn TurnResult
	for range s.MaxIterations {
		resp, err := llm.CreateCompletionWithRetry(ctx, s.backend, llm.CompletionRequest{
			SystemPrompt:   SystemPrompt(opts),
			Messages:       s.messages,
			Tools:          s.tools.Defs(),
			Temperature:    s.Temperature,
			EnableThinking: s.EnableThinking,
		})
		turn.Usage = turn.Usage.Add(resp.Usage)
		if err != nil {
			return turn, err
		}
		turn.Model = resp.Model
		if resp.Model != "" {
			s.info.Model = resp.Model
		}
		s.messages = append(s.messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
			Opaque:    resp.Opaque,
		})
		if len(resp.ToolCalls) == 0 {
			turn.Text = strings.TrimSpace(resp.Content)
			return turn, nil
		}
		if text := strings.TrimSpace(resp.Content); text != "" {
			emit(AssistantText{Text: text})
		}
		for _, call := range resp.ToolCalls {
			res := s.callTool(ctx, call, emit)
			if err := ctx.Err(); err != nil {
				return turn, err
			}
			s.messages = append(s.messages, llm.Message{
				Role: llm.RoleTool,
				ToolResult: llm.ToolResult{
					ToolCallID: call.ID,
					ToolName:   call.Name,
					Content:    s.contentForModel(res),
				},
			})
		}
		s.checkpoint = len(s.messages)
		s.persist(emit)
	}
	return turn, fmt.Errorf("max. number of iterations (%d) reached without a final answer", s.MaxIterations)
}

func (s *Session) callTool(ctx context.Context, call llm.ToolCall, emit func(Event)) *Result {
	var args map[string]any
	if call.Args != "" {
		if err := json.Unmarshal([]byte(call.Args), &args); err != nil {
			res := s.store.Add(call.Name, nil, fmt.Sprintf("error parsing arguments: %v", err), false)
			emit(ToolCallDone{Result: res})
			return res
		}
	}
	emit(ToolCallStarted{Tool: call.Name, Args: args})
	content, ok := s.tools.Call(ctx, call.Name, args)
	res := s.store.Add(call.Name, args, content, ok)
	emit(ToolCallDone{Result: res})
	return res
}

// contentForModel adds the result ID and truncates long outputs
// (the user can still see the full result in the UI)
func (s *Session) contentForModel(res *Result) string {
	content := res.Content
	if s.MaxResultChars > 0 && len(content) > s.MaxResultChars {
		cut := s.MaxResultChars
		for cut > 0 && !utf8.RuneStart(content[cut]) {
			cut--
		}
		content = content[:cut] + fmt.Sprintf(
			"\n[... truncated, %d of %d bytes shown; the user can see the full result]",
			cut, len(res.Content))
	}
	return resultIDPrefix + res.ID + "\n" + content
}

const resultIDPrefix = "result_id: "

// ResultIDOf returns the result ID a tool message content (as sent
// to the model) refers to ("" if none)
func ResultIDOf(toolContent string) string {
	rest, ok := strings.CutPrefix(toolContent, resultIDPrefix)
	if !ok {
		return ""
	}
	id, _, _ := strings.Cut(rest, "\n")
	return id
}
