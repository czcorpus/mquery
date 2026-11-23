package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestQueryHistoryAddDedupAndLimit(t *testing.T) {
	h := newQueryHistory("", 3)
	for _, q := range []string{"a", "b", "a", "  ", "c\nd", "e"} {
		assert.NoError(t, h.Add(q))
	}
	assert.Equal(t, []string{"a", "c d", "e"}, h.items)
	assert.Equal(t, "e", h.Recent(0))
	assert.Equal(t, "a", h.Recent(2))
}

func TestQueryHistoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "history")
	h := newQueryHistory(path, 10)
	assert.NoError(t, h.Load()) // missing file is OK
	assert.NoError(t, h.Add(`[lemma="house"]`))
	assert.NoError(t, h.Add(`[word="dog"]`))

	h2 := newQueryHistory(path, 10)
	assert.NoError(t, h2.Load())
	assert.Equal(t, []string{`[lemma="house"]`, `[word="dog"]`}, h2.items)

	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestQueryHistoryBrowsing(t *testing.T) {
	h := newQueryHistory("", 10)
	h.add("first")
	h.add("second")
	m := newQuery(h)
	m.Reset(Feature{QueryType: queryTypeCQL}, "draft")

	press := func(k tea.KeyType) {
		m, _ = m.Update(tea.KeyMsg{Type: k})
	}
	press(tea.KeyUp)
	assert.Equal(t, "second", m.input.Value())
	press(tea.KeyUp)
	assert.Equal(t, "first", m.input.Value())
	press(tea.KeyUp) // stays at the oldest one
	assert.Equal(t, "first", m.input.Value())
	press(tea.KeyDown)
	press(tea.KeyDown)
	assert.Equal(t, "draft", m.input.Value())
	press(tea.KeyDown) // stays at the draft
	assert.Equal(t, "draft", m.input.Value())

	// no history for non-CQL inputs
	m.Reset(Feature{QueryType: "other"}, "x")
	press(tea.KeyUp)
	assert.Equal(t, "x", m.input.Value())
}
