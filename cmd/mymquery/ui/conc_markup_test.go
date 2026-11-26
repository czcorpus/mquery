package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markupLine is a line as returned by /concordance with showMarkup=1
func markupLine(t *testing.T) concordance.Line {
	var line concordance.Line
	require.NoError(t, json.Unmarshal([]byte(`{"text":[
		{"type":"markup","structureType":"open","name":"s","attrs":{"id":"s1"}},
		{"type":"token","word":"Zvedl","attrs":{"lemma":"zvednout"}},
		{"type":"token","word":"psovi","strong":true,"matchType":"kwic","attrs":{"lemma":"pes"}},
		{"type":"token","word":"hlavu","attrs":{"lemma":"hlava"}},
		{"type":"markup","structureType":"self-close","name":"g"},
		{"type":"token","word":".","attrs":{"lemma":"."}},
		{"type":"markup","structureType":"self-close","name":"lb"},
		{"type":"markup","structureType":"close","name":"s"}
	]}`), &line))
	return line
}

func TestRenderElements(t *testing.T) {
	line := markupLine(t)
	assert.Equal(t, "<s> Zvedl psovi hlavu. <lb/> </s>",
		ansi.Strip(renderElements(line.Text, lipgloss.NewStyle(), "word")))
	assert.Equal(t, "<s> zvednout pes hlava. <lb/> </s>",
		ansi.Strip(renderElements(line.Text, lipgloss.NewStyle(), "lemma")))
}

func TestKwicLayoutWithMarkup(t *testing.T) {
	v := newTestConcView(t)
	plain := concordance.Line{Text: concordance.TokenSlice{}}
	for _, e := range markupLine(t).Text {
		if tok, ok := e.(*concordance.Token); ok {
			plain.Text = append(plain.Text, tok)
		}
	}
	// KWIC starts in the same column with and without markup
	withMarkup := ansi.Strip(v.kwicLayout(markupLine(t), lipgloss.NewStyle(), 60))
	without := ansi.Strip(v.kwicLayout(plain, lipgloss.NewStyle(), 60))
	assert.Equal(t, strings.Index(without, "psovi"), strings.Index(withMarkup, "psovi"))
	assert.Equal(t, 60, ansi.StringWidth(withMarkup))
	assert.Contains(t, withMarkup, "<s> Zvedl psovi hlavu. <lb/> </s>")
	assert.Contains(t, without, "Zvedl psovi hlavu .")
}

func TestKwicLayoutGlueAfterKwic(t *testing.T) {
	v := newTestConcView(t)
	line := markupLine(t)
	// "Zvedl psovi<g/>. ..." (glue right after KWIC)
	line.Text = append(line.Text[:3:3], line.Text[4:]...)
	got := ansi.Strip(v.kwicLayout(line, lipgloss.NewStyle(), 60))
	assert.Equal(t, 60, ansi.StringWidth(got))
	assert.Contains(t, got, "Zvedl psovi. <lb/> </s>")
	assert.Equal(t, strings.Index(ansi.Strip(v.kwicLayout(markupLine(t), lipgloss.NewStyle(), 60)), "psovi"),
		strings.Index(got, "psovi"))
}
