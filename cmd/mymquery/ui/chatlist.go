package ui

import (
	"fmt"
	"strings"

	"mquery/cmd/mymquery/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// chatListMaxItems limits the number of listed saved chats
const chatListMaxItems = 500

// chatList is a list of saved chats shown instead of the chat transcript
type chatList struct {
	items []agent.ChatInfo
	nav   listNav
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

func newChatList(items []agent.ChatInfo, currentID int64, err error) *chatList {
	l := &chatList{items: items, currentID: currentID, err: err}
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
	for i, it := range l.items {
		if it.ID == id {
			l.items = append(l.items[:i], l.items[i+1:]...)
			break
		}
	}
	if id == l.currentID {
		l.currentID = 0
	}
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
	switch msg.String() {
	case "esc", "ctrl+o", "q":
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

// rowsHeight is the number of list rows fitting the height
// (minus the title, a gap and the status line)
func (l *chatList) rowsHeight(height int) int {
	return max(1, height-3)
}

func (l *chatList) View(width, height int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Saved chats") + "\n\n")
	if l.err != nil {
		b.WriteString(errorStyle.Render(ansi.Wordwrap("failed to load saved chats: "+l.err.Error(), width, " ")))
		return b.String()
	}
	if len(l.items) == 0 {
		b.WriteString(mutedStyle.Render("no saved chats yet"))
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
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%d/%d", l.nav.cursor+1, len(l.items))))
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
	return "↑/↓ move • enter open • d delete • esc back to the chat"
}
