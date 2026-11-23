package ui

import (
	"strings"

	"mquery/cmd/mymquery/api"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type screen int

const (
	scrMenu screen = iota
	scrCorpus
	scrQuery
	scrResult
)

// header + empty line + footer
const chromeHeight = 3

// App is the root Bubble Tea model. It owns the session state
// (selected function, corpus, queries) and routes messages to screens.
type App struct {
	env      *Env
	features []Feature
	screen   screen
	width    int
	height   int

	menu    menuModel
	corpSel corpSelModel
	query   queryModel
	// results is a stack of result views; the last one is active
	// (nested views are e.g. concordances opened from a frequency list)
	results []ResultView

	featureIdx int
	corpus     *api.CorpusItem
	// queries stores the last query per feature
	queries map[int]string
}

func NewApp(env *Env) *App {
	features := DefaultFeatures()
	return &App{
		env:      env,
		features: features,
		menu:     newMenu(features),
		corpSel:  newCorpSel(env),
		query:    newQuery(),
		queries:  make(map[int]string),
	}
}

func (a *App) Init() tea.Cmd {
	// corpus list is loaded right away so it is ready when needed
	return a.corpSel.Init()
}

func (a *App) bodyHeight() int {
	return max(1, a.height-chromeHeight)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.corpSel.SetSize(a.width, a.bodyHeight())
		a.query.SetSize(a.width, a.bodyHeight())
		for _, r := range a.results {
			r.SetSize(a.width, a.bodyHeight())
		}
		return a, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			a.closeResults()
			return a, tea.Quit
		}
		return a.updateActive(msg)

	case featureSelectedMsg:
		a.featureIdx = msg.idx
		if a.corpus == nil {
			a.screen = scrCorpus
			return a, nil
		}
		return a, a.toQuery()

	case corpusSelectedMsg:
		c := msg.corpus
		a.corpus = &c
		return a, a.toQuery()

	case changeCorpusMsg:
		a.queries[a.featureIdx] = a.query.input.Value()
		if a.corpus != nil {
			a.corpSel.Select(a.corpus.ID)
		}
		a.screen = scrCorpus
		return a, nil

	case querySubmittedMsg:
		a.queries[a.featureIdx] = msg.query
		a.closeResults()
		a.screen = scrResult
		return a, a.pushResult(a.features[a.featureIdx].NewView(a.env, *a.corpus, msg.query))

	case pushResultMsg:
		if a.screen != scrResult {
			msg.view.Close()
			return a, nil
		}
		return a, a.pushResult(msg.view)

	case backMsg:
		switch a.screen {
		case scrCorpus:
			if a.corpus != nil {
				return a, a.toQuery()
			}
			a.screen = scrMenu
		case scrQuery:
			a.queries[a.featureIdx] = a.query.input.Value()
			a.query.input.Blur()
			a.screen = scrMenu
		case scrResult:
			top := len(a.results) - 1
			a.results[top].Close()
			a.results = a.results[:top]
			if len(a.results) == 0 {
				return a, a.toQuery()
			}
		}
		return a, nil
	}

	// other (async) messages: deliver to the models which may be waiting for them
	var cmds []tea.Cmd
	var cmd tea.Cmd
	a.corpSel, cmd = a.corpSel.Update(msg)
	cmds = append(cmds, cmd)
	for i, r := range a.results {
		a.results[i], cmd = r.Update(msg)
		cmds = append(cmds, cmd)
	}
	if a.screen == scrQuery {
		a.query, cmd = a.query.Update(msg) // cursor blinking
		cmds = append(cmds, cmd)
	}
	return a, tea.Batch(cmds...)
}

func (a *App) updateActive(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.screen {
	case scrMenu:
		a.menu, cmd = a.menu.Update(msg)
	case scrCorpus:
		a.corpSel, cmd = a.corpSel.Update(msg)
	case scrQuery:
		a.query, cmd = a.query.Update(msg)
	case scrResult:
		top := len(a.results) - 1
		a.results[top], cmd = a.results[top].Update(msg)
	}
	return a, cmd
}

func (a *App) toQuery() tea.Cmd {
	a.screen = scrQuery
	return a.query.Reset(a.features[a.featureIdx], a.queries[a.featureIdx])
}

func (a *App) pushResult(v ResultView) tea.Cmd {
	v.SetSize(a.width, a.bodyHeight())
	a.results = append(a.results, v)
	return v.Init()
}

func (a *App) activeResult() ResultView {
	return a.results[len(a.results)-1]
}

func (a *App) closeResults() {
	for _, r := range a.results {
		r.Close()
	}
	a.results = nil
}

func (a *App) View() string {
	if a.width == 0 {
		return ""
	}
	var body, help string
	switch a.screen {
	case scrMenu:
		body, help = a.menu.View(), a.menu.Help()
	case scrCorpus:
		body, help = a.corpSel.View(), a.corpSel.Help()
	case scrQuery:
		body, help = a.query.View(), a.query.Help()
	case scrResult:
		body, help = a.activeResult().View(), a.activeResult().Help()
	}
	if a.screen != scrCorpus && a.screen != scrResult {
		body = lipgloss.NewStyle().Padding(0, 2).Render(body)
	}
	body = lipgloss.NewStyle().Height(a.bodyHeight()).MaxHeight(a.bodyHeight()).Render(body)
	footer := footerStyle.Render(ansi.Truncate(help, max(0, a.width-2), "…"))
	return a.header() + "\n\n" + body + "\n" + footer
}

func (a *App) header() string {
	// every segment must carry the header background (inner ANSI resets
	// would otherwise clear the outer style)
	seg := headerStyle.Padding(0)
	parts := []string{headerKeyStyle.Render("MyMQuery")}
	if a.screen == scrResult {
		titles := make([]string, len(a.results))
		for i, r := range a.results {
			titles[i] = r.Title()
		}
		parts = append(parts, seg.Render(strings.Join(titles, " › ")))

	} else if a.screen != scrMenu {
		parts = append(parts, seg.Render(a.features[a.featureIdx].Title))
	}
	if a.corpus != nil {
		parts = append(parts, seg.Render("corpus: ")+headerKeyStyle.Render(a.corpus.ID))
	}
	if a.screen == scrResult {
		parts = append(parts, seg.Render("query: "+a.activeResult().Query()))
	}
	content := strings.Join(parts, seg.Render(" │ "))
	content = ansi.Truncate(content, max(0, a.width-2), "…")
	return headerStyle.Width(a.width).Render(content)
}
