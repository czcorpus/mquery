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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// GenAIBackend implements Backend against the Gemini API via
// google.golang.org/genai. On a retryable error (e.g. a 503 from an
// overloaded model) it switches to a fallback model (see modelSet).
type GenAIBackend struct {
	client *genai.Client
	models *modelSet
}

// NewGenAIBackend builds a backend for the Gemini API. The models
// must contain at least one model name; models[0] is the primary one,
// any further entries are fallbacks.
func NewGenAIBackend(ctx context.Context, apiKey string, models []string) (*GenAIBackend, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("genai backend needs at least one model name")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	return &GenAIBackend{client: client, models: newModelSet(models)}, nil
}

func (b *GenAIBackend) ModelName() string {
	return b.models.active()
}

func (b *GenAIBackend) VendorName() string {
	return "Google"
}

// prepare converts a request into the genai form
func (b *GenAIBackend) prepare(req CompletionRequest) ([]*genai.Content, *genai.GenerateContentConfig, error) {
	contents, err := toGenAIContents(req.Messages)
	if err != nil {
		return nil, nil, err
	}
	var toolList []*genai.Tool
	if len(req.Tools) > 0 {
		decls := make([]*genai.FunctionDeclaration, len(req.Tools))
		for i, t := range req.Tools {
			decls[i] = &genai.FunctionDeclaration{
				Name:                 t.Name,
				Description:          t.Description,
				ParametersJsonSchema: t.Parameters,
			}
		}
		toolList = []*genai.Tool{{FunctionDeclarations: decls}}
	}

	thinkingLevel := genai.ThinkingLevelMinimal
	if req.EnableThinking {
		thinkingLevel = genai.ThinkingLevelHigh
	}
	temperature := req.Temperature
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(req.SystemPrompt, genai.RoleUser),
		Tools:             toolList,
		Temperature:       &temperature,
		ThinkingConfig:    &genai.ThinkingConfig{ThinkingLevel: thinkingLevel},
	}
	return contents, config, nil
}

// failure creates a CompletionError for a failed genai call (and switches
// to a fallback model if the error is retryable)
func (b *GenAIBackend) failure(model string, err error) error {
	status := genAIErrorStatus(err)
	retryable := isRetryableHTTPStatus(status)
	if retryable {
		b.models.switchToFallback()
	}
	return &CompletionError{
		Err:         fmt.Errorf("completion request failed (model %q): %w", model, err),
		ShouldRetry: retryable,
		HTTPStatus:  status,
	}
}

func (b *GenAIBackend) CreateCompletion(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	model := b.models.current()
	contents, config, err := b.prepare(req)
	if err != nil {
		return CompletionResult{}, err
	}
	resp, err := b.client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return CompletionResult{}, b.failure(model, err)
	}
	var acc genAIAccumulator
	if err := acc.add(resp); err != nil {
		return CompletionResult{}, err
	}
	return acc.result(model), nil
}

// StreamCompletion implements StreamingBackend
func (b *GenAIBackend) StreamCompletion(
	ctx context.Context, req CompletionRequest, onText TextFunc,
) (CompletionResult, error) {
	model := b.models.current()
	contents, config, err := b.prepare(req)
	if err != nil {
		return CompletionResult{}, err
	}
	var acc genAIAccumulator
	for resp, err := range b.client.Models.GenerateContentStream(ctx, model, contents, config) {
		if err != nil {
			return CompletionResult{}, b.failure(model, err)
		}
		prevLen := acc.text.Len()
		if err := acc.add(resp); err != nil {
			return CompletionResult{}, err
		}
		if acc.text.Len() > prevLen {
			onText(acc.text.String())
		}
	}
	return acc.result(model), nil
}

// genAIAccumulator collects the answer from one response or from
// a sequence of streamed response chunks
type genAIAccumulator struct {
	text  strings.Builder
	calls []ToolCall
	usage *genai.GenerateContentResponseUsageMetadata
}

func (a *genAIAccumulator) add(resp *genai.GenerateContentResponse) error {
	// tokens are consumed (and billed) even if the response carries
	// no content (e.g. blocked); in a stream, the usage is cumulative
	// so the last one is kept
	if resp.UsageMetadata != nil {
		a.usage = resp.UsageMetadata
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		// Gemini can return a text part alongside a FunctionCall part in
		// the same turn; thinking text is skipped as it isn't meant to be
		// shown as the answer.
		if part.Text != "" && !part.Thought {
			a.text.WriteString(part.Text)
		}
		if part.FunctionCall == nil {
			continue
		}
		fc := part.FunctionCall
		args, err := json.Marshal(fc.Args)
		if err != nil {
			return fmt.Errorf("failed to encode tool call arguments for %q: %w", fc.Name, err)
		}
		// fc.ID may be empty - Gemini doesn't always assign one, and then
		// it correlates the FunctionCall/FunctionResponse pair by name and
		// turn order instead (so we must not fabricate one).
		//
		// Gemini requires the thought_signature attached to this part to
		// be echoed back verbatim on the next turn (see
		// https://ai.google.dev/gemini-api/docs/thought-signatures).
		a.calls = append(a.calls, ToolCall{
			ID:     fc.ID,
			Name:   fc.Name,
			Args:   string(args),
			Opaque: part.ThoughtSignature,
		})
	}
	return nil
}

func (a *genAIAccumulator) result(model string) CompletionResult {
	result := CompletionResult{Model: model, Content: a.text.String(), ToolCalls: a.calls}
	if um := a.usage; um != nil {
		result.Usage = Usage{
			InputTokens:       int(um.PromptTokenCount),
			CachedInputTokens: int(um.CachedContentTokenCount),
			OutputTokens:      int(um.CandidatesTokenCount + um.ThoughtsTokenCount),
			ReasoningTokens:   int(um.ThoughtsTokenCount),
		}
	}
	return result
}

func isFuncResponseContent(c *genai.Content) bool {
	return c.Role == string(genai.RoleUser) && len(c.Parts) > 0 && c.Parts[0].FunctionResponse != nil
}

// genAIErrorStatus returns the HTTP status code of a failed genai call, or
// zero if the call failed before getting an HTTP response.
func genAIErrorStatus(err error) int {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return 0
	}
	return apiErr.Code
}

// toGenAIContents converts a backend-neutral conversation history into
// genai's Content/Part representation. A RoleAssistant message with tool
// calls becomes a "model" Content carrying one FunctionCall part per call
// (preceded by a text part if the message has some text), and a RoleTool
// message becomes a "user" Content carrying a single FunctionResponse part.
func toGenAIContents(messages []Message) ([]*genai.Content, error) {
	contents := make([]*genai.Content, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case RoleAssistant:
			if len(m.ToolCalls) == 0 {
				contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleModel))
				continue
			}
			parts := make([]*genai.Part, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				parts = append(parts, &genai.Part{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if len(tc.Args) > 0 {
					if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
						return nil, fmt.Errorf("failed to decode tool call arguments for %q: %w", tc.Name, err)
					}
				}
				parts = append(parts, &genai.Part{
					FunctionCall:     &genai.FunctionCall{ID: tc.ID, Name: tc.Name, Args: args},
					ThoughtSignature: tc.Opaque,
				})
			}
			contents = append(contents, &genai.Content{Role: string(genai.RoleModel), Parts: parts})

		case RoleTool:
			part := &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:       m.ToolResult.ToolCallID,
					Name:     m.ToolResult.ToolName,
					Response: map[string]any{"output": m.ToolResult.Content},
				},
			}
			// responses to parallel calls go together in a single Content
			if n := len(contents); n > 0 && isFuncResponseContent(contents[n-1]) {
				contents[n-1].Parts = append(contents[n-1].Parts, part)
				continue
			}
			contents = append(contents, &genai.Content{
				Role:  string(genai.RoleUser),
				Parts: []*genai.Part{part},
			})

		default:
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleUser))
		}
	}
	return contents, nil
}
