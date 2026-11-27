package ui

import (
	"fmt"
	"testing"

	"mquery/cmd/mymquery/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatSearchMatch(t *testing.T) {
	text := newSearchText("Kočka a PES\nThe most frequent adjectives modifying the noun pes are starý and hlídací.", 0)
	cases := []struct {
		query string
		match bool
	}{
		{"pes", true},
		{"kocka pes", true}, // all the words, anywhere, any order
		{"KOČKA", true},     // case-insensitive
		{"kočka", true},     // with diacritics => exact diacritics
		{"kočká", false},
		{"hlidaci stary", true}, // without diacritics => any diacritics
		{"hlídací", true},
		{"hlidací", true},     // (each word on its own)
		{"pěs", false},        // diacritics must match
		{"pes koza", false},   // all the words must match
		{"modify noun", true}, // substrings
		{"", true},
	}
	for _, c := range cases {
		ok, _ := text.match(parseChatSearch(c.query))
		assert.Equal(t, c.match, ok, c.query)
	}
}

func TestChatSearchSnippet(t *testing.T) {
	text := newSearchText("Kočka a PES\nThe most frequent adjectives modifying the noun pes are starý and hlídací words.", 0)
	_, snippet := text.match(parseChatSearch("hlidaci kocka"))
	// around the first word, the original text on a single line
	assert.Equal(t, "…the noun pes are starý and hlídací words.", snippet)
	_, snippet = text.match(parseChatSearch("kocka"))
	assert.Equal(t, "Kočka a PES The most frequent…", snippet)

	// snippets are taken after the title (if possible, for any of the words)
	text = newSearchText("Kočka a PES\nThe most frequent adjectives modifying the noun pes are starý.", 11)
	ok, snippet := text.match(parseChatSearch("kocka pes"))
	assert.True(t, ok)
	assert.Equal(t, "…modifying the noun pes are starý.", snippet)
	ok, snippet = text.match(parseChatSearch("kocka"))
	assert.True(t, ok)
	assert.Equal(t, "", snippet)
}

func TestChatListSearch(t *testing.T) {
	items := []agent.ChatInfo{{ID: 1, Title: "dogs"}, {ID: 2, Title: "cats"}, {ID: 3, Title: "kočky a psi"}}
	loads := 0
	l := newChatList(items, 0, nil, func() (map[int64]string, error) {
		loads++
		return map[int64]string{
			1: "dogs\nHow frequent is pes in syn2020?",
			2: "cats\nWhat about kočka?",
			3: "kočky a psi\nCompare kočka and pes.",
		}, nil
	})
	typeKeys := func(s string) {
		for _, r := range s {
			l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}, 20)
		}
	}
	ids := func() []int64 {
		var ans []int64
		for _, it := range l.items {
			ans = append(ans, it.ID)
		}
		return ans
	}

	// letters do not search until "/" (e.g. "d" deletes, "q" closes)
	assert.Equal(t, chatListClose, l.Update(keyStr("q"), 20))
	assert.Equal(t, 0, loads)

	l.Update(keyStr("/"), 20)
	typeKeys("kocka")
	assert.Equal(t, []int64{2, 3}, ids())
	assert.Equal(t, 1, loads) // texts are loaded once, on the first search
	typeKeys(" pes")
	assert.Equal(t, []int64{3}, ids())
	assert.Equal(t, "kočky a psi Compare kočka and pes.", l.snippets[3])
	assert.Contains(t, l.View(100, 20), "Compare kočka and pes.")

	// enter keeps the filter; then "d" deletes again
	l.Update(tea.KeyMsg{Type: tea.KeyEnter}, 20)
	assert.False(t, l.searching)
	assert.Equal(t, []int64{3}, ids())
	l.Update(keyStr("d"), 20)
	assert.Equal(t, chatListDelete, l.Update(keyStr("y"), 20))
	l.remove(3)
	assert.Empty(t, ids())
	assert.Contains(t, l.View(100, 20), "no chats match")

	// the first esc clears the search, the second one closes the list
	assert.Equal(t, chatListNone, l.Update(tea.KeyMsg{Type: tea.KeyEsc}, 20))
	assert.Equal(t, []int64{1, 2}, ids())
	assert.Equal(t, chatListClose, l.Update(tea.KeyMsg{Type: tea.KeyEsc}, 20))
	assert.Equal(t, 1, loads)
}

func TestChatListSearchKeepsSelection(t *testing.T) {
	var items []agent.ChatInfo
	texts := map[int64]string{}
	for i := range 5 {
		items = append(items, agent.ChatInfo{ID: int64(i + 1), Title: fmt.Sprintf("chat %d", i+1)})
		texts[int64(i+1)] = fmt.Sprintf("chat %d", i+1)
	}
	texts[4] += " pes"
	texts[2] += " pes"
	l := newChatList(items, 4, nil, func() (map[int64]string, error) { return texts, nil })
	sel, _ := l.selected()
	require.Equal(t, int64(4), sel.ID)
	l.Update(keyStr("/"), 20)
	for _, r := range "pes" {
		l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}, 20)
	}
	sel, _ = l.selected()
	assert.Equal(t, int64(4), sel.ID) // the selected chat still matches
}

func TestChatListSearchLoadError(t *testing.T) {
	l := newChatList([]agent.ChatInfo{{ID: 1}}, 0, nil, func() (map[int64]string, error) {
		return nil, fmt.Errorf("db is locked")
	})
	l.Update(keyStr("/"), 20)
	l.Update(keyStr("x"), 20)
	assert.Contains(t, l.notice, "db is locked")
}
