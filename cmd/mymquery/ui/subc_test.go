package ui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/storage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSubcTestEnv creates an env with a DB and a fake /subcorpus endpoint
// knowing a saved subcorpus `good1` (of size 100); `bad1` does not belong
// to the corpus, anything else does not exist
func newSubcTestEnv(t *testing.T) *Env {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subcorpus/syn/good1":
			w.Write([]byte(`{"id":"good1","corpusId":"syn","type":"saved","size":100,"numRanges":2,"corpusSize":1000}`))
		case "/subcorpus/syn/bad1":
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"error":"subcorpus bad1 cannot belong to corpus syn"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"subcorpus not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(srv.URL, nil, time.Second)
	require.NoError(t, err)
	env := NewEnv(client, "", 50, 10)
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	env.DB = db
	return env
}

var subcTestCorpus = api.CorpusItem{
	ID:         "syn",
	Subcorpora: []api.Subcorpus{{ID: "fict", Description: "fiction"}},
}

// typeInto sends the text as single-rune keys (like a terminal does)
func typeInto(p *subcPicker, text string) {
	for _, r := range text {
		p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// runCmd runs the command and feeds the resulting message back to the picker
// until a subcSelectedMsg appears (returned) or there is nothing to do
func runCmd(p *subcPicker, cmd tea.Cmd) *subcSelectedMsg {
	for cmd != nil {
		switch msg := cmd().(type) {
		case subcSelectedMsg:
			return &msg
		default:
			cmd = p.Update(msg)
		}
	}
	return nil
}

func TestParseSubcID(t *testing.T) {
	v, err := parseSubcID("  ab12 ")
	assert.NoError(t, err)
	assert.Equal(t, "ab12", v)
	for _, s := range []string{"", "a", "a b", "ab/cd", `ab\cd`} {
		_, err := parseSubcID(s)
		assert.Error(t, err, s)
	}
}

func TestSubcStateSummary(t *testing.T) {
	assert.Equal(t, "whole corpus", subcState{}.summary())
	assert.Equal(t, "fict (configured)", subcState{ID: "fict", Alias: true}.summary())
	assert.Equal(t, "ab (100 tokens, 10.0 % of the corpus)",
		subcState{ID: "ab", Size: 100, CorpusSize: 1000}.summary())
}

func TestSubcPickerAliasAndWholeCorpus(t *testing.T) {
	p := newSubcPicker(newSubcTestEnv(t))
	p.Open(subcTestCorpus, "")
	require.True(t, p.Active())
	typeInto(p, "fict")
	msg := runCmd(p, p.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	require.NotNil(t, msg)
	assert.Equal(t, subcState{ID: "fict", Alias: true}, msg.subc)
	assert.False(t, p.Active())

	p.Open(subcTestCorpus, "fict")
	p.Update(tea.KeyMsg{Type: tea.KeyHome})
	msg = runCmd(p, p.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	require.NotNil(t, msg)
	assert.Equal(t, subcState{}, msg.subc)
}

func TestSubcPickerTypedID(t *testing.T) {
	env := newSubcTestEnv(t)
	p := newSubcPicker(env)

	// a subcorpus of a different corpus is rejected, the picker stays open
	p.Open(subcTestCorpus, "")
	typeInto(p, "bad1")
	assert.Nil(t, runCmd(p, p.Update(tea.KeyMsg{Type: tea.KeyEnter})))
	assert.True(t, p.Active())
	assert.Contains(t, p.list.err, "cannot belong")

	// a valid one is selected and stored
	for range 4 {
		p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	typeInto(p, "good1")
	msg := runCmd(p, p.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	require.NotNil(t, msg)
	assert.Equal(t, subcState{ID: "good1", Size: 100, CorpusSize: 1000}, msg.subc)
	assert.Empty(t, msg.notice)
	saved, err := env.DB.Subcorpora("syn")
	require.NoError(t, err)
	require.Len(t, saved, 1)
	assert.Equal(t, int64(100), saved[0].Size)

	// next time, it is offered
	p.Open(subcTestCorpus, "good1")
	it, ok := p.list.selected()
	require.True(t, ok)
	assert.Equal(t, "good1", it.Value)
	assert.Contains(t, it.Hint, "100 tokens")
}

func TestSubcPickerLabelAndForget(t *testing.T) {
	env := newSubcTestEnv(t)
	require.NoError(t, env.DB.UseSubcorpus("syn", "good1", 100))
	p := newSubcPicker(env)

	// aliases cannot be labelled
	p.Open(subcTestCorpus, "fict")
	p.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	assert.Nil(t, p.labelInput)
	assert.NotEmpty(t, p.list.err)

	p.Open(subcTestCorpus, "good1")
	p.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	require.NotNil(t, p.labelInput)
	typeInto(p, "news")
	p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, p.labelInput)
	it, _ := p.list.selected()
	assert.Equal(t, "news (good1)", it.Label)

	// the label can be used for filtering
	p.Open(subcTestCorpus, "")
	typeInto(p, "news")
	it, _ = p.list.selected()
	assert.Equal(t, "good1", it.Value)

	// forgetting needs a confirmation
	p.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	saved, _ := env.DB.Subcorpora("syn")
	assert.Len(t, saved, 1)
	p.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	saved, _ = env.DB.Subcorpora("syn")
	assert.Empty(t, saved)
	assert.True(t, p.Active())
}

func TestSubcPickerCancelCheck(t *testing.T) {
	p := newSubcPicker(newSubcTestEnv(t))
	p.Open(subcTestCorpus, "")
	typeInto(p, "good1")
	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, "good1", p.checking)
	p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	// a late response is ignored
	assert.Nil(t, p.Update(cmd()))
	assert.True(t, p.Active())
}
