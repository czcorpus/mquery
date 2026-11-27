package ui

import (
	"fmt"
	"slices"
	"strings"

	"mquery/cmd/mymquery/agent"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// chatListMaxItems limits the number of listed saved chats
const chatListMaxItems = 500

// chatList is a list of saved chats shown instead of the chat transcript.
// It can be filtered by a search query (see parseChatSearch).
type chatList struct {
	// all are all the listed chats, items are those matching the search
	all   []agent.ChatInfo
	items []agent.ChatInfo
	nav   listNav

	// loadTexts provides texts of chats for searching (loaded lazily)
	loadTexts func() (map[int64]string, error)
	texts     map[int64]*searchText
	search    textinput.Model
	// searching is true while the search query is being edited
	searching bool
	// query is the applied search query; snippets show where chats match
	query    string
	snippets map[int64]string

	// currentID is the chat open in the chat screen (0 = none)
	currentID int64
	// confirmDel is true while waiting for a confirmation of a deletion
	confirmDel bool
	err        error
	// notice is an error of the last action (e.g. a failed deletion)
	notice string
}

// chatListAction is what the user asked for in the list
type chatListAction int

const (
	chatListNone chatListAction = iota
	chatListClose
	chatListOpen
	chatListDelete
)

func newChatList(
	items []agent.ChatInfo, currentID int64, err error, loadTexts func() (map[int64]string, error),
) *chatList {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "words to search for"
	ti.Cursor.SetMode(cursor.CursorStatic) // no blinking => no commands needed
	l := &chatList{
		all: items, items: items, currentID: currentID, err: err, loadTexts: loadTexts, search: ti,
	}
	for i, it := range items {
		if it.ID == currentID {
			l.nav.cursor = i
		}
	}
	return l
}

func (l *chatList) selected() (agent.ChatInfo, bool) {
	if l.nav.cursor < 0 || l.nav.cursor >= len(l.items) {
		return agent.ChatInfo{}, false
	}
	return l.items[l.nav.cursor], true
}

// remove drops a deleted chat from the list
func (l *chatList) remove(id int64) {
	without := func(items []agent.ChatInfo) []agent.ChatInfo {
		return slices.DeleteFunc(items, func(it agent.ChatInfo) bool { return it.ID == id })
	}
	// (items may share the backing array with all - DeleteFunc works in place)
	l.items = without(slices.Clone(l.items))
	l.all = without(l.all)
	delete(l.texts, id)
	if id == l.currentID {
		l.currentID = 0
	}
}

// setQuery filters the chats: each word of the query must occur in a chat
// (case-insensitive; a word without diacritics matches also words with them)
func (l *chatList) setQuery(query string) {
	if query == l.query {
		return
	}
	l.query = query
	selected, _ := l.selected()
	words := parseChatSearch(query)
	if len(words) == 0 {
		l.items, l.snippets = l.all, nil

	} else {
		if err := l.ensureTexts(); err != nil {
			l.notice = "failed to load chats for searching: " + err.Error()
			return
		}
		l.items, l.snippets = nil, make(map[int64]string)
		for _, it := range l.all {
			if t, ok := l.texts[it.ID]; ok {
				if ok, snippet := t.match(words); ok {
					l.items = append(l.items, it)
					l.snippets[it.ID] = snippet
				}
			}
		}
	}
	// keep the selected chat if it still matches
	l.nav.cursor = max(0, slices.IndexFunc(l.items, func(it agent.ChatInfo) bool { return it.ID == selected.ID }))
}

func (l *chatList) ensureTexts() error {
	if l.texts != nil {
		return nil
	}
	if l.loadTexts == nil {
		l.texts = make(map[int64]*searchText)
		return nil
	}
	texts, err := l.loadTexts()
	if err != nil {
		return err
	}
	// titles are shown, so snippets are taken from the conversations
	// (the texts start with the title, see DB.ChatTexts)
	titleLen := make(map[int64]int, len(l.all))
	for _, it := range l.all {
		titleLen[it.ID] = len([]rune(it.Title))
	}
	l.texts = make(map[int64]*searchText, len(texts))
	for id, s := range texts {
		l.texts[id] = newSearchText(s, titleLen[id])
	}
	return nil
}

func (l *chatList) Update(msg tea.KeyMsg, height int) chatListAction {
	l.notice = ""
	if l.confirmDel {
		l.confirmDel = false
		if msg.String() == "y" {
			return chatListDelete
		}
		return chatListNone
	}
	if l.searching {
		return l.updateSearch(msg, height)
	}
	switch msg.String() {
	case "/":
		l.searching = true
		l.search.Focus()
	case "esc":
		if l.query != "" { // the first esc clears the search
			l.search.SetValue("")
			l.setQuery("")
			return chatListNone
		}
		return chatListClose
	case "ctrl+o", "q":
		return chatListClose
	case "enter":
		if _, ok := l.selected(); ok {
			return chatListOpen
		}
	case "d", "delete":
		if _, ok := l.selected(); ok {
			l.confirmDel = true
		}
	default:
		l.nav.handleKey(msg.String(), len(l.items), l.rowsHeight(height))
	}
	return chatListNone
}

func (l *chatList) updateSearch(msg tea.KeyMsg, height int) chatListAction {
	switch msg.String() {
	case "esc":
		l.search.SetValue("")
		l.setQuery("")
		fallthrough
	case "enter":
		l.searching = false
		l.search.Blur()
		return chatListNone
	case "up", "down", "pgup", "pgdown":
		l.nav.handleKey(msg.String(), len(l.items), l.rowsHeight(height))
		return chatListNone
	}
	l.search, _ = l.search.Update(msg)
	l.setQuery(l.search.Value())
	return chatListNone
}

// rowsHeight is the number of list rows fitting the height
// (minus the title, a gap and the status line)
func (l *chatList) rowsHeight(height int) int {
	return max(1, height-3)
}

func (l *chatList) View(width, height int) string {
	var b strings.Builder
	title := titleStyle.Render("Saved chats")
	if l.searching || l.query != "" {
		// (the input is the width + its prompt + a cursor cell)
		l.search.Width = max(10, width-lipgloss.Width(title)-4-lipgloss.Width(l.search.Prompt)-1)
		title += "    " + l.search.View()
	}
	b.WriteString(ansi.Truncate(title, width, "…") + "\n\n")
	if l.err != nil {
		b.WriteString(errorStyle.Render(ansi.Wordwrap("failed to load saved chats: "+l.err.Error(), width, " ")))
		return b.String()
	}
	if len(l.all) == 0 {
		b.WriteString(mutedStyle.Render("no saved chats yet"))
		return b.String()
	}
	if len(l.items) == 0 {
		msg := "no chats match (each word must occur in a chat)"
		if l.notice != "" {
			msg = l.notice
		}
		b.WriteString(mutedStyle.Render(msg))
		return b.String()
	}
	rows := l.rowsHeight(height)
	l.nav.fix(len(l.items), rows)
	from, to := l.nav.visibleRange(len(l.items), rows)
	for i := from; i < to; i++ {
		b.WriteString(l.renderRow(l.items[i], i == l.nav.cursor, width) + "\n")
	}
	// keep the status line at the bottom
	for i := to - from; i < rows; i++ {
		b.WriteString("\n")
	}
	if l.notice != "" {
		b.WriteString(errorStyle.Render(ansi.Truncate(l.notice, width, "…")))
	} else if l.confirmDel {
		it, _ := l.selected()
		b.WriteString(errorStyle.Render(ansi.Truncate(
			fmt.Sprintf("delete %q? y = yes, any other key = no", it.Title), width, "…")))
	} else {
		status := fmt.Sprintf("%d/%d", l.nav.cursor+1, len(l.items))
		if len(l.items) != len(l.all) {
			status += fmt.Sprintf(" (matching, of %d)", len(l.all))
		}
		b.WriteString(mutedStyle.Render(status))
	}
	return b.String()
}

func (l *chatList) renderRow(it agent.ChatInfo, selected bool, width int) string {
	marker := "  "
	if selected {
		marker = accentStyle.Render("❯ ")
	}
	current := "  "
	if it.ID == l.currentID {
		current = accentStyle.Render("● ")
	}
	date := mutedStyle.Render(it.Updated.Format("2006-01-02 15:04"))
	var details []string
	if it.CorpusID != "" {
		details = append(details, it.CorpusID)
	}
	if it.Model != "" {
		details = append(details, it.Model)
	}
	details = append(details, formatInt(it.Usage.InputTokens+it.Usage.OutputTokens)+" tokens")
	if snippet := l.snippets[it.ID]; snippet != "" {
		// (where the chat matches is more important when searching)
		details = []string{snippet}
	}
	title := it.Title
	if title == "" {
		title = "(untitled)"
	}
	// the title is truncated first so the details stay visible
	detailsStr := strings.Join(details, " · ")
	prefix := marker + current + date + "  "
	titleW := max(20, width-lipgloss.Width(prefix)-2-lipgloss.Width(detailsStr))
	line := prefix + boldStyle.Render(ansi.Truncate(title, titleW, "…")) + "  " + mutedStyle.Render(detailsStr)
	line = ansi.Truncate(line, width, "…")
	if selected {
		line = lipgloss.NewStyle().Background(colSelBg).Width(width).Render(line)
	}
	return line
}

func (l *chatList) Help() string {
	if l.confirmDel {
		return "y confirm deletion • any other key cancels"
	}
	if l.searching {
		return "type words to search for (all must occur in a chat) • ↑/↓ move • enter done • esc clear"
	}
	if l.query != "" {
		return "↑/↓ move • enter open • / edit search • d delete • esc clear search"
	}
	return "↑/↓ move • enter open • / search • d delete • esc back to the chat"
}
