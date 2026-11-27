package ui

import (
	"strings"
	"testing"

	"mquery/cmd/mymquery/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderJSONPlain(t *testing.T, data string, width int) string {
	t.Helper()
	n, err := parseOrderedJSON(data)
	require.NoError(t, err)
	return ansi.Strip(renderJSON(n, width))
}

func TestRenderJSONObject(t *testing.T) {
	out := renderJSONPlain(t,
		`{"total":17014,"arf":5351.68,"ipm":139.657,"corpusSize":121826797,"resultType":"termFrequency","x":0.00123,"n":null}`, 80)
	assert.Equal(t, strings.Join([]string{
		"total        17,014",
		"arf          5,351.68",
		"ipm          139.66",
		"corpus size  121,826,797",
		"x            0.00123",
		"n            —",
	}, "\n"), out) // (keys keep their order, resultType is hidden)
}

func TestRenderJSONNestedTables(t *testing.T) {
	out := renderJSONPlain(t, `{"text-type":{"concSize":10,"freqs":[
		{"word":"NOV: próza","freq":7,"ipm":1.5},
		{"word":"POE: poezie","freq":3,"ipm":12.25}
	]},"tags":["a","b"]}`, 80)
	lines := strings.Split(out, "\n")
	assert.Equal(t, "text type", lines[0])
	assert.Equal(t, "  conc size  10", lines[1])
	assert.Contains(t, out, "  freqs\n")
	assert.Regexp(t, `word\s+│\s+freq\s+│\s+ipm`, out)
	assert.Regexp(t, `NOV: próza\s+│\s+7 │\s+1\.50`, out) // numbers are right-aligned
	assert.Contains(t, out, "tags\n  a, b")
}

func TestRenderJSONMixedArray(t *testing.T) {
	out := renderJSONPlain(t, `[{"a":1},[1,2],"x"]`, 80)
	assert.Equal(t, "• a  1\n• 1, 2\n• x", out)
	assert.Equal(t, "(empty)", renderJSONPlain(t, `[]`, 80))
	_, err := parseOrderedJSON(`{"a":1} trailing`)
	assert.Error(t, err)
}

func TestHumanizeJSONKey(t *testing.T) {
	assert.Equal(t, "corpus size", humanizeJSONKey("corpusSize"))
	assert.Equal(t, "text type", humanizeJSONKey("text-type"))
	assert.Equal(t, "max items", humanizeJSONKey("max_items"))
	assert.Equal(t, "IPM", humanizeJSONKey("IPM"))
}

func TestGenericResultViewToggle(t *testing.T) {
	res := &agent.Result{ID: "r1", Tool: "term_frequency", OK: true,
		Content: `{"total":17014,"ipm":139.657}`}
	v := newChatResultView(newTestEnv(t), res).(*rawResultView)
	v.SetSize(80, 20)
	assert.Contains(t, ansi.Strip(v.View()), "total  17,014")
	assert.Contains(t, v.Help(), "r raw JSON")

	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	assert.Contains(t, ansi.Strip(v.View()), `"total": 17014`)
	assert.Contains(t, v.Help(), "r readable form")

	// non-JSON (e.g. an error) is shown as text, without the toggle
	failed := newRawResultView(&agent.Result{ID: "r2", Tool: "freqs", Content: "error: bad query"})
	failed.SetSize(80, 20)
	assert.Contains(t, ansi.Strip(failed.View()), "error: bad query")
	assert.NotContains(t, failed.Help(), "raw JSON")
}
