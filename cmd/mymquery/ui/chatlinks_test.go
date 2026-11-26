package ui

import (
	"fmt"
	"strings"
	"testing"

	"mquery/cmd/mymquery/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownRefsAreNotWrapped(t *testing.T) {
	md := newMDRenderer()
	text := "The noun is more frequent in fiction than in news texts [r1, r3]."
	for w := 20; w < 70; w++ {
		out := ansi.Strip(md.Render(text, w, false))
		assert.Contains(t, out, "[r1, r3]", "width %d:\n%s", w, out)
	}
	// irregular spacing is normalized
	assert.Contains(t, ansi.Strip(md.Render("see [r1 ,r3]", 40, false)), "[r1, r3]")
}

func TestFindRefLinks(t *testing.T) {
	store := agent.NewStore()
	r1 := store.Add("freqs", nil, "{}", true)
	r2 := store.Add("freqs", nil, "{}", true)
	resolve := func(id string) *agent.Result {
		res, _ := store.Get(id)
		return res
	}
	lines := []string{
		"Dogs " + kwicStyle.Render("[r1]") + " and cats",
		"pes " + kwicStyle.Render("[r2, r9, r1]"),
	}
	links := findRefLinks(lines, 10, 4, resolve)
	require.Len(t, links, 3) // r9 is unknown
	assert.Equal(t, chatLink{key: chatLinkKey{4, 0}, line: 10, col: 6, width: 2, result: r1, isRef: true}, links[0])
	assert.Equal(t, chatLink{key: chatLinkKey{4, 1}, line: 11, col: 5, width: 2, result: r2, isRef: true}, links[1])
	assert.Equal(t, 13, links[2].col) // "pes [r2, r9, " is 13 cells
	assert.Equal(t, r1, links[2].result)
}

func TestHighlightRange(t *testing.T) {
	line := "see " + kwicStyle.Render("[r1, r3]") + " now"
	out := highlightRange(line, 9, 2, selRefStyle)
	assert.Equal(t, ansi.Strip(line), ansi.Strip(out))
	assert.Contains(t, out, selRefStyle.Render("r3"))
}

// newLinkedChat creates a chat with two results and an answer referring to them
func newLinkedChat(t *testing.T) (chatModel, *agent.Result, *agent.Result) {
	env := newTestEnv(t)
	store := agent.NewStore()
	env.Chat = agent.NewSession(blockingBackend{}, &agent.Toolbox{}, store)
	r1 := store.Add("freqs", map[string]any{"corpus_id": "syn", "q": "[lemma=\"pes\"]"}, "{}", true)
	r2 := store.Add("collocations", map[string]any{"corpus_id": "syn", "q": "[lemma=\"pes\"]"}, "{}", true)
	m := newChat(env)
	m.SetSize(80, 30)
	m.add(chatEntry{kind: entryUser, text: "dogs?"})
	m.add(chatEntry{kind: entryResult, result: r1})
	m.add(chatEntry{kind: entryResult, result: r2})
	m.add(chatEntry{kind: entryAssistant, text: "Dogs are frequent [r1], typical collocates are listed in [r2, r7]."})
	return m, r1, r2
}

func openedResult(t *testing.T, cmd tea.Cmd) *agent.Result {
	t.Helper()
	require.NotNil(t, cmd)
	msg, ok := cmd().(openChatResultMsg)
	require.True(t, ok)
	return msg.result
}

func TestChatBrowseLinks(t *testing.T) {
	m, r1, r2 := newLinkedChat(t)
	// two result lines + [r1] + r2 in [r2, r7] (r7 does not exist)
	require.Len(t, m.links, 4)

	// tab selects the last reference of the answer
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.True(t, m.browsing)
	sel, ok := m.selectedLink()
	require.True(t, ok)
	assert.True(t, sel.isRef)
	assert.Equal(t, r2, sel.result)
	view := m.vp.View()
	assert.Contains(t, view, selRefStyle.Render("r2"))
	assert.Contains(t, ansi.Strip(view), "[r2, r7]")

	// ↑ moves backwards through the references and result lines
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	sel, _ = m.selectedLink()
	assert.True(t, sel.isRef)
	assert.Equal(t, r1, sel.result)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	sel, _ = m.selectedLink()
	assert.False(t, sel.isRef)
	assert.Equal(t, r2, sel.result)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, r2, openedResult(t, cmd))

	// the selection is kept when the transcript is re-rendered (e.g. resized)
	m.SetSize(50, 30)
	sel, _ = m.selectedLink()
	assert.Equal(t, r2, sel.result)
	assert.False(t, sel.isRef)
}

func TestChatClickLink(t *testing.T) {
	m, r1, _ := newLinkedChat(t)
	var ref chatLink
	for _, l := range m.links {
		if l.isRef && l.result == r1 {
			ref = l
		}
	}
	require.True(t, ref.isRef)
	click := func(x, y int) tea.Cmd {
		_, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		return cmd
	}
	y := ref.line - m.vp.YOffset
	assert.Equal(t, r1, openedResult(t, click(ref.col+1, y)))
	assert.Nil(t, click(ref.col+ref.width, y)) // the closing bracket
	assert.Nil(t, click(ref.col, -1))
	// a result line is clickable on its whole width
	res := m.links[0]
	assert.Equal(t, res.result, openedResult(t, click(40, res.line-m.vp.YOffset)))
}

func TestChatLinkPositionsMatchView(t *testing.T) {
	m, _, _ := newLinkedChat(t)
	lines := strings.Split(ansi.Strip(m.vp.View()), "\n")
	for _, l := range m.links {
		if !l.isRef {
			continue
		}
		row := l.line - m.vp.YOffset
		require.True(t, row >= 0 && row < len(lines))
		assert.Equal(t, l.result.ID, ansi.Cut(lines[row], l.col, l.col+l.width), fmt.Sprint(l))
	}
}

// cmdMsgTypes runs a command (incl. batches) and returns types of the messages
func cmdMsgTypes(cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var ans []string
		for _, c := range batch {
			ans = append(ans, cmdMsgTypes(c)...)
		}
		return ans
	}
	return []string{fmt.Sprintf("%T", msg)}
}

func TestAppMouseOnlyInChat(t *testing.T) {
	env := newTestEnv(t)
	env.Chat = agent.NewSession(blockingBackend{}, &agent.Toolbox{}, agent.NewStore())
	app := NewApp(env)
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	chatIdx := -1
	for i, f := range app.features {
		if f.Chat {
			chatIdx = i
		}
	}
	require.GreaterOrEqual(t, chatIdx, 0)

	_, cmd := app.Update(featureSelectedMsg{idx: chatIdx})
	assert.Contains(t, cmdMsgTypes(cmd), "tea.enableMouseCellMotionMsg")
	_, cmd = app.Update(backMsg{})
	assert.Contains(t, cmdMsgTypes(cmd), "tea.disableMouseMsg")
	_, cmd = app.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	assert.Empty(t, cmdMsgTypes(cmd))
}
