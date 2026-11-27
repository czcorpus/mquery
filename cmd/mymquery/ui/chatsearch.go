package ui

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// chatSnippetContext is the number of characters shown around a match
const chatSnippetContext = 30

// searchText is a text prepared for searching: folded versions have
// exactly the same characters (runes) as the original, so positions of
// matches can be used to cut snippets of the original
type searchText struct {
	orig []rune
	// lower is lower-cased
	lower []rune
	// base is lower-cased and without diacritics
	base []rune
	// snippetFrom is the position from which snippets are taken (e.g. after
	// a title, which is shown anyway)
	snippetFrom int
}

// newSearchText prepares a text; snippets are taken from matches at
// `snippetFrom` (in runes) or later
func newSearchText(s string, snippetFrom int) *searchText {
	orig := []rune(s)
	t := &searchText{
		orig: orig, lower: make([]rune, len(orig)), base: make([]rune, len(orig)), snippetFrom: snippetFrom,
	}
	for i, r := range orig {
		t.lower[i] = unicode.ToLower(r)
		t.base[i] = baseRune(t.lower[i])
	}
	return t
}

// baseRune returns a character without diacritics (e.g. č => c)
func baseRune(r rune) rune {
	if r < 128 {
		return r
	}
	for _, d := range norm.NFD.String(string(r)) {
		return d // the base character comes first
	}
	return r
}

// searchWord is a lower-cased word of a search query
type searchWord []rune

// parseChatSearch splits a query into words. Matching is case-insensitive;
// a character with diacritics must match exactly, a character without them
// matches also characters with them (e.g. "kocka" or "kočka" match "Kočka",
// "pěs" does not match "pes").
func parseChatSearch(query string) []searchWord {
	var ans []searchWord
	for _, w := range strings.Fields(query) {
		ans = append(ans, searchWord(strings.ToLower(w)))
	}
	return ans
}

// match returns whether all the words occur in the text (each anywhere)
// and a snippet around the first match of a word after snippetFrom
// ("" if there is none)
func (t *searchText) match(words []searchWord) (bool, string) {
	snippet := ""
	for _, w := range words {
		if t.index(w, 0) < 0 {
			return false, ""
		}
		if pos := t.index(w, t.snippetFrom); snippet == "" && pos >= 0 {
			snippet = t.snippet(pos, len(w))
		}
	}
	return true, snippet
}

// index returns the position (in runes) of the first match of a word
// starting at `from` (or -1)
func (t *searchText) index(w searchWord, from int) int {
	if len(w) == 0 {
		return from
	}
outer:
	for i := from; i+len(w) <= len(t.lower); i++ {
		for j, r := range w {
			if baseRune(r) == r { // no diacritics => any
				if t.base[i+j] != r {
					continue outer
				}
			} else if t.lower[i+j] != r {
				continue outer
			}
		}
		return i
	}
	return -1
}

// snippet returns the original text around [pos, pos+n) on a single line
// (cut at word boundaries)
func (t *searchText) snippet(pos, n int) string {
	from := max(0, pos-chatSnippetContext)
	to := min(len(t.orig), pos+n+chatSnippetContext)
	if from > 0 {
		for i := from; i < pos; i++ {
			if unicode.IsSpace(t.orig[i]) {
				from = i + 1
				break
			}
		}
	}
	if to < len(t.orig) {
		for i := to - 1; i >= pos+n; i-- {
			if unicode.IsSpace(t.orig[i]) {
				to = i
				break
			}
		}
	}
	s := strings.Join(strings.Fields(string(t.orig[from:to])), " ")
	if from > 0 {
		s = "…" + s
	}
	if to < len(t.orig) {
		s += "…"
	}
	return s
}
