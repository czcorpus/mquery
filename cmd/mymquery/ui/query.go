package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// querySubmittedMsg carries the submitted query and (for features
// supporting it) an optional collocate filter
type querySubmittedMsg struct {
	query string
	// coll is a collocate CQL query ("" = no collocate filter)
	coll string
	// collWindow is [left, right] (both >= 0) around KWIC
	collWindow [2]int
}

type changeCorpusMsg struct{}

// openSubcPickerMsg asks for the subcorpus picker
type openSubcPickerMsg struct{}

// query screen input fields
const (
	fieldQuery = iota
	fieldColl
	fieldWindow
)

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

	// hasColl enables the collocate filter fields (coll query + window)
	hasColl     bool
	collInput   textinput.Model
	windowInput textinput.Model
	// focus is one of fieldQuery, fieldColl, fieldWindow
	focus int

	// corpusID and subcSummary describe the search target (set by the app)
	corpusID    string
	subcSummary string
}

func newTextInput(prompt string, limit int) textinput.Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.PromptStyle = accentStyle
	ti.CharLimit = limit
	return ti
}

func newQuery(history *queryHistory) queryModel {
	coll := newTextInput("❯ ", 2000)
	coll.Placeholder = `optional, e.g. [lemma="big"]`
	win := newTextInput("❯ ", 20)
	win.Placeholder = "left,right (e.g. 3,5)"
	return queryModel{
		input:       newTextInput("❯ ", 2000),
		history:     history,
		histPos:     -1,
		collInput:   coll,
		windowInput: win,
	}
}

// Reset prepares the input for a (possibly different) feature
func (m *queryModel) Reset(f Feature, value, coll string, window [2]int) tea.Cmd {
	m.label = f.InputLabel
	m.input.Placeholder = f.InputPlaceholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.notice = ""
	m.useHistory = f.QueryType == queryTypeCQL
	m.histPos = -1
	m.draft = ""
	m.hasColl = f.HasColl
	m.collInput.SetValue(coll)
	m.collInput.CursorEnd()
	m.windowInput.SetValue(windowValue(window))
	m.windowInput.CursorEnd()
	return m.setFocus(fieldQuery)
}

// SetTarget sets the corpus and subcorpus shown above the inputs
func (m *queryModel) SetTarget(corpusID, subcSummary string) {
	m.corpusID, m.subcSummary = corpusID, subcSummary
}

// Blur removes focus from all the fields
func (m *queryModel) Blur() {
	m.input.Blur()
	m.collInput.Blur()
	m.windowInput.Blur()
}

func (m *queryModel) setFocus(field int) tea.Cmd {
	m.Blur()
	m.focus = field
	return m.focused().Focus()
}

func (m *queryModel) focused() *textinput.Model {
	switch m.focus {
	case fieldColl:
		return &m.collInput
	case fieldWindow:
		return &m.windowInput
	default:
		return &m.input
	}
}

// cycleFocus moves the focus by delta among the fields (wrapping around)
func (m *queryModel) cycleFocus(delta int) tea.Cmd {
	if !m.hasColl {
		return nil
	}
	const numFields = 3
	return m.setFocus(((m.focus+delta)%numFields + numFields) % numFields)
}

// browseHistory moves in the history by delta (+1 = older, -1 = newer);
// moving past the most recent query restores the original input
func (m *queryModel) browseHistory(delta int) {
	if !m.useHistory || m.focus != fieldQuery {
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
	width := max(10, w-4)
	m.input.Width = width
	m.collInput.Width = width
	m.windowInput.Width = width
}

func (m queryModel) submit() (queryModel, tea.Cmd) {
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		m.notice = "the query must not be empty"
		return m, m.setFocus(fieldQuery)
	}
	msg := querySubmittedMsg{query: q}
	if m.hasColl {
		msg.coll = strings.TrimSpace(m.collInput.Value())
		if msg.coll != "" {
			w, err := parseWindowPair(m.windowInput.Value())
			if err != nil {
				m.notice = "invalid collocate window: " + err.Error()
				return m, m.setFocus(fieldWindow)
			}
			msg.collWindow = w
		}
	}
	return m, func() tea.Msg { return msg }
}

func (m queryModel) Update(msg tea.Msg) (queryModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			return m.submit()
		case "esc":
			return m, goBack
		case "ctrl+o":
			return m, func() tea.Msg { return changeCorpusMsg{} }
		case "ctrl+s":
			return m, func() tea.Msg { return openSubcPickerMsg{} }
		case "tab":
			return m, m.cycleFocus(1)
		case "shift+tab":
			return m, m.cycleFocus(-1)
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
	if _, ok := msg.(tea.KeyMsg); ok {
		f := m.focused()
		*f, cmd = f.Update(msg)
		return m, cmd
	}
	// non-key messages (cursor blinking) go to all inputs; unfocused ones ignore them
	var cmds [3]tea.Cmd
	m.input, cmds[0] = m.input.Update(msg)
	m.collInput, cmds[1] = m.collInput.Update(msg)
	m.windowInput, cmds[2] = m.windowInput.Update(msg)
	return m, tea.Batch(cmds[:]...)
}

func (m queryModel) fieldLabel(field int, text string) string {
	if m.focus == field {
		return titleStyle.Render(text)
	}
	return mutedStyle.Render(text)
}

func (m queryModel) View() string {
	var b strings.Builder
	if m.corpusID != "" {
		b.WriteString(mutedStyle.Render("Corpus: ") + accentStyle.Render(m.corpusID) +
			mutedStyle.Render("  ·  Subcorpus: ") + accentStyle.Render(m.subcSummary) + "\n\n")
	}
	b.WriteString(m.fieldLabel(fieldQuery, m.label) + "\n\n")
	b.WriteString(m.input.View() + "\n")
	if m.hasColl {
		b.WriteString("\n" + m.fieldLabel(fieldColl, "Collocate (CQL)") + "\n\n")
		b.WriteString(m.collInput.View() + "\n")
		b.WriteString("\n" + m.fieldLabel(fieldWindow, "Collocate window (tokens left,right of KWIC)") + "\n\n")
		b.WriteString(m.windowInput.View() + "\n")
	}
	if m.notice != "" {
		b.WriteString("\n" + errorStyle.Render(m.notice) + "\n")
	} else if m.histPos >= 0 {
		b.WriteString("\n" + mutedStyle.Render(fmt.Sprintf("history %d/%d", m.histPos+1, m.history.Len())) + "\n")
	}
	return b.String()
}

func (m queryModel) Help() string {
	parts := []string{"enter search"}
	if m.hasColl {
		parts = append(parts, "tab/shift+tab switch field")
	}
	if m.useHistory && m.focus == fieldQuery {
		parts = append(parts, "↑/↓ history")
	}
	parts = append(parts, "ctrl+o change corpus", "ctrl+s subcorpus", "esc back to menu", "ctrl+c quit")
	return strings.Join(parts, " • ")
}
