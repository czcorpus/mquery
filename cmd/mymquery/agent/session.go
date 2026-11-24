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
// ToolCallStarted, ToolCallDone, AssistantText.
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

	messages []llm.Message
	// usage is the token usage of the whole session
	usage llm.Usage
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

// Reset forgets the conversation (but the stored results are kept)
func (s *Session) Reset() {
	s.messages = nil
	s.usage = llm.Usage{}
}

// Run processes a user's message: it iterates (calling tools as requested
// by the model) until the model provides a final answer. The emit function
// is called (synchronously) with progress events. On an error (incl.
// a cancellation), the conversation is restored to its state before the
// call. Run must not be called concurrently.
func (s *Session) Run(
	ctx context.Context, userInput string, opts PromptOptions, emit func(Event),
) (TurnResult, error) {
	snapshot := len(s.messages)
	ans, err := s.run(ctx, userInput, opts, emit)
	// tokens are billed even if the turn fails
	s.usage = s.usage.Add(ans.Usage)
	if err != nil {
		s.messages = s.messages[:snapshot]
		return TurnResult{}, err
	}
	return ans, nil
}

func (s *Session) run(
	ctx context.Context, userInput string, opts PromptOptions, emit func(Event),
) (TurnResult, error) {
	s.messages = append(s.messages, llm.Message{Role: llm.RoleUser, Content: userInput})
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
	return fmt.Sprintf("result_id: %s\n%s", res.ID, content)
}
