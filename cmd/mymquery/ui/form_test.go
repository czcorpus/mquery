package ui

import (
	"testing"

	"mquery/cmd/mymquery/api"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keyType(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func TestOptionsForm(t *testing.T) {
	f := newOptionsForm("test",
		newInputField("num", "", "5", parsePositiveInt(1, 10)),
		newChoiceField("ch", "", []selectItem{{Value: "a"}, {Value: "b"}, {Value: "c"}}, "b"),
	)
	vals, err := f.values()
	require.NoError(t, err)
	assert.Equal(t, []string{"5", "b"}, vals)

	// choices cycle (with wrapping), ↑/↓ wrap around the fields too
	f.Update(keyType(tea.KeyUp))
	assert.Equal(t, 1, f.focus)
	f.Update(keyType(tea.KeyRight))
	f.Update(keyType(tea.KeyRight))
	vals, _ = f.values()
	assert.Equal(t, "a", vals[1])
	f.Update(keyType(tea.KeyLeft))
	vals, _ = f.values()
	assert.Equal(t, "c", vals[1])

	// invalid input => the form stays open with the field focused
	f.Update(keyType(tea.KeyDown))
	assert.Equal(t, 0, f.focus)
	f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")}) // "50"
	f.Update(keyType(tea.KeyDown))
	assert.Equal(t, selectPending, f.Update(keyType(tea.KeyEnter)))
	assert.Equal(t, 0, f.focus)
	assert.NotEmpty(t, f.err)

	f.Update(keyType(tea.KeyBackspace))
	assert.Equal(t, selectChosen, f.Update(keyType(tea.KeyEnter)))
	vals, _ = f.values()
	assert.Equal(t, []string{"5", "c"}, vals)

	assert.Equal(t, selectCancelled, f.Update(keyType(tea.KeyEsc)))
}

func newTestConcView(t *testing.T) *concView {
	env := newTestEnv(t)
	env.structCache["syn"] = []corpusAttr{{Name: "doc"}, {Name: "s"}}
	v := newConcViewWithNote(env, searchTarget{CorpusItem: api.CorpusItem{ID: "syn"}}, "x", "")
	v.resp = &api.ConcordanceResponse{ConcSize: 1000}
	return v
}

// fetchArgs returns the args the view would request
func fetchArgs(t *testing.T, v *concView) api.ConcordanceArgs {
	t.Helper()
	return v.requestArgs()
}

func TestConcOptionsDefaults(t *testing.T) {
	v := newTestConcView(t)
	args := fetchArgs(t, v)
	assert.Equal(t, 10, args.ContextWidth)
	assert.Equal(t, "", args.ContextStruct)
	assert.False(t, args.TokenContext)
	assert.False(t, args.NoShuffle)
	assert.Equal(t, 2, args.ShowTextProps)
	assert.Equal(t, 50, args.MaxRows)
	assert.Equal(t, "", v.optionsSummary())

	// unchanged options => no reload
	v.optsForm = v.newOptionsForm()
	_, cmd := v.Update(keyType(tea.KeyEnter))
	assert.Nil(t, cmd)
	assert.Nil(t, v.optsForm)
}

func TestConcOptionsApply(t *testing.T) {
	v := newTestConcView(t)
	v.rowsOffset = 150

	// context structure: default → tokens → <doc>
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	require.NotNil(t, v.optsForm)
	v.Update(keyType(tea.KeyDown))
	v.Update(keyType(tea.KeyRight))
	v.Update(keyType(tea.KeyRight))
	// page size 50 → 20
	v.Update(keyType(tea.KeyUp))
	v.Update(keyType(tea.KeyUp))
	for range 2 {
		v.Update(keyType(tea.KeyBackspace))
	}
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("20")})
	_, cmd := v.Update(keyType(tea.KeyEnter))
	assert.NotNil(t, cmd)
	assert.Nil(t, v.optsForm)

	args := fetchArgs(t, v)
	assert.Equal(t, "doc", args.ContextStruct)
	assert.False(t, args.TokenContext)
	assert.Equal(t, 20, args.MaxRows)
	assert.Equal(t, 140, v.rowsOffset) // the page containing line 151
	assert.Equal(t, "context: 10 within <doc>", v.optionsSummary())

	// tokens only + corpus order (the offset is reset)
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v.Update(keyType(tea.KeyDown))
	v.Update(keyType(tea.KeyLeft)) // <doc> → tokens only
	v.Update(keyType(tea.KeyDown))
	v.Update(keyType(tea.KeyRight))
	v.Update(keyType(tea.KeyEnter))
	args = fetchArgs(t, v)
	assert.True(t, args.TokenContext, args)
	assert.True(t, args.NoShuffle)
	assert.Equal(t, 0, v.rowsOffset)
	assert.Equal(t, "context: 10 tokens, corpus order", v.optionsSummary())

	// the options are session-wide (e.g. for drill-down views)
	args = fetchArgs(t, newConcViewWithNote(v.env, v.corpus, "y", ""))
	assert.True(t, args.TokenContext)
	assert.Equal(t, 20, args.MaxRows)
}

func TestConcOptionsMarkup(t *testing.T) {
	v := newTestConcView(t)
	v.rowsOffset = 100
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v.Update(keyType(tea.KeyUp)) // → lines per page
	v.Update(keyType(tea.KeyUp)) // → structures
	v.Update(keyType(tea.KeyRight))
	_, cmd := v.Update(keyType(tea.KeyEnter))
	assert.NotNil(t, cmd)
	assert.True(t, fetchArgs(t, v).ShowMarkup)
	assert.Equal(t, 100, v.rowsOffset) // the same lines, just with markup
	assert.Equal(t, "structures", v.optionsSummary())
}
