package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type featureSelectedMsg struct{ idx int }

type menuModel struct {
	features []Feature
	cursor   int
	notice   string
}

func newMenu(features []Feature) menuModel {
	return menuModel{features: features}
}

func (m menuModel) Update(msg tea.Msg) (menuModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.notice = ""
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.features)-1 {
			m.cursor++
		}
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := int(key.Runes[0] - '1'); i < len(m.features) {
			m.cursor = i
			return m.choose()
		}
	case "enter", " ":
		return m.choose()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m menuModel) choose() (menuModel, tea.Cmd) {
	if !m.features[m.cursor].Implemented() {
		m.notice = m.features[m.cursor].Title + " is not implemented yet"
		return m, nil
	}
	idx := m.cursor
	return m, func() tea.Msg { return featureSelectedMsg{idx: idx} }
}

func (m menuModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose a function") + "\n\n")
	for i, f := range m.features {
		marker := "  "
		title := f.Title
		if i == m.cursor {
			marker = accentStyle.Render("❯ ")
			title = boldStyle.Render(title)
		}
		line := marker + mutedStyle.Render(string(rune('1'+i))+". ") + title
		desc := f.Description
		if !f.Implemented() {
			desc += " (not implemented yet)"
		}
		b.WriteString(line + "  " + mutedStyle.Render(desc) + "\n")
	}
	if m.notice != "" {
		b.WriteString("\n" + errorStyle.Render(m.notice) + "\n")
	}
	return b.String()
}

func (m menuModel) Help() string {
	return "↑/↓ move • enter select • 1-4 quick select • q quit"
}
