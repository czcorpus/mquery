package ui

import (
	"errors"
	"testing"

	"mquery/cmd/mymquery/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typeText(m chatModel, s string) chatModel {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestChatSetupForm(t *testing.T) {
	env := newTestEnv(t)
	env.ChatBackends = []ChatBackend{
		{ID: "mistral", Label: "Mistral AI", DefaultModels: "m1,m2", KeyName: "MISTRAL_API_KEY"},
		{ID: "gemini", Label: "Google Gemini", DefaultModels: "g1", KeyName: "GEMINI_API_KEY"},
	}
	env.ChatConfigPath = "/home/u/.config/mymquery/env"
	var got []ChatSetup
	env.ChatConfigure = func(s ChatSetup) (*agent.Session, string, error) {
		got = append(got, s)
		if s.APIKey == "" {
			return nil, "", errors.New("an API key (GEMINI_API_KEY) is required")
		}
		return agent.NewSession(blockingBackend{}, &agent.Toolbox{}, agent.NewStore()), "saved", nil
	}
	app := NewApp(env)
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	chatIdx := -1
	for i, f := range app.features {
		if f.Chat {
			chatIdx = i
		}
	}

	// no chat configured => selecting it opens the setup
	app.Update(featureSelectedMsg{idx: chatIdx})
	require.Equal(t, scrChat, app.screen)
	require.NotNil(t, app.chat.setup)
	view := ansi.Strip(app.View())
	assert.Contains(t, view, "AI chat setup")
	assert.Contains(t, view, "Mistral AI")

	m := app.chat
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // => Gemini
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = typeText(m, "g2,g3")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // (the URL field is hidden => the key)
	// a missing key is reported in the form
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, m.setup)
	assert.Contains(t, m.setup.err, "GEMINI_API_KEY")

	m = typeText(m, "secret-key")
	assert.NotContains(t, ansi.Strip(m.View()), "secret-key") // masked
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // remember: yes
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, m.setup)
	require.NotNil(t, env.Chat)
	assert.Equal(t, ChatSetup{Backend: "gemini", Models: "g2,g3", APIKey: "secret-key", Save: true}, got[len(got)-1])
	assert.Equal(t, ChatSetup{Backend: "gemini", Models: "g2,g3"}, env.ChatCurrent)
	view = ansi.Strip(m.vp.View())
	assert.Contains(t, view, "AI chat: v · m")
	assert.Contains(t, view, "saved")

	// ctrl+l reopens it (prefilled), esc keeps the current chat
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	require.NotNil(t, m.setup)
	vals, _ := m.setup.values()
	assert.Equal(t, []string{"gemini", "g2,g3", "", "", "0"}, vals)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, m.setup)
	assert.NotNil(t, cmd) // input focus
}

func TestChatSetupCancelWithoutChat(t *testing.T) {
	env := newTestEnv(t)
	env.ChatBackends = []ChatBackend{{ID: "mistral", Label: "Mistral AI"}}
	env.ChatConfigure = func(ChatSetup) (*agent.Session, string, error) { return nil, "", nil }
	m := newChat(env)
	m.SetSize(100, 30)
	m.openSetup()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, isBack := cmd().(backMsg)
	assert.True(t, isBack) // the chat cannot be used => back to the menu
}

func TestChatSetupFormURL(t *testing.T) {
	env := newTestEnv(t)
	env.ChatBackends = []ChatBackend{
		{ID: "mistral", Label: "Mistral AI", DefaultModels: "m1", KeyName: "MISTRAL_API_KEY"},
		{ID: "openai", Label: "OpenAI-compatible", KeyName: "OPENAI_API_KEY",
			KeyOptional: true, HasURL: true, DefaultURL: "http://localhost:8080/v1"},
	}
	var got ChatSetup
	env.ChatConfigure = func(s ChatSetup) (*agent.Session, string, error) {
		got = s
		return agent.NewSession(blockingBackend{}, &agent.Toolbox{}, agent.NewStore()), "", nil
	}
	m := newChat(env)
	m.SetSize(100, 30)
	m.openSetup()
	require.NotNil(t, m.setup)
	assert.NotContains(t, ansi.Strip(m.View()), "Server URL") // mistral => hidden
	assert.Equal(t, "required", m.setup.fields[setupKey].input.Placeholder)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // => openai
	view := ansi.Strip(m.View())
	assert.Contains(t, view, "Server URL")
	assert.Contains(t, view, "http://localhost:8080/v1") // placeholder
	assert.Equal(t, "none or the configured one", m.setup.fields[setupKey].input.Placeholder)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // models
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // URL
	m = typeText(m, "http://gpu:8080/v1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, m.setup)
	assert.Equal(t, ChatSetup{Backend: "openai", URL: "http://gpu:8080/v1"}, got)
	assert.Equal(t, "http://gpu:8080/v1", env.ChatCurrent.URL)

	// reopened: prefilled with the URL
	m.openSetup()
	vals, _ := m.setup.values()
	assert.Equal(t, []string{"openai", "", "http://gpu:8080/v1", "", "0"}, vals)
}
