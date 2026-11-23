package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/czcorpus/mquery-common/corp"
)

// corpusAttr is a positional attribute info (name, size, description)
type corpusAttr = corp.Attr

func attrNames(attrs []corpusAttr) []string {
	if attrs == nil {
		return nil
	}
	ans := make([]string, len(attrs))
	for i, a := range attrs {
		ans[i] = a.Name
	}
	return ans
}

// viewSeq provides unique view IDs so that async responses
// can be matched to the view which requested them
var viewSeq int

func nextViewID() int {
	viewSeq++
	return viewSeq
}

// pushResultMsg asks the app to open a nested result view (e.g. a concordance
// for a frequency item). `esc` in the nested view returns to the current one.
type pushResultMsg struct{ view ResultView }

func pushResult(v ResultView) tea.Cmd {
	return func() tea.Msg { return pushResultMsg{view: v} }
}

// corpusAttrsMsg delivers positional attributes of a corpus (in the corpus order)
type corpusAttrsMsg struct {
	viewID   int
	corpusID string
	attrs    []corpusAttr
	err      error
}

// fetchCorpusAttrs loads positional attributes of a corpus via /info.
// The result should be stored to env.attrCache by the receiver.
func fetchCorpusAttrs(env *Env, ctx context.Context, corpusID string, viewID int) tea.Cmd {
	client, lang := env.Client, env.Lang
	return func() tea.Msg {
		info, err := client.CorpusInfo(ctx, corpusID, lang)
		if err != nil {
			return corpusAttrsMsg{viewID: viewID, corpusID: corpusID, err: err}
		}
		return corpusAttrsMsg{viewID: viewID, corpusID: corpusID, attrs: info.AttrList}
	}
}

// listNav is a cursor + scroll position of a list with single-line rows
type listNav struct {
	cursor int
	scroll int
}

// handleKey applies a navigation key; returns false if the key is not a navigation one
func (n *listNav) handleKey(key string, size, height int) bool {
	switch key {
	case "up", "k":
		n.cursor--
	case "down", "j":
		n.cursor++
	case "pgup", "ctrl+u":
		n.cursor -= height
	case "pgdown", "ctrl+d":
		n.cursor += height
	case "home", "g":
		n.cursor = 0
	case "end", "G":
		n.cursor = size - 1
	default:
		return false
	}
	n.fix(size, height)
	return true
}

// fix keeps the cursor within the list and visible
func (n *listNav) fix(size, height int) {
	n.cursor = max(0, min(n.cursor, size-1))
	if n.cursor < n.scroll {
		n.scroll = n.cursor
	}
	if n.cursor >= n.scroll+height {
		n.scroll = n.cursor - height + 1
	}
	n.scroll = max(0, min(n.scroll, size-height))
}

func (n *listNav) reset() {
	n.cursor, n.scroll = 0, 0
}

// visibleRange returns [from, to) indices of rows to be rendered
func (n *listNav) visibleRange(size, height int) (int, int) {
	return n.scroll, min(size, n.scroll+height)
}
