package main

import (
	"context"
	"fmt"
	"strings"

	"mquery/cmd/mymquery/llm"
)

// supported AI chat backends (LLM_BACKEND, -llm-backend)
const (
	llmBackendGemini  = "gemini"
	llmBackendMistral = "mistral"
)

// defaultLLMModels are comma separated lists of models per backend
// (the first one is the primary one, the others are fallbacks)
var defaultLLMModels = map[string]string{
	llmBackendGemini:  "gemini-3.5-flash-lite,gemini-3.1-flash-lite",
	llmBackendMistral: "mistral-medium-latest,mistral-small-latest",
}

// llmConf is the AI chat configuration from the environment and flags
type llmConf struct {
	// backend is one of llmBackend* ("" = the first one with an API key)
	backend string
	// models is a comma separated list of models ("" = the backend's default)
	models     string
	geminiKey  string
	mistralKey string
}

// llmSetup is a resolved llmConf
type llmSetup struct {
	backend string
	models  []string
	apiKey  string
}

// resolve determines the backend, its models and API key. If the chat
// cannot be used (no API key), `unavailable` explains why.
func (c llmConf) resolve() (setup llmSetup, unavailable string, err error) {
	keys := map[string]string{llmBackendGemini: c.geminiKey, llmBackendMistral: c.mistralKey}
	keyNames := map[string]string{llmBackendGemini: "GEMINI_API_KEY", llmBackendMistral: "MISTRAL_API_KEY"}
	switch c.backend {
	case "":
		for _, b := range []string{llmBackendGemini, llmBackendMistral} {
			if keys[b] != "" {
				setup.backend = b
				break
			}
		}
		if setup.backend == "" {
			return setup, "AI chat requires GEMINI_API_KEY or MISTRAL_API_KEY (environment or the .env file)", nil
		}
	case llmBackendGemini, llmBackendMistral:
		setup.backend = c.backend
		if keys[c.backend] == "" {
			return setup, fmt.Sprintf(
				"AI chat with the %s backend requires %s (environment or the .env file)",
				c.backend, keyNames[c.backend]), nil
		}
	default:
		return setup, "", fmt.Errorf(
			"unknown LLM backend %q (supported: %s, %s)", c.backend, llmBackendGemini, llmBackendMistral)
	}
	setup.apiKey = keys[setup.backend]
	models := c.models
	if strings.TrimSpace(models) == "" {
		models = defaultLLMModels[setup.backend]
	}
	for _, m := range strings.Split(models, ",") {
		if m = strings.TrimSpace(m); m != "" {
			setup.models = append(setup.models, m)
		}
	}
	return setup, "", nil
}

// newLLMBackend creates a backend for a resolved setup
func newLLMBackend(ctx context.Context, setup llmSetup) (llm.Backend, error) {
	switch setup.backend {
	case llmBackendGemini:
		return llm.NewGenAIBackend(ctx, setup.apiKey, setup.models)
	case llmBackendMistral:
		return llm.NewMistralBackend("", setup.apiKey, setup.models)
	}
	return nil, fmt.Errorf("unknown LLM backend %q", setup.backend)
}
