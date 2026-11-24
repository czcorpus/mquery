package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func submitQuery(t *testing.T, m queryModel) (queryModel, *querySubmittedMsg) {
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		return m, nil
	}
	msg, ok := cmd().(querySubmittedMsg)
	if !ok {
		return m, nil
	}
	return m, &msg
}

func TestQueryFieldSwitching(t *testing.T) {
	m := newQuery(newQueryHistory("", 10))
	m.Reset(Feature{QueryType: queryTypeCQL, HasColl: true}, "[word=\"a\"]", "", [2]int{5, 5})
	assert.Equal(t, fieldQuery, m.focus)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, fieldColl, m.focus)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[x]")})
	assert.Equal(t, "[x]", m.collInput.Value())
	assert.Equal(t, "[word=\"a\"]", m.input.Value())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, fieldWindow, m.focus)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, fieldQuery, m.focus)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Equal(t, fieldWindow, m.focus)
}

func TestQueryTabWithoutColl(t *testing.T) {
	m := newQuery(newQueryHistory("", 10))
	m.Reset(Feature{QueryType: queryTypeCQL}, "q", "", [2]int{5, 5})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, fieldQuery, m.focus)
	_, msg := submitQuery(t, m)
	assert.Equal(t, &querySubmittedMsg{query: "q"}, msg)
}

func TestQuerySubmitColl(t *testing.T) {
	m := newQuery(newQueryHistory("", 10))
	m.Reset(Feature{QueryType: queryTypeCQL, HasColl: true}, "q", " [c] ", [2]int{5, 5})
	m.windowInput.SetValue("3 0")
	_, msg := submitQuery(t, m)
	assert.Equal(t, &querySubmittedMsg{query: "q", coll: "[c]", collWindow: [2]int{3, 0}}, msg)

	// invalid window is reported and focused
	m.windowInput.SetValue("0,0")
	m, msg = submitQuery(t, m)
	assert.Nil(t, msg)
	assert.Equal(t, fieldWindow, m.focus)
	assert.Contains(t, m.notice, "window")

	// window is ignored without a collocate
	m.collInput.SetValue("")
	_, msg = submitQuery(t, m)
	assert.Equal(t, &querySubmittedMsg{query: "q"}, msg)
}
