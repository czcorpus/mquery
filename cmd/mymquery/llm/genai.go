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
	"sync"
	"time"

	"google.golang.org/genai"
)

// fallbackRecoveryInterval is how long GenAIBackend stays on a fallback
// model after a retryable error before it tries the primary model again.
const fallbackRecoveryInterval = 5 * time.Minute

// GenAIBackend implements Backend against the Gemini API via
// google.golang.org/genai. On a retryable error (e.g. a 503 from an
// overloaded model) it switches to the next model in `models` (if any)
// and it switches back to the primary model (models[0]) once
// fallbackRecoveryInterval has passed.
type GenAIBackend struct {
	client           *genai.Client
	mu               sync.Mutex
	models           []string
	activeIdx        int
	primaryDownSince time.Time
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
	return &GenAIBackend{client: client, models: models}, nil
}

func (b *GenAIBackend) ModelName() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.models[b.activeIdx]
}

func (b *GenAIBackend) VendorName() string {
	return "Google"
}

// currentModel returns the model the next request should use, switching
// back to the primary model first if fallbackRecoveryInterval has passed
// since the last fallback.
func (b *GenAIBackend) currentModel() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.activeIdx != 0 && time.Since(b.primaryDownSince) >= fallbackRecoveryInterval {
		b.activeIdx = 0
		b.primaryDownSince = time.Time{}
	}
	return b.models[b.activeIdx]
}

// switchToFallback moves to the next fallback model (cycling among
// models[1:] if there's more than one). It is a no-op if no fallback
// is configured.
func (b *GenAIBackend) switchToFallback() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.models) < 2 {
		return
	}
	if b.activeIdx == 0 {
		b.primaryDownSince = time.Now()
		b.activeIdx = 1

	} else {
		b.activeIdx = b.activeIdx%(len(b.models)-1) + 1
	}
}

func (b *GenAIBackend) CreateCompletion(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	model := b.currentModel()

	contents, err := toGenAIContents(req.Messages)
	if err != nil {
		return CompletionResult{}, err
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

	resp, err := b.client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		status := genAIErrorStatus(err)
		retryable := isRetryableHTTPStatus(status)
		if retryable {
			b.switchToFallback()
		}
		return CompletionResult{}, &CompletionError{
			Err:         fmt.Errorf("completion request failed (model %q): %w", model, err),
			ShouldRetry: retryable,
			HTTPStatus:  status,
		}
	}

	result := CompletionResult{Model: model}
	// tokens are consumed (and billed) even if the response carries
	// no content (e.g. blocked)
	if um := resp.UsageMetadata; um != nil {
		result.Usage = Usage{
			InputTokens:       int(um.PromptTokenCount),
			CachedInputTokens: int(um.CachedContentTokenCount),
			OutputTokens:      int(um.CandidatesTokenCount + um.ThoughtsTokenCount),
			ReasoningTokens:   int(um.ThoughtsTokenCount),
		}
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return result, nil
	}
	var text strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		// Gemini can return a text part alongside a FunctionCall part in
		// the same turn; thinking text is skipped as it isn't meant to be
		// shown as the answer.
		if part.Text != "" && !part.Thought {
			text.WriteString(part.Text)
		}
		if part.FunctionCall == nil {
			continue
		}
		fc := part.FunctionCall
		args, err := json.Marshal(fc.Args)
		if err != nil {
			return CompletionResult{}, fmt.Errorf("failed to encode tool call arguments for %q: %w", fc.Name, err)
		}
		// fc.ID may be empty - Gemini doesn't always assign one, and then
		// it correlates the FunctionCall/FunctionResponse pair by name and
		// turn order instead (so we must not fabricate one).
		//
		// Gemini requires the thought_signature attached to this part to
		// be echoed back verbatim on the next turn (see
		// https://ai.google.dev/gemini-api/docs/thought-signatures).
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:     fc.ID,
			Name:   fc.Name,
			Args:   string(args),
			Opaque: part.ThoughtSignature,
		})
	}
	result.Content = text.String()
	return result, nil
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
