package ui

import (
	"path/filepath"
	"testing"

	"mquery/cmd/mymquery/storage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryHistoryAddDedupAndLimit(t *testing.T) {
	h := newQueryHistory(nil, 3)
	for _, q := range []string{"a", "b", "a", "  ", "c\nd", "e"} {
		assert.NoError(t, h.Add("syn", "", "conc", q))
	}
	assert.Equal(t, []string{"a", "c d", "e"}, h.items)
	assert.Equal(t, "e", h.Recent(0))
	assert.Equal(t, "a", h.Recent(2))
}

func TestQueryHistoryPersistence(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	h := newQueryHistory(db, 10)
	assert.NoError(t, h.Load())
	assert.NoError(t, h.Add("syn", "", "conc", `[lemma="house"]`))
	assert.NoError(t, h.Add("syn", "", "freqs", `[word="dog"]`))
	assert.NoError(t, h.Add("syn", "", "freqs", "  "))

	h2 := newQueryHistory(db, 10)
	assert.NoError(t, h2.Load())
	assert.Equal(t, []string{`[lemma="house"]`, `[word="dog"]`}, h2.items)
}

func TestQueryHistoryBrowsing(t *testing.T) {
	h := newQueryHistory(nil, 10)
	h.add("first")
	h.add("second")
	m := newQuery(h)
	m.Reset(Feature{QueryType: queryTypeCQL}, "draft", "", [2]int{5, 5})

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
	m.Reset(Feature{QueryType: "other"}, "x", "", [2]int{5, 5})
	press(tea.KeyUp)
	assert.Equal(t, "x", m.input.Value())
}
