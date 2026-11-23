package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type querySubmittedMsg struct{ query string }

type changeCorpusMsg struct{}

type queryModel struct {
	input  textinput.Model
	label  string
	notice string
}

func newQuery() queryModel {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.PromptStyle = accentStyle
	ti.CharLimit = 2000
	return queryModel{input: ti}
}

// Reset prepares the input for a (possibly different) feature
func (m *queryModel) Reset(f Feature, value string) tea.Cmd {
	m.label = f.InputLabel
	m.input.Placeholder = f.InputPlaceholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.notice = ""
	return m.input.Focus()
}

func (m *queryModel) SetSize(w, _ int) {
	m.input.Width = max(10, w-4)
}

func (m queryModel) Update(msg tea.Msg) (queryModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			q := strings.TrimSpace(m.input.Value())
			if q == "" {
				m.notice = "the query must not be empty"
				return m, nil
			}
			return m, func() tea.Msg { return querySubmittedMsg{query: q} }
		case "esc":
			return m, goBack
		case "tab":
			return m, func() tea.Msg { return changeCorpusMsg{} }
		}
		m.notice = ""
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m queryModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.label) + "\n\n")
	b.WriteString(m.input.View() + "\n")
	if m.notice != "" {
		b.WriteString("\n" + errorStyle.Render(m.notice) + "\n")
	}
	return b.String()
}

func (m queryModel) Help() string {
	return "enter search • tab change corpus • esc back to menu • ctrl+c quit"
}
