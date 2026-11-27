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
	// llmBackendOpenAI is any OpenAI-compatible server (mostly llama-server)
	llmBackendOpenAI = "openai"
)

// defaultLLMModels are comma separated lists of models per backend
// (the first one is the primary one, the others are fallbacks)
var defaultLLMModels = map[string]string{
	llmBackendGemini:  "gemini-3.5-flash-lite,gemini-3.1-flash-lite",
	llmBackendMistral: "mistral-medium-latest,mistral-small-latest",
	// (empty = the server's model, see llm.OpenAIBackend)
	llmBackendOpenAI: "",
}

// llmConf is the AI chat configuration from the environment and flags
type llmConf struct {
	// backend is one of llmBackend* ("" = the first one with an API key)
	backend string
	// models is a comma separated list of models ("" = the backend's default)
	models     string
	geminiKey  string
	mistralKey string
	// openaiURL is the OpenAI-compatible server ("" = llm.DefaultOpenAIAPIURL
	// if the backend is chosen explicitly)
	openaiURL string
	// openaiKey is the optional API key of the OpenAI-compatible server
	openaiKey string
}

// llmSetup is a resolved llmConf
type llmSetup struct {
	backend string
	models  []string
	apiKey  string
	// url is the server URL (openai only)
	url string
}

// resolve determines the backend, its models and API key. If the chat
// cannot be used (no API key), `unavailable` explains why.
func (c llmConf) resolve() (setup llmSetup, unavailable string, err error) {
	keys := map[string]string{
		llmBackendGemini: c.geminiKey, llmBackendMistral: c.mistralKey, llmBackendOpenAI: c.openaiKey}
	switch c.backend {
	case "":
		// an OpenAI-compatible server needs no key, so its URL is what selects it
		for _, b := range []string{llmBackendGemini, llmBackendMistral} {
			if keys[b] != "" {
				setup.backend = b
				break
			}
		}
		if setup.backend == "" && c.openaiURL != "" {
			setup.backend = llmBackendOpenAI
		}
		if setup.backend == "" {
			return setup, "AI chat requires GEMINI_API_KEY, MISTRAL_API_KEY or OPENAI_API_URL " +
				"(environment or the .env file)", nil
		}
	case llmBackendGemini, llmBackendMistral:
		setup.backend = c.backend
		if keys[c.backend] == "" {
			return setup, fmt.Sprintf(
				"AI chat with the %s backend requires %s (environment or the .env file)",
				c.backend, llmKeyNames[c.backend]), nil
		}
	case llmBackendOpenAI:
		setup.backend = c.backend
	default:
		return setup, "", fmt.Errorf(
			"unknown LLM backend %q (supported: %s, %s, %s)",
			c.backend, llmBackendGemini, llmBackendMistral, llmBackendOpenAI)
	}
	setup.apiKey = keys[setup.backend]
	if setup.backend == llmBackendOpenAI {
		setup.url = c.openaiURL
		if setup.url == "" {
			setup.url = llm.DefaultOpenAIAPIURL
		}
	}
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
	case llmBackendOpenAI:
		return llm.NewOpenAIBackend(setup.url, setup.apiKey, setup.models)
	}
	return nil, fmt.Errorf("unknown LLM backend %q", setup.backend)
}
