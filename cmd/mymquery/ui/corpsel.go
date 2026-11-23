package ui

import (
	"context"
	"fmt"
	"strings"

	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type corpusSelectedMsg struct{ corpus api.CorpusItem }

type corplistLoadedMsg struct {
	corpora []api.CorpusItem
	err     error
}

type corpusListItem struct{ c api.CorpusItem }

func (i corpusListItem) Title() string {
	if i.c.FullName != "" && i.c.FullName != i.c.ID {
		return fmt.Sprintf("%s  (%s)", i.c.FullName, i.c.ID)
	}
	return i.c.ID
}

func (i corpusListItem) Description() string {
	parts := make([]string, 0, 2)
	if i.c.Language != "" {
		parts = append(parts, "["+i.c.Language+"]")
	}
	if i.c.Description != "" {
		parts = append(parts, i.c.Description)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func (i corpusListItem) FilterValue() string {
	return i.c.ID + " " + i.c.FullName + " " + i.c.Language
}

// filterCorpora returns corpora containing query (case-insensitive infix)
// in their ID, full name or language. Exact ID/name matches go first.
func filterCorpora(corpora []api.CorpusItem, query string) []api.CorpusItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return corpora
	}
	var exact, rest []api.CorpusItem
	for _, c := range corpora {
		id, name := strings.ToLower(c.ID), strings.ToLower(c.FullName)
		switch {
		case id == q || name == q:
			exact = append(exact, c)
		case strings.Contains(id, q) || strings.Contains(name, q) ||
			strings.Contains(strings.ToLower(c.Language), q):
			rest = append(rest, c)
		}
	}
	return append(exact, rest...)
}

type corpSelModel struct {
	env     *Env
	list    list.Model
	filter  textinput.Model
	corpora []api.CorpusItem
	spinner spinner.Model
	loading bool
	err     error
}

// corpSelHeaderHeight is the number of lines above the list (title, filter, gap)
const corpSelHeaderHeight = 3

func newCorpSel(env *Env) corpSelModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false) // we filter ourselves via m.filter
	l.DisableQuitKeybindings()
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = accentStyle
	ti.Placeholder = "type to filter (ID, name, language)"
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.Focus()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return corpSelModel{env: env, list: l, filter: ti, spinner: sp, loading: true}
}

func (m corpSelModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.load())
}

func (m corpSelModel) load() tea.Cmd {
	client, lang := m.env.Client, m.env.Lang
	return func() tea.Msg {
		corpora, err := client.Corplist(context.Background(), lang)
		return corplistLoadedMsg{corpora: corpora, err: err}
	}
}

func (m *corpSelModel) SetSize(w, h int) {
	m.list.SetSize(w, max(1, h-corpSelHeaderHeight))
	m.filter.Width = max(10, w-6)
}

// Filtering reports whether a user is typing a filter (keys must not be
// interpreted as commands then).
func (m corpSelModel) Filtering() bool {
	return m.filter.Value() != ""
}

func (m *corpSelModel) refilter() tea.Cmd {
	matched := filterCorpora(m.corpora, m.filter.Value())
	items := make([]list.Item, len(matched))
	for i, c := range matched {
		items[i] = corpusListItem{c: c}
	}
	cmd := m.list.SetItems(items)
	m.list.ResetSelected()
	return cmd
}

func (m corpSelModel) Update(msg tea.Msg) (corpSelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case corplistLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.corpora = msg.corpora
		return m, m.refilter()

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "esc" {
				return m, goBack
			}
			return m, nil
		}
		if m.err != nil {
			switch msg.String() {
			case "r":
				m.loading, m.err = true, nil
				return m, tea.Batch(m.spinner.Tick, m.load())
			case "esc":
				return m, goBack
			case "q":
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "esc":
			if m.filter.Value() != "" {
				m.filter.SetValue("")
				return m, m.refilter()
			}
			return m, goBack
		case "enter":
			if it, ok := m.list.SelectedItem().(corpusListItem); ok {
				return m, func() tea.Msg { return corpusSelectedMsg{corpus: it.c} }
			}
			return m, nil
		case "up", "down", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		prev := m.filter.Value()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != prev {
			return m, tea.Batch(cmd, m.refilter())
		}
		return m, cmd
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Select clears the filter and moves the cursor to the corpus with
// the given ID (if present)
func (m *corpSelModel) Select(corpusID string) {
	if m.filter.Value() != "" {
		m.filter.SetValue("")
		m.refilter()
	}
	for i, it := range m.list.Items() {
		if ci, ok := it.(corpusListItem); ok && ci.c.ID == corpusID {
			m.list.Select(i)
			return
		}
	}
}

func (m corpSelModel) View() string {
	if m.loading {
		return m.spinner.View() + " loading corpora from " + m.env.Client.BaseURL() + " …"
	}
	if m.err != nil {
		return errorStyle.Render("Failed to load the list of corpora:\n\n"+m.err.Error()) +
			"\n\n" + mutedStyle.Render("press r to retry")
	}
	header := "  " + titleStyle.Render("Select a corpus") + "\n" +
		"  " + m.filter.View() + "\n\n"
	if len(m.list.Items()) == 0 {
		return header + mutedStyle.Render("  no matching corpora")
	}
	return header + m.list.View()
}

func (m corpSelModel) Help() string {
	if m.filter.Value() != "" {
		return "type to filter • ↑/↓ move • enter select • esc clear filter • ctrl+c quit"
	}
	return "type to filter • ↑/↓ move • enter select • esc back • ctrl+c quit"
}
