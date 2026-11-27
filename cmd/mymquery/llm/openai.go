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
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// DefaultOpenAIAPIURL is the default URL of an OpenAI-compatible server
// (llama-server's default address)
const DefaultOpenAIAPIURL = "http://localhost:8080/v1"

// OpenAIBackend implements Backend (incl. streaming) against an
// OpenAI-compatible /v1/chat/completions endpoint - typically a local
// llama-server (llama.cpp), but also vLLM, Ollama or OpenAI itself;
// ported from the NLQL project.
//
// If no model is configured, the first model listed by the server
// (/v1/models) is used - a llama-server serves just the loaded one.
// It is looked up on the first request (so creating the backend does not
// require the server to be running). On a retryable error, it switches
// to a fallback model (see modelSet).
type OpenAIBackend struct {
	client *openai.Client
	host   string
	// sendTemplateKwargs enables chat_template_kwargs (a llama.cpp/vLLM
	// extension controlling thinking; OpenAI itself rejects it)
	sendTemplateKwargs bool

	mu     sync.Mutex
	models *modelSet // nil until discovered (if not configured)
}

// NewOpenAIBackend builds a backend for an OpenAI-compatible server.
// An empty baseURL means DefaultOpenAIAPIURL. A non-empty apiKey is sent
// as "Authorization: Bearer <apiKey>" (e.g. llama-server's --api-key);
// it may be empty for servers without authentication. models[0] is
// the primary model, any further entries are fallbacks; no models means
// the server's (first) model.
func NewOpenAIBackend(baseURL, apiKey string, models []string) (*OpenAIBackend, error) {
	if baseURL == "" {
		baseURL = DefaultOpenAIAPIURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid OpenAI-compatible server URL %q (expected e.g. %s)",
			baseURL, DefaultOpenAIAPIURL)
	}
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = strings.TrimSuffix(baseURL, "/")
	// no client timeout: requests are bound by their context (and
	// a timeout would break long streamed responses)
	b := &OpenAIBackend{
		client:             openai.NewClientWithConfig(config),
		host:               u.Host,
		sendTemplateKwargs: u.Hostname() != "api.openai.com",
	}
	if len(models) > 0 {
		b.models = newModelSet(models)
	}
	return b, nil
}

func (b *OpenAIBackend) ModelName() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.models == nil {
		return "(the server's model)"
	}
	return b.models.active()
}

func (b *OpenAIBackend) VendorName() string {
	if !b.sendTemplateKwargs {
		return "OpenAI"
	}
	return "OpenAI-compatible (" + b.host + ")"
}

// modelSet returns the models, discovering the server's model if needed
func (b *OpenAIBackend) modelSet(ctx context.Context) (*modelSet, error) {
	b.mu.Lock()
	models := b.models
	b.mu.Unlock()
	if models != nil {
		return models, nil
	}
	// (the lock is not held during the request - ModelName is called by the UI)
	list, err := b.client.ListModels(ctx)
	if err != nil {
		status := openAIErrorStatus(err)
		return nil, &CompletionError{
			Err:         fmt.Errorf("failed to get the server's model (set the model explicitly?): %w", err),
			ShouldRetry: isRetryableHTTPStatus(status),
			HTTPStatus:  status,
		}
	}
	if len(list.Models) == 0 || list.Models[0].ID == "" {
		return nil, errors.New("the server offers no model")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.models == nil {
		b.models = newModelSet([]string{list.Models[0].ID})
	}
	return b.models, nil
}

func (b *OpenAIBackend) newChatRequest(model string, req CompletionRequest) openai.ChatCompletionRequest {
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages)+1)
	messages = append(messages, openai.ChatCompletionMessage{Role: "system", Content: req.SystemPrompt})
	for _, m := range req.Messages {
		messages = append(messages, toOpenAIMessage(m))
	}
	toolsList := make([]openai.Tool, len(req.Tools))
	for i, t := range req.Tools {
		toolsList[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	chatReq := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Tools:       toolsList,
		Temperature: req.Temperature,
	}
	if b.sendTemplateKwargs {
		// reasoning models (e.g. Qwen3) think by default, so it is sent
		// in both cases
		chatReq.ChatTemplateKwargs = map[string]any{"enable_thinking": req.EnableThinking}
	}
	return chatReq
}

// completionError wraps a failed go-openai call (switching to a fallback
// model on a retryable error)
func (b *OpenAIBackend) completionError(models *modelSet, model string, err error) error {
	status := openAIErrorStatus(err)
	retryable := isRetryableHTTPStatus(status)
	if retryable {
		models.switchToFallback()
	}
	return &CompletionError{
		Err:         fmt.Errorf("completion request failed (model %q): %w", model, err),
		ShouldRetry: retryable,
		HTTPStatus:  status,
	}
}

func (b *OpenAIBackend) CreateCompletion(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	models, err := b.modelSet(ctx)
	if err != nil {
		return CompletionResult{}, err
	}
	model := models.current()
	resp, err := b.client.CreateChatCompletion(ctx, b.newChatRequest(model, req))
	if err != nil {
		return CompletionResult{}, b.completionError(models, model, err)
	}
	if len(resp.Choices) == 0 {
		return CompletionResult{}, fmt.Errorf("no choices in response")
	}
	msg := resp.Choices[0].Message
	result := CompletionResult{
		Content: msg.Content,
		Model:   firstNonEmpty(resp.Model, model),
		Usage:   toOpenAIUsage(&resp.Usage),
	}
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}
	return result, nil
}

// StreamCompletion implements StreamingBackend
func (b *OpenAIBackend) StreamCompletion(
	ctx context.Context, req CompletionRequest, onText TextFunc,
) (CompletionResult, error) {
	models, err := b.modelSet(ctx)
	if err != nil {
		return CompletionResult{}, err
	}
	model := models.current()
	chatReq := b.newChatRequest(model, req)
	chatReq.Stream = true
	chatReq.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
	stream, err := b.client.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		return CompletionResult{}, b.completionError(models, model, err)
	}
	defer stream.Close()

	var answer strings.Builder
	calls := make(map[int]*openai.ToolCall)
	result := CompletionResult{Model: model}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return CompletionResult{}, ctxErr
			}
			return CompletionResult{}, b.completionError(models, model, err)
		}
		if chunk.Model != "" {
			result.Model = chunk.Model
		}
		if chunk.Usage != nil {
			result.Usage = toOpenAIUsage(chunk.Usage)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			answer.WriteString(delta.Content)
			onText(answer.String())
		}
		for i, tc := range delta.ToolCalls {
			idx := i
			if tc.Index != nil {
				idx = *tc.Index
			}
			curr, ok := calls[idx]
			if !ok {
				call := tc
				calls[idx] = &call
				continue
			}
			// a continuation of a call (arguments come in parts)
			if tc.ID != "" {
				curr.ID = tc.ID
			}
			if tc.Function.Name != "" {
				curr.Function.Name = tc.Function.Name
			}
			curr.Function.Arguments += tc.Function.Arguments
		}
	}
	result.Content = answer.String()
	idxs := make([]int, 0, len(calls))
	for idx := range calls {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)
	for _, idx := range idxs {
		tc := calls[idx]
		if tc.Function.Name == "" {
			return CompletionResult{}, errors.New("a streamed tool call has no function name")
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}
	return result, nil
}

func toOpenAIUsage(u *openai.Usage) Usage {
	ans := Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
	// the details are optional - e.g. llama.cpp typically omits them
	if d := u.PromptTokensDetails; d != nil {
		ans.CachedInputTokens = d.CachedTokens
	}
	if d := u.CompletionTokensDetails; d != nil {
		ans.ReasoningTokens = d.ReasoningTokens
	}
	return ans
}

// openAIErrorStatus returns the HTTP status code of a failed go-openai
// call, or zero if the call failed before getting an HTTP response.
func openAIErrorStatus(err error) int {
	var apiErr *openai.APIError
	var reqErr *openai.RequestError
	switch {
	case errors.As(err, &apiErr):
		return apiErr.HTTPStatusCode
	case errors.As(err, &reqErr):
		return reqErr.HTTPStatusCode
	default:
		return 0
	}
}

func toOpenAIMessage(m Message) openai.ChatCompletionMessage {
	switch m.Role {
	case RoleTool:
		return openai.ChatCompletionMessage{
			Role:       "tool",
			Content:    m.ToolResult.Content,
			ToolCallID: m.ToolResult.ToolCallID,
		}
	case RoleAssistant:
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: m.Content}
		for _, tc := range m.ToolCalls {
			args := tc.Args
			if args == "" {
				args = "{}"
			}
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: args,
				},
			})
		}
		return msg
	default:
		return openai.ChatCompletionMessage{Role: "user", Content: m.Content}
	}
}
