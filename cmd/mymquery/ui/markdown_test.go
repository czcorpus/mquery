package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAnswer = `The lemma **pes** occurs *17,014* times [r1].

Top collocates [r2, r3]:
- kočka (cat)
- ` + "`štěkat`" + ` (to bark)

| lemma | logDice |
|-------|---------|
| kočka | 9.1 |
| štěkat | 8.7 |`

func TestRenderMarkdown(t *testing.T) {
	r := newMDRenderer()
	out := r.Render(testAnswer, 40, true)
	t.Log("\n" + out)
	plain := ansi.Strip(out)
	assert.False(t, strings.HasPrefix(plain, "\n"))
	assert.False(t, strings.HasSuffix(plain, "\n"))
	assert.NotContains(t, plain, "**")
	assert.Contains(t, plain, "The lemma pes occurs 17,014 times [r1].")
	assert.Contains(t, plain, "[r2, r3]")
	assert.NotContains(t, plain, string(refOpen))
	assert.Contains(t, plain, "kočka")
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), 40, line)
		assert.Equal(t, strings.TrimRight(ansi.Strip(line), " "), ansi.Strip(line))
	}
	// references are highlighted
	assert.Contains(t, out, kwicStyle.Render("[r1]"))

	// cached vs. not cached
	assert.Len(t, r.cache, 1)
	assert.Equal(t, out, r.Render(testAnswer, 40, true))
	r.Render("partial **text", 40, false)
	assert.Len(t, r.cache, 1)
	assert.Contains(t, ansi.Strip(r.Render("partial **text", 40, false)), "partial")
}

func TestTrimRightSpaceANSI(t *testing.T) {
	assert.Equal(t, "abc", trimRightSpaceANSI("abc"))
	line := "\x1b[1mab\x1b[0m   "
	assert.Equal(t, "ab", ansi.Strip(trimRightSpaceANSI(line)))
}

func TestSplitMDTables(t *testing.T) {
	segs := splitMDTables("intro\n| a | b |\n|---|--:|\n| 1 | 2 |\nafter\n```\n| x |\n|---|\n```")
	require.Len(t, segs, 3)
	assert.Equal(t, mdSegment{text: "intro"}, segs[0])
	assert.Equal(t, mdSegment{text: "| a | b |\n|---|--:|\n| 1 | 2 |", isTable: true}, segs[1])
	assert.False(t, segs[2].isTable) // the table in a code block is kept

	// a pipe line without a separator is not a table
	assert.Len(t, splitMDTables("| just text |\nmore"), 1)
}

func TestRenderMDTable(t *testing.T) {
	out := ansi.Strip(renderMDTable("| lemma | freq |\n|:--|--:|\n| **pes** | 17014 [r1] |\n| kočka \\| cat | 5637 |", 80))
	t.Log("\n" + out)
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 4)
	assert.Contains(t, lines[2], "pes ")
	assert.NotContains(t, out, "**")
	assert.Contains(t, lines[3], "kočka | cat")
	// natural width (not stretched), right alignment
	assert.Less(t, ansi.StringWidth(lines[0]), 40)
	assert.True(t, strings.HasSuffix(lines[3], " 5637"))

	// a wide table is wrapped to the max. width
	wide := "| a | b |\n|---|---|\n| " + strings.Repeat("word ", 30) + "| x |"
	for _, line := range strings.Split(renderMDTable(wide, 40), "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), 40)
	}
}
