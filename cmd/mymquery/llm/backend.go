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

// Package llm is a provider-neutral chat completion + tool calling
// interface (ported from the NLQL project). Note that the package must
// not log anything to stdout/stderr as it runs within a terminal UI.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Role identifies who authored a Message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn of a backend-neutral conversation history.
// Depending on Role, only a subset of the fields is meaningful:
//   - RoleUser: Content
//   - RoleAssistant: Content and/or ToolCalls (a turn that only requests
//     tool calls has empty Content)
//   - RoleTool: ToolResult
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolResult ToolResult
	// Opaque carries backend-specific, message-level state of
	// a RoleAssistant turn that must be echoed back verbatim on the next
	// turn. It's copied from CompletionResult.Opaque. It is a byte slice
	// so conversations can be persisted (backends needing a structured
	// state must encode it).
	Opaque []byte
}

// ToolCall is one function/tool invocation requested by the model.
type ToolCall struct {
	// ID refers to the ToolResult answering the call (may be empty
	// for backends correlating calls and results by order, e.g. Gemini)
	ID   string
	Name string
	// Args holds the raw JSON arguments object, as produced by the model.
	Args string
	// Opaque carries backend-specific state that must be echoed back
	// verbatim alongside this call on the next turn (e.g. Gemini's
	// thought_signature, required for tool-calling to work correctly).
	Opaque []byte
}

// ToolResult is the outcome of running a ToolCall, fed back to the model
// as a RoleTool Message on the next turn.
type ToolResult struct {
	ToolCallID string
	ToolName   string
	Content    string
}

// ToolDef describes one callable tool in a backend-neutral form.
type ToolDef struct {
	Name        string
	Description string
	// Parameters is the tool's input JSON schema
	Parameters json.RawMessage
}

// CompletionRequest is one turn's worth of input to a Backend.
type CompletionRequest struct {
	SystemPrompt   string
	Messages       []Message
	Tools          []ToolDef
	Temperature    float32
	EnableThinking bool
}

// Usage is the token usage of a single completion, normalized across vendors.
type Usage struct {
	// InputTokens is the whole prompt, including its cached part.
	InputTokens int
	// CachedInputTokens is the subset of InputTokens served from a cache.
	CachedInputTokens int
	// OutputTokens is everything billed as output, including reasoning.
	OutputTokens int
	// ReasoningTokens is the subset of OutputTokens spent on thinking.
	ReasoningTokens int
}

// Add returns the sum of two usages
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:       u.InputTokens + other.InputTokens,
		CachedInputTokens: u.CachedInputTokens + other.CachedInputTokens,
		OutputTokens:      u.OutputTokens + other.OutputTokens,
		ReasoningTokens:   u.ReasoningTokens + other.ReasoningTokens,
	}
}

// CompletionResult is what a Backend returns for one turn: either final
// text, or a set of tool calls the caller must run and feed back as
// RoleTool Messages before calling CreateCompletion again.
type CompletionResult struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
	// Model is the model that actually served this completion
	// (it may differ from the primary one in case of a fallback).
	Model string
	// Opaque is backend-specific, message-level state of this turn the
	// caller must store as Message.Opaque of the matching RoleAssistant
	// message when continuing the conversation.
	Opaque []byte
}

// CompletionError is the error a Backend returns from CreateCompletion
// when the underlying LLM call fails.
type CompletionError struct {
	Err error
	// ShouldRetry distinguishes transient failures worth retrying (e.g.
	// a 503 from an overloaded backend, a rate limit) from ones that will
	// fail again on retry (bad request, auth failure, ...).
	ShouldRetry bool
	// HTTPStatus is the status code of the failed LLM API call, or zero
	// if the call failed before getting an HTTP response.
	HTTPStatus int
}

func (e *CompletionError) Error() string {
	return e.Err.Error()
}

func (e *CompletionError) Unwrap() error {
	return e.Err
}

// IsRetryable reports whether err is (or wraps) a CompletionError
// marked as retryable.
func IsRetryable(err error) bool {
	var cerr *CompletionError
	return errors.As(err, &cerr) && cerr.ShouldRetry
}

func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// Backend abstracts one LLM provider's chat completion + tool calling API.
type Backend interface {
	CreateCompletion(ctx context.Context, req CompletionRequest) (CompletionResult, error)
	// ModelName is the name of the (currently active) model
	ModelName() string
	VendorName() string
}

const (
	// maxCompletionRetries bounds how many times a single CreateCompletion
	// call is retried after a retryable error
	maxCompletionRetries = 3
	// completionRetryBaseDelay is the delay before the first retry;
	// subsequent retries back off exponentially.
	completionRetryBaseDelay = 2 * time.Second
)

// CreateCompletionWithRetry calls the backend's CreateCompletion, retrying
// with exponential backoff on errors the backend marked as retryable.
func CreateCompletionWithRetry(
	ctx context.Context, backend Backend, req CompletionRequest,
) (CompletionResult, error) {
	for attempt := 0; ; attempt++ {
		resp, err := backend.CreateCompletion(ctx, req)
		if err == nil || !IsRetryable(err) || attempt >= maxCompletionRetries {
			return resp, err
		}
		delay := completionRetryBaseDelay * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(delay):
		}
	}
}
