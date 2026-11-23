package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type querySubmittedMsg struct{ query string }

type changeCorpusMsg struct{}

type queryModel struct {
	input   textinput.Model
	label   string
	notice  string
	history *queryHistory
	// useHistory is false for inputs other than CQL queries
	useHistory bool
	// histPos is the position in the history while browsing it
	// (-1 = not browsing, 0 = the most recent query, ...)
	histPos int
	// draft is the input value before history browsing started
	draft string
}

func newQuery(history *queryHistory) queryModel {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.PromptStyle = accentStyle
	ti.CharLimit = 2000
	return queryModel{input: ti, history: history, histPos: -1}
}

// Reset prepares the input for a (possibly different) feature
func (m *queryModel) Reset(f Feature, value string) tea.Cmd {
	m.label = f.InputLabel
	m.input.Placeholder = f.InputPlaceholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.notice = ""
	m.useHistory = f.QueryType == queryTypeCQL
	m.histPos = -1
	m.draft = ""
	return m.input.Focus()
}

// browseHistory moves in the history by delta (+1 = older, -1 = newer);
// moving past the most recent query restores the original input
func (m *queryModel) browseHistory(delta int) {
	if !m.useHistory {
		return
	}
	pos := m.histPos + delta
	if pos >= m.history.Len() || pos < -1 {
		return
	}
	if m.histPos == -1 {
		m.draft = m.input.Value()
	}
	m.histPos = pos
	if pos == -1 {
		m.input.SetValue(m.draft)
	} else {
		m.input.SetValue(m.history.Recent(pos))
	}
	m.input.CursorEnd()
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
		case "up", "ctrl+p":
			m.browseHistory(1)
			return m, nil
		case "down", "ctrl+n":
			m.browseHistory(-1)
			return m, nil
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
	} else if m.histPos >= 0 {
		b.WriteString("\n" + mutedStyle.Render(fmt.Sprintf("history %d/%d", m.histPos+1, m.history.Len())) + "\n")
	}
	return b.String()
}

func (m queryModel) Help() string {
	if !m.useHistory {
		return "enter search • tab change corpus • esc back to menu • ctrl+c quit"
	}
	return "enter search • ↑/↓ history • tab change corpus • esc back to menu • ctrl+c quit"
}
