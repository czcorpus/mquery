package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMConfResolve(t *testing.T) {
	// no key => unavailable
	_, unavail, err := llmConf{}.resolve()
	require.NoError(t, err)
	assert.Contains(t, unavail, "MISTRAL_API_KEY")

	// the first backend with a key is used
	setup, unavail, err := llmConf{mistralKey: "m"}.resolve()
	require.NoError(t, err)
	assert.Empty(t, unavail)
	assert.Equal(t, llmSetup{
		backend: llmBackendMistral, apiKey: "m",
		models: []string{"mistral-medium-latest", "mistral-small-latest"},
	}, setup)

	setup, _, _ = llmConf{geminiKey: "g", mistralKey: "m"}.resolve()
	assert.Equal(t, llmBackendGemini, setup.backend)
	assert.Equal(t, "g", setup.apiKey)

	// explicit backend and models
	setup, _, _ = llmConf{backend: "mistral", models: " large , ", geminiKey: "g", mistralKey: "m"}.resolve()
	assert.Equal(t, llmSetup{backend: llmBackendMistral, apiKey: "m", models: []string{"large"}}, setup)

	_, unavail, err = llmConf{backend: "mistral", geminiKey: "g"}.resolve()
	require.NoError(t, err)
	assert.Contains(t, unavail, "MISTRAL_API_KEY")

	_, _, err = llmConf{backend: "openai-x", geminiKey: "g"}.resolve()
	assert.Error(t, err)
}

func TestLLMConfResolveOpenAI(t *testing.T) {
	// no key needed; the URL defaults to llama-server's, the model to the server's
	setup, unavail, err := llmConf{backend: "openai"}.resolve()
	require.NoError(t, err)
	assert.Empty(t, unavail)
	assert.Equal(t, llmSetup{backend: llmBackendOpenAI, url: "http://localhost:8080/v1"}, setup)

	setup, _, _ = llmConf{
		backend: "openai", models: "qwen3", openaiURL: "http://gpu:8000/v1", openaiKey: "k"}.resolve()
	assert.Equal(t, llmSetup{
		backend: llmBackendOpenAI, url: "http://gpu:8000/v1", apiKey: "k", models: []string{"qwen3"}}, setup)

	// auto: a key of another backend wins, otherwise the URL selects openai
	setup, _, _ = llmConf{openaiURL: "http://gpu:8000/v1", mistralKey: "m"}.resolve()
	assert.Equal(t, llmBackendMistral, setup.backend)
	setup, _, _ = llmConf{openaiURL: "http://gpu:8000/v1"}.resolve()
	assert.Equal(t, llmBackendOpenAI, setup.backend)
	// ... an API key alone does not
	_, unavail, _ = llmConf{openaiKey: "k"}.resolve()
	assert.Contains(t, unavail, "OPENAI_API_URL")

	b, err := newLLMBackend(context.Background(), setup)
	require.NoError(t, err)
	assert.Equal(t, "OpenAI-compatible (gpu:8000)", b.VendorName())
}
