package ui

import (
	"context"
	"fmt"
	"strings"

	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
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

type corpSelModel struct {
	env     *Env
	list    list.Model
	spinner spinner.Model
	loading bool
	err     error
}

func newCorpSel(env *Env) corpSelModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select a corpus"
	l.Styles.Title = titleStyle
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return corpSelModel{env: env, list: l, spinner: sp, loading: true}
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
	m.list.SetSize(w, h)
}

// Filtering reports whether a user is typing a filter (keys must not be
// interpreted as commands then).
func (m corpSelModel) Filtering() bool {
	return m.list.SettingFilter()
}

func (m corpSelModel) Update(msg tea.Msg) (corpSelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case corplistLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.corpora))
		for i, c := range msg.corpora {
			items[i] = corpusListItem{c: c}
		}
		return m, m.list.SetItems(items)

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
		if !m.list.SettingFilter() {
			switch msg.String() {
			case "esc":
				if m.list.FilterState() == list.Unfiltered {
					return m, goBack
				}
			case "q":
				return m, tea.Quit
			case "enter":
				if it, ok := m.list.SelectedItem().(corpusListItem); ok {
					return m, func() tea.Msg { return corpusSelectedMsg{corpus: it.c} }
				}
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Select moves the cursor to the corpus with the given ID (if present)
func (m *corpSelModel) Select(corpusID string) {
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
	return m.list.View()
}

func (m corpSelModel) Help() string {
	if m.list.SettingFilter() {
		return "type to filter • enter apply filter • esc cancel"
	}
	return "↑/↓ move • / filter • enter select • esc back • q quit"
}
