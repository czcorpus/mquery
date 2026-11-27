package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// default argument values of the MQuery MCP tools (see the mcp package);
// they are needed to show a stored result with the same settings it was
// calculated with
const (
	mcpDefaultMaxItems    = 20
	mcpDefaultFLimit      = 1
	mcpDefaultFreqAttr    = "lemma"
	mcpDefaultCollMeasure = "logDice"
	mcpDefaultCollAttr    = "lemma"
	mcpDefaultCollWindow  = 5
	mcpDefaultMinCollFreq = 3
)

// corpusOfView is implemented by views which can tell their corpus
// (used e.g. in the header for results opened from the AI chat)
type corpusOfView interface {
	// CorpusLabel returns the corpus ID, incl. a subcorpus if any
	// (see searchTarget.Label)
	CorpusLabel() string
}

// newChatResultView creates a view for a stored tool call result. Results
// with a dedicated view (concordance, frequencies, ...) are shown in it
// (without requerying), the others are shown as (pretty printed) raw data.
func newChatResultView(env *Env, res *agent.Result) ResultView {
	if res.OK {
		if v := newDedicatedResultView(env, res); v != nil {
			return v
		}
	}
	return newRawResultView(res)
}

func newDedicatedResultView(env *Env, res *agent.Result) ResultView {
	corpus := searchTarget{
		CorpusItem: api.CorpusItem{ID: res.StringArg("corpus_id")},
		Subc:       res.StringArg("subcorpus"),
	}
	query := res.StringArg("q")
	if corpus.ID == "" || query == "" {
		return nil
	}
	switch res.Tool {
	case "freqs":
		var resp api.FreqsResponse
		if json.Unmarshal([]byte(res.Content), &resp) != nil {
			return nil
		}
		v := newFreqsView(env, corpus, query).(*freqsView)
		v.attr = stringArgOr(res, "attr", mcpDefaultFreqAttr)
		v.matchCase = res.BoolArg("match_case")
		v.maxItems = res.IntArg("max_items", mcpDefaultMaxItems)
		v.flimit = res.IntArg("flimit", mcpDefaultFLimit)
		v.preloaded = &resp
		return v

	case "text_types":
		var resp api.FreqsResponse
		if json.Unmarshal([]byte(res.Content), &resp) != nil || res.StringArg("attr") == "" {
			return nil
		}
		v := newTTypesView(env, corpus, query).(*ttypesView)
		v.attr = res.StringArg("attr")
		v.maxItems = res.IntArg("max_items", mcpDefaultMaxItems)
		v.flimit = res.IntArg("flimit", mcpDefaultFLimit)
		v.preloaded = &resp
		return v

	case "collocations":
		var resp api.CollocationsResponse
		if json.Unmarshal([]byte(res.Content), &resp) != nil {
			return nil
		}
		v := newCollsView(env, corpus, query).(*collsView)
		v.attr = stringArgOr(res, "srch_attr", mcpDefaultCollAttr)
		v.measure = stringArgOr(res, "measure", mcpDefaultCollMeasure)
		v.window = [2]int{
			res.IntArg("srch_left", mcpDefaultCollWindow),
			res.IntArg("srch_right", mcpDefaultCollWindow),
		}
		v.minFreq = res.IntArg("min_coll_freq", mcpDefaultMinCollFreq)
		v.maxItems = res.IntArg("max_items", mcpDefaultMaxItems)
		v.preloaded = &resp
		return v

	case "concordance":
		if f := res.StringArg("format"); f != "" && f != "json" {
			return nil
		}
		var resp api.ConcordanceResponse
		if json.Unmarshal([]byte(res.Content), &resp) != nil {
			return nil
		}
		v := newConcViewWithNote(env, corpus, query, "")
		v.rowsOffset = res.IntArg("rows_offset", 0)
		if coll := res.StringArg("coll"); coll != "" {
			if left, right, ok := parseCollRange(res.StringArg("coll_range")); ok {
				v.withColl(coll, left, right)
				v.note = fmt.Sprintf("collocate %s, range %d,%d", coll, left, right)
			}
		}
		v.preloaded = &resp
		return v
	}
	return nil
}

func stringArgOr(res *agent.Result, name, dflt string) string {
	if v := res.StringArg(name); v != "" {
		return v
	}
	return dflt
}

// parseCollRange parses `left,right` (e.g. -3,3)
func parseCollRange(s string) (int, int, bool) {
	l, r, ok := strings.Cut(s, ",")
	if !ok {
		return 0, 0, false
	}
	left, err1 := strconv.Atoi(strings.TrimSpace(l))
	right, err2 := strconv.Atoi(strings.TrimSpace(r))
	return left, right, err1 == nil && err2 == nil
}

func (v *freqsView) CorpusLabel() string  { return v.corpus.Label() }
func (v *ttypesView) CorpusLabel() string { return v.corpus.Label() }
func (v *collsView) CorpusLabel() string  { return v.corpus.Label() }
func (v *concView) CorpusLabel() string   { return v.corpus.Label() }

// ---------------------------------- generic result view

// rawResultView shows a stored tool result without a dedicated view:
// JSON in a readable form (see renderJSON) or as (pretty printed) text
type rawResultView struct {
	res *agent.Result
	// text is the (pretty printed) original
	text string
	// data is the parsed JSON (nil if the result is not JSON)
	data *jsonNode
	// showRaw shows the original text instead of the readable form
	showRaw bool
	vp      viewport.Model
	width   int
	height  int
}

func newRawResultView(res *agent.Result) *rawResultView {
	v := &rawResultView{res: res, text: prettyJSON(res.Content), vp: viewport.New(0, 0)}
	if res.OK {
		if data, err := parseOrderedJSON(res.Content); err == nil {
			v.data = data
		}
	}
	return v
}

func (v *rawResultView) Init() tea.Cmd { return nil }

func (v *rawResultView) Title() string { return v.res.ID + " " + v.res.Tool }

func (v *rawResultView) Query() string { return v.res.StringArg("q") }

func (v *rawResultView) CorpusLabel() string {
	return searchTarget{
		CorpusItem: api.CorpusItem{ID: v.res.StringArg("corpus_id")},
		Subc:       v.res.StringArg("subcorpus"),
	}.Label()
}

func (v *rawResultView) Close() {}

func (v *rawResultView) SetSize(w, h int) {
	v.width, v.height = w, h
	v.vp.Width = max(1, w-2)
	v.vp.Height = max(1, h-2)
	v.refresh()
}

func (v *rawResultView) refresh() {
	switch {
	case v.data != nil && !v.showRaw:
		v.vp.SetContent(renderJSON(v.data, v.vp.Width))
	case !v.res.OK:
		v.vp.SetContent(errorStyle.Render(ansi.Wrap(v.text, v.vp.Width, "")))
	default:
		v.vp.SetContent(ansi.Wrap(v.text, v.vp.Width, ""))
	}
}

func (v *rawResultView) Update(msg tea.Msg) (ResultView, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, nil
	}
	switch key.String() {
	case "esc":
		return v, goBack
	case "q":
		return v, tea.Quit
	case "g", "home":
		v.vp.GotoTop()
		return v, nil
	case "G", "end":
		v.vp.GotoBottom()
		return v, nil
	case "r":
		if v.data != nil {
			v.showRaw = !v.showRaw
			v.refresh()
			v.vp.GotoTop()
		}
		return v, nil
	}
	var cmd tea.Cmd
	v.vp, cmd = v.vp.Update(msg)
	return v, cmd
}

func (v *rawResultView) View() string {
	summary := mutedStyle.Render(fmt.Sprintf("%s · %s %s", v.res.ID, v.res.Tool, formatToolArgs(v.res.Args)))
	return " " + ansi.Truncate(summary, max(0, v.width-2), "…") + "\n\n" +
		indentLines(v.vp.View(), " ")
}

func (v *rawResultView) Help() string {
	toggle := ""
	if v.data != nil {
		toggle = " • r raw JSON"
		if v.showRaw {
			toggle = " • r readable form"
		}
	}
	return "↑/↓/pgup/pgdn scroll • g/G top/bottom" + toggle + " • esc back • q quit"
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
