package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// chat setup form fields (in the form order)
const (
	setupBackend = iota
	setupModels
	setupURL
	setupKey
	setupSave
)

// newChatSetupForm creates a form for choosing the AI chat backend, models
// and API key (prefilled with the current setup)
func newChatSetupForm(env *Env) *optionsForm {
	cur := env.ChatCurrent
	var backends []selectItem
	var defaults []string
	for _, b := range env.ChatBackends {
		backends = append(backends, selectItem{Value: b.ID, Label: b.Label})
		defaults = append(defaults, fmt.Sprintf("%s: %s", b.Label, b.DefaultModels))
	}
	if cur.Backend == "" && len(backends) > 0 {
		cur.Backend = backends[0].Value
	}
	keyHint := "the key is not shown; empty = the currently configured key of the backend (if any)"
	var urlHint, defaultURL string
	for _, b := range env.ChatBackends {
		if b.KeyOptional {
			keyHint += "; " + b.Label + ": optional (for a server requiring authentication, " +
				"e.g. llama-server --api-key)"
		}
		if b.HasURL {
			urlHint = "the server's OpenAI-compatible API (e.g. llama-server); empty = " + b.DefaultURL
			defaultURL = b.DefaultURL
		}
	}
	curURL := ""
	if chatBackend(env, cur.Backend).HasURL {
		curURL = cur.URL
	}
	savePath := env.ChatConfigPath
	if savePath == "" {
		savePath = "your configuration"
	}
	form := newOptionsForm(
		"AI chat setup",
		newChoiceField("Backend", "", backends, cur.Backend),
		newInputField("Models", "comma separated, the first one is primary, the others are fallbacks; "+
			"empty = the default ("+strings.Join(defaults, "; ")+")",
			cur.Models, func(s string) (string, error) { return s, nil }).
			withCharLimit(200).withPlaceholder("default"),
		newInputField("Server URL", urlHint, curURL, func(s string) (string, error) { return s, nil }).
			withCharLimit(500).withPlaceholder(defaultURL),
		newInputField("API key", keyHint, "", func(s string) (string, error) { return s, nil }).
			withCharLimit(500).masked(),
		newChoiceField("Remember",
			"saved to "+savePath+" (unencrypted, readable only by you); otherwise the setup "+
				"is used just until you quit",
			[]selectItem{{Value: "0", Label: "no (this run only)"}, {Value: "1", Label: "yes, save it"}}, "0"),
	)
	form.minWidth = chatSetupWidth
	form.fields[setupURL].shownIf(func() bool {
		return chatBackend(env, form.fields[setupBackend].options[form.fields[setupBackend].choice].Value).HasURL
	})
	updateSetupKeyPlaceholder(env, form)
	return form
}

// chatBackend returns the backend's description (zero if unknown)
func chatBackend(env *Env, id string) ChatBackend {
	for _, b := range env.ChatBackends {
		if b.ID == id {
			return b
		}
	}
	return ChatBackend{}
}

// updateSetupKeyPlaceholder describes an empty API key for the chosen backend
func updateSetupKeyPlaceholder(env *Env, form *optionsForm) {
	backend := form.fields[setupBackend].options[form.fields[setupBackend].choice].Value
	ph := "required"
	switch {
	case env.Chat != nil && backend == env.ChatCurrent.Backend:
		ph = "keep the current key"
	case chatBackend(env, backend).KeyOptional:
		ph = "none or the configured one"
	}
	form.fields[setupKey].withPlaceholder(ph)
}

// chatSetupWidth is the inner width of the setup form (if the screen allows)
const chatSetupWidth = 76

// openSetup shows the AI chat setup form (unless a turn is running)
func (m *chatModel) openSetup() {
	if m.running || m.env.ChatConfigure == nil || len(m.env.ChatBackends) == 0 {
		return
	}
	m.setup = newChatSetupForm(m.env)
	m.input.Blur()
}

func (m chatModel) updateSetup(msg tea.KeyMsg) (chatModel, tea.Cmd) {
	res := m.setup.Update(msg)
	if res == selectPending {
		updateSetupKeyPlaceholder(m.env, m.setup)
	}
	switch res {
	case selectCancelled:
		m.setup = nil
		if m.env.Chat == nil {
			return m, goBack // the chat cannot be used without a setup
		}
		return m, m.input.Focus()

	case selectChosen:
		vals, _ := m.setup.values()
		setup := ChatSetup{
			Backend: vals[setupBackend],
			Models:  strings.TrimSpace(vals[setupModels]),
			URL:     strings.TrimSpace(vals[setupURL]),
			APIKey:  strings.TrimSpace(vals[setupKey]),
			Save:    vals[setupSave] == "1",
		}
		sess, notice, err := m.env.ChatConfigure(setup)
		if err != nil {
			m.setup.err = err.Error()
			return m, nil
		}
		m.setup = nil
		hadChat := m.env.Chat != nil && m.env.Chat.ChatID() != 0
		m.env.Chat = sess
		m.env.ChatCurrent = ChatSetup{Backend: setup.Backend, Models: setup.Models, URL: setup.URL}
		m.newConversation()
		info := "── AI chat: " + sess.Backend().VendorName() + " · " + sess.Backend().ModelName()
		if hadChat {
			info += " (new conversation, the previous one is saved)"
		}
		m.add(chatEntry{kind: entryInfo, text: info + " ──"})
		if notice != "" {
			m.add(chatEntry{kind: entryInfo, text: notice})
		}
		return m, m.input.Focus()
	}
	return m, nil
}
