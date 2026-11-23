package ui

import (
	"mquery/cmd/mymquery/api"

	tea "github.com/charmbracelet/bubbletea"
)

// Env holds app-wide dependencies and settings shared by all views.
// It is accessed only from the Bubble Tea update loop (i.e. not from commands).
type Env struct {
	Client       *api.Client
	Lang         string
	PageSize     int
	ContextWidth int

	// attrCache maps corpus ID => ordered positional attributes
	attrCache map[string][]string
}

func NewEnv(client *api.Client, lang string, pageSize, contextWidth int) *Env {
	return &Env{
		Client:       client,
		Lang:         lang,
		PageSize:     pageSize,
		ContextWidth: contextWidth,
		attrCache:    make(map[string][]string),
	}
}

// ResultView is implemented by each function's result screen.
type ResultView interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (ResultView, tea.Cmd)
	View() string
	SetSize(width, height int)
	// Help returns a short key help shown in the footer
	Help() string
	// Close is called when the view is left (e.g. to cancel requests)
	Close()
}

// Feature is a single application function (concordance, freqs, ...)
type Feature struct {
	Title            string
	Description      string
	InputLabel       string
	InputPlaceholder string
	// NewView creates a result view; nil means "not implemented yet"
	NewView func(env *Env, corpus api.CorpusItem, query string) ResultView
}

func (f Feature) Implemented() bool {
	return f.NewView != nil
}

func DefaultFeatures() []Feature {
	return []Feature{
		{
			Title:            "Concordance",
			Description:      "search for a query and show matching lines (KWIC)",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [lemma="house"]`,
			NewView:          newConcView,
		},
		{
			Title:            "Token frequencies",
			Description:      "frequency distribution of a positional attribute",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [tag="N.*"]`,
		},
		{
			Title:            "Text type frequencies",
			Description:      "frequency distribution by text metadata",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [lemma="house"]`,
		},
		{
			Title:            "Collocations",
			Description:      "collocation candidates of a query",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [lemma="house"]`,
		},
	}
}

// backMsg is emitted by a screen that wants to return to the previous one
type backMsg struct{}

func goBack() tea.Msg { return backMsg{} }
