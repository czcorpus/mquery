package ui

import (
	"regexp"

	"mquery/cmd/mymquery/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// chatLink is a selectable (and clickable) item of the chat transcript
// opening a stored result: either a result line or a result reference
// (`rN` in e.g. `[r1, r3]`) in an answer
type chatLink struct {
	key chatLinkKey
	// position in the rendered transcript (line, column and width in cells)
	line, col, width int
	result           *agent.Result
	// isRef is true for a reference in an answer (false = a result line)
	isRef bool
}

// chatLinkKey identifies a link independently of the rendering (width)
type chatLinkKey struct {
	// entry is an index of the transcript entry
	entry int
	// n is an ordinal number of the link within the entry
	n int
}

var refIDRegexp = regexp.MustCompile(`r\d+`)

// selRefStyle highlights the selected reference in an answer
var selRefStyle = kwicStyle.Background(colSelBg).Underline(true)

// findRefLinks finds result references in rendered lines of an entry
// (the first one is the transcript line `firstLine`). References to unknown
// results (`resolve` returns nil) are not links.
func findRefLinks(
	lines []string, firstLine, entry int, resolve func(id string) *agent.Result,
) []chatLink {
	var ans []chatLink
	for i, line := range lines {
		plain := ansi.Strip(line)
		for _, m := range resultRefRegexp.FindAllStringSubmatchIndex(plain, -1) {
			inner := plain[m[2]:m[3]]
			for _, idm := range refIDRegexp.FindAllStringIndex(inner, -1) {
				id := inner[idm[0]:idm[1]]
				res := resolve(id)
				if res == nil {
					continue
				}
				ans = append(ans, chatLink{
					key:    chatLinkKey{entry: entry, n: len(ans)},
					line:   firstLine + i,
					col:    ansi.StringWidth(plain[:m[2]+idm[0]]),
					width:  len(id),
					result: res,
					isRef:  true,
				})
			}
		}
	}
	return ans
}

// highlightRange renders cells [col, col+width) of a (styled) line with `st`
func highlightRange(line string, col, width int, st lipgloss.Style) string {
	end := col + width
	return ansi.Cut(line, 0, col) +
		st.Render(ansi.Strip(ansi.Cut(line, col, end))) +
		ansi.Cut(line, end, ansi.StringWidth(line))
}

// linkIndex returns the position of the link with `key` (-1 if not found)
func (m *chatModel) linkIndex(key chatLinkKey) int {
	for i, l := range m.links {
		if l.key == key {
			return i
		}
	}
	return -1
}

// selectedLink returns the selected link (when browsing)
func (m *chatModel) selectedLink() (chatLink, bool) {
	if !m.browsing {
		return chatLink{}, false
	}
	if i := m.linkIndex(m.selKey); i >= 0 {
		return m.links[i], true
	}
	return chatLink{}, false
}

// linkAt returns the link at a transcript position
func (m *chatModel) linkAt(line, col int) (chatLink, bool) {
	for _, l := range m.links {
		if l.line == line && col >= l.col && col < l.col+l.width {
			return l, true
		}
	}
	return chatLink{}, false
}

// resolveResult returns a stored result of the session (nil if not found)
func (m *chatModel) resolveResult(id string) *agent.Result {
	if m.env.Chat == nil {
		return nil
	}
	if res, ok := m.env.Chat.Store().Get(id); ok {
		return res
	}
	return nil
}

func openResult(res *agent.Result) tea.Cmd {
	return func() tea.Msg { return openChatResultMsg{result: res} }
}
