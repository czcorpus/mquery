// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Department of Linguistics,
// Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

const (
	// DefaultMistralAPIURL is the Mistral API base URL
	DefaultMistralAPIURL = "https://api.mistral.ai/v1"

	// maxMistralErrBodyLen bounds how much of an error response body ends
	// up in the returned error message.
	maxMistralErrBodyLen = 1024

	// maxMistralSSELine bounds a single line of a streamed response
	maxMistralSSELine = 4 * 1024 * 1024
)

// MistralBackend implements Backend (incl. streaming) against the Mistral
// chat completions API (/v1/chat/completions); ported from the NLQL project.
// It deliberately doesn't use any third-party SDK, nor an OpenAI client
// against Mistral's mostly-OpenAI-shaped endpoint:
//   - the available Go SDKs either can't express tool messages properly
//     (no tool_call_id/name) or are barely used,
//   - with reasoning enabled, Mistral returns the assistant message
//     content as an array of chunks (thinking + text) rather than a
//     string, and Mistral rejects unknown request fields.
//
// On a retryable error it switches to a fallback model (see modelSet).
type MistralBackend struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	models     *modelSet
}

// NewMistralBackend builds a backend for the Mistral API. The models
// must contain at least one model name; models[0] is the primary one,
// any further entries are fallbacks. An empty baseURL means DefaultMistralAPIURL.
func NewMistralBackend(baseURL, apiKey string, models []string) (*MistralBackend, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("mistral backend needs at least one model name")
	}
	if baseURL == "" {
		baseURL = DefaultMistralAPIURL
	}
	return &MistralBackend{
		// no client timeout: requests are bound by their context (and
		// a timeout would break long streamed responses)
		httpClient: &http.Client{},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
		models:     newModelSet(models),
	}, nil
}

func (b *MistralBackend) ModelName() string {
	return b.models.active()
}

func (b *MistralBackend) VendorName() string {
	return "Mistral AI"
}

// ---- wire format

type mistralFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type mistralTool struct {
	Type     string          `json:"type"`
	Function mistralFunction `json:"function"`
}

type mistralFunctionCall struct {
	Name string `json:"name"`
	// Arguments is a JSON-encoded object. Mistral documents it as
	// "object or string" - we always send a string and decode
	// both forms (see mistralToolCall.argsString).
	Arguments json.RawMessage `json:"arguments"`
}

type mistralToolCall struct {
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function mistralFunctionCall `json:"function"`
	// Index identifies a call in streamed deltas
	Index *int `json:"index,omitempty"`
}

// argsString returns the call's arguments as a raw JSON object string,
// regardless of whether Mistral sent them as an object or as a
// JSON-encoded string.
func (tc mistralToolCall) argsString() string {
	var s string
	if err := json.Unmarshal(tc.Function.Arguments, &s); err == nil {
		return s
	}
	return string(tc.Function.Arguments)
}

type mistralMessage struct {
	Role string `json:"role"`
	// Content is either a string or a raw JSON array of content chunks
	// (see mistralChunk); nil omits the field (an assistant turn that
	// only requests tool calls).
	Content    any               `json:"content,omitempty"`
	ToolCalls  []mistralToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Name       string            `json:"name,omitempty"`
}

type mistralChatRequest struct {
	Model           string           `json:"model"`
	Messages        []mistralMessage `json:"messages"`
	Tools           []mistralTool    `json:"tools,omitempty"`
	Temperature     float32          `json:"temperature"`
	ReasoningEffort string           `json:"reasoning_effort,omitempty"`
	Stream          bool             `json:"stream,omitempty"`
}

// mistralChunk is one element of a chunked message content. For
// type == "text", Text is set; for type == "thinking", Thinking holds a
// list of (text) chunks with the model's reasoning trace.
type mistralChunk struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	Thinking []mistralChunk `json:"thinking,omitempty"`
}

type mistralUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (u *mistralUsage) toUsage() Usage {
	if u == nil {
		return Usage{}
	}
	// Mistral reports neither cached nor reasoning tokens separately
	// (completion_tokens already includes reasoning)
	return Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
}

type mistralChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   json.RawMessage   `json:"content"`
			ToolCalls []mistralToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *mistralUsage `json:"usage"`
}

// mistralStreamChunk is one event of a streamed response
type mistralStreamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content   json.RawMessage   `json:"content"`
			ToolCalls []mistralToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *mistralUsage `json:"usage"`
	// Object is "error" for an error reported within a stream
	Object  string `json:"object"`
	Message any    `json:"message"`
}

// ---- requests

func (b *MistralBackend) newChatRequest(model string, req CompletionRequest, stream bool) mistralChatRequest {
	messages := make([]mistralMessage, 0, len(req.Messages)+1)
	messages = append(messages, mistralMessage{Role: "system", Content: req.SystemPrompt})
	for _, m := range req.Messages {
		messages = append(messages, toMistralMessage(m))
	}
	toolsList := make([]mistralTool, len(req.Tools))
	for i, t := range req.Tools {
		toolsList[i] = mistralTool{
			Type: "function",
			Function: mistralFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	chatReq := mistralChatRequest{
		Model:       model,
		Messages:    messages,
		Tools:       toolsList,
		Temperature: req.Temperature,
		Stream:      stream,
	}
	// reasoning_effort is only sent when thinking is requested - models
	// without reasoning support may reject the field, and for models that
	// do support it, omitting it means no thinking trace.
	if req.EnableThinking {
		chatReq.ReasoningEffort = "high"
	}
	return chatReq
}

// post sends a chat request; on success, the caller must close the response body
func (b *MistralBackend) post(ctx context.Context, chatReq mistralChatRequest) (*http.Response, error) {
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode completion request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, b.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create completion request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if chatReq.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)

	httpResp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, &CompletionError{
			Err: fmt.Errorf("completion request failed (model %q): %w", chatReq.Model, err),
		}
	}
	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, maxMistralErrBodyLen+1))
		errBody := string(respBody)
		if len(errBody) > maxMistralErrBodyLen {
			errBody = errBody[:maxMistralErrBodyLen] + "..."
		}
		retryable := isRetryableHTTPStatus(httpResp.StatusCode)
		if retryable {
			b.models.switchToFallback()
		}
		return nil, &CompletionError{
			Err: fmt.Errorf(
				"completion request failed (model %q): HTTP %d: %s",
				chatReq.Model, httpResp.StatusCode, errBody),
			ShouldRetry: retryable,
			HTTPStatus:  httpResp.StatusCode,
		}
	}
	return httpResp, nil
}

func (b *MistralBackend) CreateCompletion(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	model := b.models.current()
	httpResp, err := b.post(ctx, b.newChatRequest(model, req, false))
	if err != nil {
		return CompletionResult{}, err
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("failed to read completion response: %w", err)
	}
	var resp mistralChatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return CompletionResult{}, fmt.Errorf("failed to decode completion response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return CompletionResult{}, fmt.Errorf("no choices in response")
	}
	msg := resp.Choices[0].Message
	text, hasChunks, err := parseMistralContent(msg.Content)
	if err != nil {
		return CompletionResult{}, err
	}
	result := CompletionResult{
		Content: text.answer,
		Model:   firstNonEmpty(resp.Model, model),
		Usage:   resp.Usage.toUsage(),
	}
	// Mistral asks for the full assistant content - including thinking
	// chunks - to be replayed on the following turns (see
	// https://docs.mistral.ai/capabilities/reasoning), so keep the raw
	// chunk array as message-level opaque state.
	if hasChunks {
		result.Opaque = msg.Content
	}
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.argsString(),
		})
	}
	return result, nil
}

// StreamCompletion implements StreamingBackend (server-sent events,
// `data: <json>` lines terminated by `data: [DONE]`)
func (b *MistralBackend) StreamCompletion(
	ctx context.Context, req CompletionRequest, onText TextFunc,
) (CompletionResult, error) {
	model := b.models.current()
	httpResp, err := b.post(ctx, b.newChatRequest(model, req, true))
	if err != nil {
		return CompletionResult{}, err
	}
	defer httpResp.Body.Close()

	var acc mistralStreamAccumulator
	acc.model = model
	// an event consists of `data:` lines (joined by "\n") ended by an empty line
	var event []string
	dispatch := func() (done bool, err error) {
		data := strings.Join(event, "\n")
		event = event[:0]
		if data == "" {
			return false, nil
		}
		if data == "[DONE]" {
			return true, nil
		}
		prevLen := acc.answer.Len()
		if err := acc.add([]byte(data)); err != nil {
			return false, err
		}
		if acc.answer.Len() > prevLen {
			onText(acc.answer.String())
		}
		return false, nil
	}
	sc := bufio.NewScanner(httpResp.Body)
	sc.Buffer(make([]byte, 64*1024), maxMistralSSELine)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if done, err := dispatch(); err != nil {
				return CompletionResult{}, err

			} else if done {
				return acc.result()
			}
			continue
		}
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			event = append(event, strings.TrimPrefix(data, " "))
		}
		// other fields (event:, id:, retry:) and comments are ignored
	}
	if err := sc.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return CompletionResult{}, ctxErr
		}
		return CompletionResult{}, fmt.Errorf("failed to read streamed response: %w", err)
	}
	// the stream may end without an empty line
	if _, err := dispatch(); err != nil {
		return CompletionResult{}, err
	}
	return acc.result()
}

// mistralStreamAccumulator collects a streamed response
type mistralStreamAccumulator struct {
	model    string
	answer   strings.Builder
	thinking strings.Builder
	// hasChunks is true if the content came as chunk arrays (reasoning models)
	hasChunks bool
	// calls are tool calls by their index
	calls map[int]*mistralToolCall
	usage *mistralUsage
}

func (a *mistralStreamAccumulator) add(data []byte) error {
	var chunk mistralStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return fmt.Errorf("failed to decode a streamed response chunk: %w", err)
	}
	if chunk.Object == "error" {
		return &CompletionError{Err: fmt.Errorf("streamed completion failed: %v", chunk.Message)}
	}
	if chunk.Model != "" {
		a.model = chunk.Model
	}
	if chunk.Usage != nil {
		a.usage = chunk.Usage
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	delta := chunk.Choices[0].Delta
	text, hasChunks, err := parseMistralContent(delta.Content)
	if err != nil {
		return err
	}
	a.hasChunks = a.hasChunks || hasChunks
	a.answer.WriteString(text.answer)
	a.thinking.WriteString(text.thinking)
	for i, tc := range delta.ToolCalls {
		idx := i
		if tc.Index != nil {
			idx = *tc.Index
		}
		if a.calls == nil {
			a.calls = make(map[int]*mistralToolCall)
		}
		curr, ok := a.calls[idx]
		if !ok {
			call := tc
			a.calls[idx] = &call
			continue
		}
		// a continuation of a call (arguments may come in parts)
		if tc.ID != "" {
			curr.ID = tc.ID
		}
		if tc.Function.Name != "" {
			curr.Function.Name = tc.Function.Name
		}
		curr.Function.Arguments = appendMistralArgs(curr.Function.Arguments, tc.Function.Arguments)
	}
	return nil
}

// appendMistralArgs joins streamed parts of tool call arguments (JSON strings
// are concatenated; anything else replaces the previous value)
func appendMistralArgs(prev, next json.RawMessage) json.RawMessage {
	var p, n string
	if json.Unmarshal(prev, &p) != nil || json.Unmarshal(next, &n) != nil {
		if len(next) > 0 {
			return next
		}
		return prev
	}
	joined, _ := json.Marshal(p + n)
	return joined
}

func (a *mistralStreamAccumulator) result() (CompletionResult, error) {
	result := CompletionResult{
		Content: a.answer.String(),
		Model:   a.model,
		Usage:   a.usage.toUsage(),
	}
	if a.hasChunks {
		// the content to be replayed (see CreateCompletion) is rebuilt
		// from the streamed parts
		var chunks []mistralChunk
		if a.thinking.Len() > 0 {
			chunks = append(chunks, mistralChunk{
				Type:     "thinking",
				Thinking: []mistralChunk{{Type: "text", Text: a.thinking.String()}},
			})
		}
		if a.answer.Len() > 0 {
			chunks = append(chunks, mistralChunk{Type: "text", Text: a.answer.String()})
		}
		if len(chunks) > 0 {
			opaque, err := json.Marshal(chunks)
			if err != nil {
				return CompletionResult{}, err
			}
			result.Opaque = opaque
		}
	}
	idxs := make([]int, 0, len(a.calls))
	for idx := range a.calls {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)
	for _, idx := range idxs {
		tc := a.calls[idx]
		if tc.Function.Name == "" {
			return CompletionResult{}, errors.New("a streamed tool call has no function name")
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.argsString(),
		})
	}
	return result, nil
}

// ---- content conversion

type mistralText struct {
	answer   string
	thinking string
}

// parseMistralContent extracts the answer text (and a thinking trace) from
// a message's content, which is either a plain string, null, or an array
// of chunks. hasChunks reports whether content was the chunk array form.
func parseMistralContent(raw json.RawMessage) (text mistralText, hasChunks bool, err error) {
	if len(raw) == 0 || string(raw) == "null" {
		return text, false, nil
	}
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text.answer); err != nil {
			return text, false, fmt.Errorf("failed to decode response content: %w", err)
		}
		return text, false, nil
	}
	var chunks []mistralChunk
	if err := json.Unmarshal(raw, &chunks); err != nil {
		return text, false, fmt.Errorf("failed to decode response content chunks: %w", err)
	}
	var answer, thinking strings.Builder
	for _, ch := range chunks {
		switch ch.Type {
		case "text":
			answer.WriteString(ch.Text)
		case "thinking":
			for _, t := range ch.Thinking {
				thinking.WriteString(t.Text)
			}
		}
	}
	return mistralText{answer: answer.String(), thinking: thinking.String()}, true, nil
}

func toMistralMessage(m Message) mistralMessage {
	switch m.Role {
	case RoleTool:
		return mistralMessage{
			Role:       "tool",
			Content:    m.ToolResult.Content,
			ToolCallID: m.ToolResult.ToolCallID,
			Name:       m.ToolResult.ToolName,
		}
	case RoleAssistant:
		msg := mistralMessage{Role: "assistant"}
		if len(m.Opaque) > 0 && json.Valid(m.Opaque) {
			msg.Content = json.RawMessage(m.Opaque)

		} else if m.Content != "" {
			msg.Content = m.Content
		}
		for _, tc := range m.ToolCalls {
			args := tc.Args
			if args == "" {
				args = "{}"
			}
			encArgs, _ := json.Marshal(args) // a JSON-encoded string
			msg.ToolCalls = append(msg.ToolCalls, mistralToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: mistralFunctionCall{
					Name:      tc.Name,
					Arguments: encArgs,
				},
			})
		}
		return msg
	default:
		return mistralMessage{Role: "user", Content: m.Content}
	}
}

// firstNonEmpty returns the first non-empty string
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
