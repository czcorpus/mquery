package ui

import (
	"mquery/cmd/mymquery/agent"
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
	// HistoryPath is a file storing submitted queries ("" = no persistence)
	HistoryPath string
	HistorySize int

	// Chat is an AI chat session (nil = the chat is not configured)
	Chat *agent.Session
	// ChatUnavailable explains why Chat is nil
	ChatUnavailable string

	// attrCache maps corpus ID => ordered positional attributes
	attrCache map[string][]corpusAttr

	// token frequency settings (kept for the whole session)
	freqAttr      map[string]string // corpus ID => attribute
	freqMatchCase bool
	freqMaxItems  int
	freqFLimit    int

	// text type frequency settings
	ttAttrOptions map[string]ttAttrOptions // corpus ID => available struct. attributes
	ttAttr        map[string]string        // corpus ID => attribute
	ttMaxItems    int
	ttFLimit      int

	// collocation settings
	collMeasure  string
	collAttr     map[string]string // corpus ID => attribute
	collWindow   [2]int            // left, right (both >= 0)
	collMinFreq  int
	collMaxItems int
}

func NewEnv(client *api.Client, lang string, pageSize, contextWidth int) *Env {
	return &Env{
		Client:        client,
		Lang:          lang,
		PageSize:      pageSize,
		ContextWidth:  contextWidth,
		HistorySize:   DefaultHistorySize,
		attrCache:     make(map[string][]corpusAttr),
		freqAttr:      make(map[string]string),
		freqMaxItems:  freqMaxItemsSteps[0],
		freqFLimit:    1,
		ttAttrOptions: make(map[string]ttAttrOptions),
		ttAttr:        make(map[string]string),
		ttMaxItems:    freqMaxItemsSteps[0],
		ttFLimit:      1,
		collMeasure:   collDefaultMeasure,
		collAttr:      make(map[string]string),
		collWindow:    [2]int{5, 5},
		collMinFreq:   3,
		collMaxItems:  freqMaxItemsSteps[0],
	}
}

// ResultView is implemented by each function's result screen.
type ResultView interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (ResultView, tea.Cmd)
	View() string
	SetSize(width, height int)
	// Title is a name of the view shown in the header (e.g. "Concordance")
	Title() string
	// Query is the query the view shows results for
	Query() string
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
	// QueryType identifies the kind of input; the last query is shared
	// among features of the same type (e.g. a CQL query typed for
	// a concordance is offered for frequencies too)
	QueryType string
	// HasColl enables an optional collocate filter in the query input;
	// the view returned by NewView must implement collFilterable
	HasColl bool
	// NewView creates a result view; nil means "not implemented yet"
	NewView func(env *Env, corpus api.CorpusItem, query string) ResultView
	// Chat marks the AI chat (which has its own screen instead of
	// the query + result views)
	Chat bool
}

// collFilterable is implemented by views supporting a collocate filter
// (see Feature.HasColl)
type collFilterable interface {
	// setCollFilter restricts results to those with a collocate matching
	// the CQL query `coll` within `window` ([left, right], both >= 0)
	setCollFilter(coll string, window [2]int)
}

func (f Feature) Implemented() bool {
	return f.NewView != nil || f.Chat
}

const queryTypeCQL = "cql"

func DefaultFeatures() []Feature {
	return []Feature{
		{
			Title:            "Concordance",
			Description:      "search for a query and show matching lines (KWIC)",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [lemma="house"]`,
			QueryType:        queryTypeCQL,
			HasColl:          true,
			NewView:          newConcView,
		},
		{
			Title:            "Token frequencies",
			Description:      "frequency distribution of a positional attribute",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [tag="N.*"]`,
			QueryType:        queryTypeCQL,
			NewView:          newFreqsView,
		},
		{
			Title:            "Text type frequencies",
			Description:      "frequency distribution by text metadata",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [lemma="house"]`,
			QueryType:        queryTypeCQL,
			NewView:          newTTypesView,
		},
		{
			Title:            "Collocations",
			Description:      "collocation candidates of a query",
			InputLabel:       "CQL query",
			InputPlaceholder: `e.g. [lemma="house"]`,
			QueryType:        queryTypeCQL,
			NewView:          newCollsView,
		},
		{
			Title:       "AI chat",
			Description: "ask a language model which queries the corpora for you",
			Chat:        true,
		},
	}
}

// backMsg is emitted by a screen that wants to return to the previous one
type backMsg struct{}

func goBack() tea.Msg { return backMsg{} }
