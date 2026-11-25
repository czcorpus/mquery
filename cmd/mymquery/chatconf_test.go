package main

import (
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

	_, _, err = llmConf{backend: "openai", geminiKey: "g"}.resolve()
	assert.Error(t, err)
}
