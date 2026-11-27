package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/ui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestChatManager(t *testing.T, conf llmConf) *chatManager {
	client, err := api.NewClient("http://127.0.0.1:1", nil, time.Second)
	require.NoError(t, err)
	cm := &chatManager{
		client: client, llmConf: conf,
		userConfig: filepath.Join(t.TempDir(), "mymquery", "env"),
	}
	t.Cleanup(cm.Close)
	return cm
}

func TestChatManagerConfigure(t *testing.T) {
	cm := newTestChatManager(t, llmConf{})

	// a key is required (none is configured)
	_, _, err := cm.configure(ui.ChatSetup{Backend: llmBackendMistral})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISTRAL_API_KEY")

	sess, notice, err := cm.configure(ui.ChatSetup{Backend: llmBackendMistral, APIKey: " k1 "})
	require.NoError(t, err)
	assert.Equal(t, "mistral-medium-latest", sess.Backend().ModelName())
	assert.Empty(t, notice)
	assert.Equal(t, ui.ChatSetup{Backend: llmBackendMistral, Models: defaultLLMModels[llmBackendMistral]}, cm.current())
	_, err = os.Stat(cm.userConfig)
	assert.True(t, os.IsNotExist(err)) // not saved

	// an empty key = the one used before (here: entered in this run)
	sess, _, err = cm.configure(ui.ChatSetup{Backend: llmBackendMistral, Models: "mistral-small-latest"})
	require.NoError(t, err)
	assert.Equal(t, "mistral-small-latest", sess.Backend().ModelName())
}

func TestChatManagerConfigureSave(t *testing.T) {
	cm := newTestChatManager(t, llmConf{mistralKey: "configured"})
	_, notice, err := cm.configure(ui.ChatSetup{
		Backend: llmBackendMistral, Models: "mistral-large-latest", APIKey: "new-key", Save: true,
	})
	require.NoError(t, err)
	assert.Contains(t, notice, "saved to "+cm.userConfig)
	values, err := loadDotEnv(cm.userConfig)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"LLM_BACKEND": "mistral", "LLM_MODEL": "mistral-large-latest", "MISTRAL_API_KEY": "new-key",
	}, values)
	st, _ := os.Stat(cm.userConfig)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	// keeping the current key does not write it
	_, _, err = cm.configure(ui.ChatSetup{Backend: llmBackendMistral, Save: true})
	require.NoError(t, err)
	values, _ = loadDotEnv(cm.userConfig)
	assert.Equal(t, "new-key", values["MISTRAL_API_KEY"])
	assert.Equal(t, "", values["LLM_MODEL"])
}

func TestChatManagerConfigureOpenAI(t *testing.T) {
	cm := newTestChatManager(t, llmConf{mistralKey: "m"})
	// no key required
	sess, _, err := cm.configure(ui.ChatSetup{
		Backend: llmBackendOpenAI, Models: "qwen3", URL: " http://gpu:8080/v1 ", Save: true})
	require.NoError(t, err)
	assert.Equal(t, "qwen3", sess.Backend().ModelName())
	assert.Equal(t, ui.ChatSetup{Backend: llmBackendOpenAI, Models: "qwen3", URL: "http://gpu:8080/v1"}, cm.current())
	values, err := loadDotEnv(cm.userConfig)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"LLM_BACKEND": "openai", "LLM_MODEL": "qwen3", "OPENAI_API_URL": "http://gpu:8080/v1",
	}, values)

	// with a key
	_, _, err = cm.configure(ui.ChatSetup{Backend: llmBackendOpenAI, APIKey: "tok", Save: true})
	require.NoError(t, err)
	assert.Equal(t, "tok", cm.setup.apiKey)
	assert.Equal(t, "http://localhost:8080/v1", cm.setup.url)
	values, _ = loadDotEnv(cm.userConfig)
	assert.Equal(t, "tok", values["OPENAI_API_KEY"])

	_, _, err = cm.configure(ui.ChatSetup{Backend: llmBackendOpenAI, URL: "gpu:8080"})
	assert.Error(t, err)
}
