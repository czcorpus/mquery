package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/llm"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	chatInputHeight = 3
	// chatMaxArgWidth limits displayed tool argument values
	chatMaxArgWidth = 60
)

type chatEntryKind int

const (
	entryUser chatEntryKind = iota
	entryAssistant
	// entryInterim is an assistant's text produced along with tool calls
	entryInterim
	entryResult
	entryError
	entryInfo
)

type chatEntry struct {
	kind   chatEntryKind
	text   string
	result *agent.Result
}

// chatEventMsg delivers a progress event of a running turn
type chatEventMsg struct {
	runID int
	event agent.Event
	ch    <-chan tea.Msg
}

// chatDoneMsg finishes a running turn
type chatDoneMsg struct {
	runID int
	ans   agent.TurnResult
	err   error
}

// openChatResultMsg asks the app to show a stored result
type openChatResultMsg struct {
	result *agent.Result
}

type chatModel struct {
	env     *Env
	entries []chatEntry
	input   textarea.Model
	vp      viewport.Model
	spinner spinner.Model
	width   int
	height  int

	// browsing is true if the transcript (instead of the input) is focused;
	// then `sel` is an index of the selected result entry
	browsing bool
	sel      int
	// resultLines maps entry index => first line in the rendered transcript
	resultLines map[int]int

	running bool
	runID   int
	cancel  context.CancelFunc
	// activity describes what the running turn is doing right now
	activity string
	// canRetry is true if the last turn failed and can be retried
	canRetry bool
	// list shows saved chats instead of the transcript (nil = not shown)
	list *chatList
	// persistWarned is true if the running turn already reported
	// a failure to save the conversation
	persistWarned bool
}

func newChat(env *Env) chatModel {
	ta := textarea.New()
	ta.Placeholder = "Ask about the corpora…"
	ta.ShowLineNumbers = false
	ta.Prompt = "┃ "
	ta.CharLimit = 4000
	ta.SetHeight(chatInputHeight)
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Prompt = accentStyle
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return chatModel{
		env:     env,
		input:   ta,
		vp:      viewport.New(0, 0),
		spinner: sp,
	}
}

// Activate is called when the chat screen is entered; the input is focused
// unless the user was browsing results
func (m *chatModel) Activate() tea.Cmd {
	if m.browsing || m.list != nil {
		return nil
	}
	return m.input.Focus()
}

func (m *chatModel) Blur() {
	m.input.Blur()
}

// Close cancels a running turn (if any)
func (m *chatModel) Close() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *chatModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.input.SetWidth(max(10, w-4))
	m.vp.Width = max(10, w-4)
	m.vp.Height = max(1, h-chatInputHeight-2)
	m.refresh(false)
}

// resultIdxs returns indices of result entries
func (m *chatModel) resultIdxs() []int {
	var ans []int
	for i, e := range m.entries {
		if e.kind == entryResult {
			ans = append(ans, i)
		}
	}
	return ans
}

func (m *chatModel) add(e chatEntry) {
	m.entries = append(m.entries, e)
	m.refresh(true)
}

// submit sends the input (or with `retry`, it continues the last failed turn)
func (m *chatModel) submit(corpusID, subc string, retry bool) tea.Cmd {
	if m.running || (retry && !m.canRetry) {
		return nil
	}
	sess := m.env.Chat
	opts := agent.PromptOptions{CorpusID: corpusID, Subcorpus: subc}
	var run func(ctx context.Context, emit func(agent.Event)) (agent.TurnResult, error)
	if retry {
		m.add(chatEntry{kind: entryInfo, text: "↻ retrying (completed steps are not repeated)"})
		run = func(ctx context.Context, emit func(agent.Event)) (agent.TurnResult, error) {
			return sess.Retry(ctx, opts, emit)
		}

	} else {
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return nil
		}
		m.input.Reset()
		m.add(chatEntry{kind: entryUser, text: text})
		run = func(ctx context.Context, emit func(agent.Event)) (agent.TurnResult, error) {
			return sess.Run(ctx, text, opts, emit)
		}
	}
	m.canRetry = false
	m.persistWarned = false
	m.running = true
	m.activity = "thinking"
	m.runID++
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan tea.Msg)
	runID := m.runID
	// Note: the sends below must not give up on a cancelled ctx (a `select`
	// with ctx.Done() would randomly drop the final message after esc and
	// the chat would wait forever). The UI keeps reading the channel until
	// chatDoneMsg arrives; the only case nobody listens is the app exit.
	go func() {
		defer close(ch)
		emit := func(e agent.Event) {
			ch <- chatEventMsg{runID: runID, event: e, ch: ch}
		}
		ans, err := run(ctx, emit)
		ch <- chatDoneMsg{runID: runID, ans: ans, err: err}
	}()
	m.refresh(true)
	return tea.Batch(m.spinner.Tick, waitChat(ch))
}

func waitChat(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m chatModel) Update(msg tea.Msg) (chatModel, tea.Cmd) {
	switch msg := msg.(type) {
	case chatEventMsg:
		if msg.runID != m.runID {
			return m, waitChat(msg.ch) // keep draining so the sender is never blocked
		}
		switch e := msg.event.(type) {
		case agent.AssistantText:
			m.add(chatEntry{kind: entryInterim, text: e.Text})
		case agent.ToolCallStarted:
			m.activity = "running " + e.Tool
			m.refresh(true)
		case agent.ToolCallDone:
			m.activity = "thinking"
			m.add(chatEntry{kind: entryResult, result: e.Result})
		case agent.PersistFailed:
			if !m.persistWarned {
				m.persistWarned = true
				m.add(chatEntry{kind: entryError, text: "failed to save the conversation: " + e.Err.Error()})
			}
		}
		return m, waitChat(msg.ch)

	case chatDoneMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		m.running = false
		m.cancel()
		m.cancel = nil
		if msg.err != nil {
			m.canRetry = true
			const hint = " - ctrl+r retries the request, a new message discards it"
			if errors.Is(msg.err, context.Canceled) {
				m.add(chatEntry{kind: entryInfo, text: "(cancelled" + hint + ")"})
			} else {
				m.add(chatEntry{kind: entryError, text: msg.err.Error() + hint})
			}
			return m, nil
		}
		text := msg.ans.Text
		if text == "" {
			text = "(empty answer)"
		}
		m.add(chatEntry{kind: entryAssistant, text: text})
		return m, nil

	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.refresh(false)
		return m, cmd

	case tea.KeyMsg:
		if m.list != nil {
			return m.updateList(msg)
		}
		if m.browsing {
			return m.updateBrowsing(msg)
		}
		return m.updateInput(msg)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg) // cursor blinking
	return m, cmd
}

func (m chatModel) updateInput(msg tea.KeyMsg) (chatModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m, func() tea.Msg { return chatSubmitMsg{} }
	case "ctrl+r":
		if m.canRetry && !m.running {
			return m, func() tea.Msg { return chatSubmitMsg{retry: true} }
		}
		return m, nil
	case "esc":
		if m.running {
			m.cancel()
			return m, nil
		}
		return m, goBack
	case "tab":
		if idxs := m.resultIdxs(); len(idxs) > 0 {
			m.browsing = true
			m.input.Blur()
			m.sel = idxs[len(idxs)-1]
			m.refresh(false)
			m.scrollToSel()
		}
		return m, nil
	case "ctrl+n":
		if m.running {
			return m, nil
		}
		saved := m.env.Chat.ChatID() != 0
		m.newConversation()
		if saved {
			m.add(chatEntry{kind: entryInfo, text: "── new conversation (the previous one is saved) ──"})
		} else {
			m.add(chatEntry{kind: entryInfo, text: "── new conversation ──"})
		}
		return m, nil
	case "ctrl+o":
		if m.running {
			return m, nil
		}
		if m.env.DB == nil {
			m.add(chatEntry{kind: entryInfo, text: "chats are not saved (the -db option is empty)"})
			return m, nil
		}
		items, err := m.env.DB.ListChats(chatListMaxItems)
		m.list = newChatList(items, m.env.Chat.ChatID(), err)
		m.input.Blur()
		return m, nil
	case "pgup", "pgdown":
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// newConversation resets the session and clears the transcript
func (m *chatModel) newConversation() {
	m.env.Chat.Reset()
	m.canRetry = false
	m.browsing = false
	m.entries = nil
	m.resultLines = nil
	m.vp.SetYOffset(0)
}

func (m chatModel) updateList(msg tea.KeyMsg) (chatModel, tea.Cmd) {
	switch m.list.Update(msg, m.listHeight()) {
	case chatListClose:
		m.list = nil
		m.refresh(false)
		return m, m.input.Focus()

	case chatListOpen:
		info, _ := m.list.selected()
		if info.ID != m.env.Chat.ChatID() {
			chat, err := m.env.DB.LoadChat(info.ID)
			if err != nil {
				m.list.notice = "failed to load the chat: " + err.Error()
				return m, nil
			}
			m.openChat(chat)
		}
		m.list = nil
		return m, m.input.Focus()

	case chatListDelete:
		info, _ := m.list.selected()
		if err := m.env.DB.DeleteChat(info.ID); err != nil {
			m.list.notice = "failed to delete the chat: " + err.Error()
			return m, nil
		}
		m.list.remove(info.ID)
		if info.ID == m.env.Chat.ChatID() {
			m.newConversation()
			m.add(chatEntry{kind: entryInfo, text: "── the chat was deleted, new conversation ──"})
		}
	}
	return m, nil
}

func (m *chatModel) listHeight() int {
	return max(1, m.height)
}

// openChat loads a saved chat into the session and rebuilds the transcript
func (m *chatModel) openChat(chat agent.SavedChat) {
	m.newConversation()
	sess := m.env.Chat
	sess.Load(chat)
	m.entries = chatEntriesFromMessages(sess.Messages(), sess.Store())
	m.entries = append([]chatEntry{{
		kind: entryInfo,
		text: fmt.Sprintf("── saved chat from %s ──", chat.Info.Created.Format("2006-01-02 15:04")),
	}}, m.entries...)
	if sess.CanRetry() {
		m.canRetry = true
		m.entries = append(m.entries, chatEntry{
			kind: entryInfo,
			text: "(the last request did not finish - ctrl+r retries it, a new message discards it)",
		})
	}
	m.refresh(true)
}

// chatEntriesFromMessages creates a transcript of a conversation
func chatEntriesFromMessages(msgs []llm.Message, store *agent.Store) []chatEntry {
	var ans []chatEntry
	for _, msg := range msgs {
		switch msg.Role {
		case llm.RoleUser:
			ans = append(ans, chatEntry{kind: entryUser, text: msg.Content})
		case llm.RoleAssistant:
			text := strings.TrimSpace(msg.Content)
			if len(msg.ToolCalls) > 0 {
				if text != "" {
					ans = append(ans, chatEntry{kind: entryInterim, text: text})
				}
			} else {
				if text == "" {
					text = "(empty answer)"
				}
				ans = append(ans, chatEntry{kind: entryAssistant, text: text})
			}
		case llm.RoleTool:
			if res, ok := store.Get(agent.ResultIDOf(msg.ToolResult.Content)); ok {
				ans = append(ans, chatEntry{kind: entryResult, result: res})
			}
		}
	}
	return ans
}

func (m chatModel) updateBrowsing(msg tea.KeyMsg) (chatModel, tea.Cmd) {
	idxs := m.resultIdxs()
	pos := slices.Index(idxs, m.sel)
	switch msg.String() {
	case "tab", "esc", "i":
		m.browsing = false
		m.refresh(false)
		return m, m.input.Focus()
	case "q":
		return m, tea.Quit
	case "up", "k":
		if pos > 0 {
			m.sel = idxs[pos-1]
		}
	case "down", "j":
		if pos >= 0 && pos < len(idxs)-1 {
			m.sel = idxs[pos+1]
		}
	case "home", "g":
		m.sel = idxs[0]
	case "end", "G":
		m.sel = idxs[len(idxs)-1]
	case "enter":
		if pos >= 0 {
			res := m.entries[m.sel].result
			return m, func() tea.Msg { return openChatResultMsg{result: res} }
		}
	case "pgup", "pgdown":
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	m.refresh(false)
	m.scrollToSel()
	return m, nil
}

func (m *chatModel) scrollToSel() {
	line, ok := m.resultLines[m.sel]
	if !ok {
		return
	}
	if line < m.vp.YOffset {
		m.vp.SetYOffset(line)
	} else if line >= m.vp.YOffset+m.vp.Height {
		m.vp.SetYOffset(line - m.vp.Height + 1)
	}
}

// refresh re-renders the transcript; with `follow`, it is scrolled to the end
func (m *chatModel) refresh(follow bool) {
	if m.vp.Width == 0 {
		return
	}
	w := m.vp.Width
	m.resultLines = make(map[int]int)
	var lines []string
	for i, e := range m.entries {
		if e.kind == entryUser || (e.kind == entryResult && (i == 0 || m.entries[i-1].kind != entryResult)) ||
			e.kind == entryAssistant {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
		}
		if e.kind == entryResult {
			m.resultLines[i] = len(lines)
		}
		lines = append(lines, strings.Split(m.renderEntry(i, e, w), "\n")...)
	}
	if m.running {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.spinner.View()+mutedStyle.Render(m.activity+"… (esc to cancel)"))
	}
	if len(m.entries) == 0 && !m.running {
		lines = append(lines, mutedStyle.Render(ansi.Wordwrap(
			"Ask a question about the corpora - e.g. \"What are the most frequent "+
				"adjectives modifying the noun 'house'?\". The model queries MQuery and each "+
				"result it obtains can be opened as an interactive table.", w, " ")))
	}
	m.vp.SetContent(strings.Join(lines, "\n"))
	if follow {
		m.vp.GotoBottom()
	}
}

var resultRefRegexp = regexp.MustCompile(`\[(r\d+(?:\s*,\s*r\d+)*)\]`)

func (m *chatModel) renderEntry(idx int, e chatEntry, w int) string {
	switch e.kind {
	case entryUser:
		return accentStyle.Render("❯ ") + boldStyle.Render(ansi.Wordwrap(e.text, w-2, " "))
	case entryAssistant:
		text := ansi.Wordwrap(e.text, w, " ")
		return resultRefRegexp.ReplaceAllStringFunc(text, func(s string) string {
			return kwicStyle.Render(s)
		})
	case entryInterim:
		return mutedStyle.Render(ansi.Wordwrap(e.text, w, " "))
	case entryError:
		return errorStyle.Render(ansi.Wordwrap(e.text, w, " "))
	case entryInfo:
		return mutedStyle.Render(ansi.Wordwrap(e.text, w, " "))
	case entryResult:
		return m.renderResult(idx, e.result, w)
	}
	return ""
}

func (m *chatModel) renderResult(idx int, res *agent.Result, w int) string {
	selected := m.browsing && idx == m.sel
	marker := "  "
	if selected {
		marker = accentStyle.Render("❯ ")
	}
	id := kwicStyle.Render(res.ID)
	tool := res.Tool
	if !res.OK {
		tool = errorStyle.Render(tool + " (failed)")
	}
	line := marker + "⚙ " + id + " " + tool + " " + mutedStyle.Render(formatToolArgs(res.Args))
	line = ansi.Truncate(line, w, "…")
	if selected {
		line = lipgloss.NewStyle().Background(colSelBg).Width(w).Render(line)
	}
	return line
}

// formatToolArgs formats arguments as `name=value` (corpus and query first)
func formatToolArgs(args map[string]any) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		if k != "corpus_id" && k != "q" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range []string{"q", "corpus_id"} {
		if _, ok := args[k]; ok {
			keys = append([]string{k}, keys...)
		}
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		var val string
		if s, ok := args[k].(string); ok {
			val = s
		} else {
			data, _ := json.Marshal(args[k])
			val = string(data)
		}
		parts[i] = k + "=" + ansi.Truncate(val, chatMaxArgWidth, "…")
	}
	return strings.Join(parts, " ")
}

func (m chatModel) View() string {
	if m.list != nil {
		return m.list.View(m.vp.Width, m.listHeight())
	}
	input := m.input.View()
	if m.browsing {
		input = mutedStyle.Render(input)
	}
	return m.vp.View() + "\n\n" + input
}

func (m chatModel) Help() string {
	if m.list != nil {
		return m.list.Help()
	}
	if m.browsing {
		return "↑/↓ select result • enter open • pgup/pgdn scroll • tab/esc back to input • q quit"
	}
	if m.running {
		return "esc cancel • pgup/pgdn scroll"
	}
	retry := ""
	if m.canRetry {
		retry = " • ctrl+r retry"
	}
	return "enter send" + retry + " • alt+enter newline • tab browse results • pgup/pgdn scroll • ctrl+n new conversation • ctrl+o saved chats • esc menu"
}

// chatSubmitMsg asks the app to submit the chat input or to retry the last
// failed turn (the app adds context, e.g. the corpus)
type chatSubmitMsg struct {
	retry bool
}

// chatUsageInfo returns model and token usage info for the header
func chatUsageInfo(env *Env) string {
	if env.Chat == nil {
		return ""
	}
	u := env.Chat.Usage()
	return fmt.Sprintf("%s · tokens in %s / out %s",
		env.Chat.Backend().ModelName(), formatInt(u.InputTokens), formatInt(u.OutputTokens))
}
