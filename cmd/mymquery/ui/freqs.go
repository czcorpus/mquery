package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/cql"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const freqsSummaryHeight = 2 // summary line + empty line

// freqsPicker identifies which option is being edited via a selectList
type freqsPicker int

const (
	pickNone freqsPicker = iota
	pickAttr
	pickFLimit
	pickMaxItems
)

type freqsLoadedMsg struct {
	viewID int
	reqID  int
	resp   *api.FreqsResponse
	err    error
}

type freqsView struct {
	id     int
	env    *Env
	corpus api.CorpusItem
	query  string
	width  int
	height int

	spinner spinner.Model
	loading bool
	err     error
	resp    *api.FreqsResponse
	reqID   int
	// notice is a short-lived message (e.g. a hint for an unavailable action)
	notice string

	// attrs are positional attributes of the corpus; nil until loaded
	attrs     []string
	attr      string
	matchCase bool
	maxItems  int
	flimit    int

	picker    *selectList
	pickerFor freqsPicker

	table freqTable

	ctx    context.Context
	cancel context.CancelFunc
}

func newFreqsView(env *Env, corpus api.CorpusItem, query string) ResultView {
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &freqsView{
		id:        nextViewID(),
		env:       env,
		corpus:    corpus,
		query:     query,
		spinner:   sp,
		attrs:     attrNames(env.attrCache[corpus.ID]),
		matchCase: env.freqMatchCase,
		maxItems:  env.freqMaxItems,
		flimit:    env.freqFLimit,
		// all items share the same base (corpus size) so ipm order == freq order
		table:  newFreqTable(freqSortByFreq, freqSortByValue),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (v *freqsView) Init() tea.Cmd {
	v.loading = true
	if v.attrs == nil {
		// we need to know the attributes first to choose a valid default
		return tea.Batch(v.spinner.Tick, fetchCorpusAttrs(v.env, v.ctx, v.corpus.ID, v.id))
	}
	v.chooseAttr()
	return v.fetch()
}

func (v *freqsView) Title() string { return "Token frequencies" }

func (v *freqsView) Query() string { return v.query }

func (v *freqsView) Close() { v.cancel() }

func (v *freqsView) SetSize(w, h int) {
	v.width, v.height = w, h
	v.table.fix(v.tableHeight())
}

func (v *freqsView) tableHeight() int {
	return max(2, v.height-freqsSummaryHeight)
}

// chooseAttr picks the last used attribute for the corpus,
// then `lemma` (= the server default), then `word`
func (v *freqsView) chooseAttr() {
	for _, a := range []string{v.env.freqAttr[v.corpus.ID], "lemma", "word"} {
		if a != "" && (len(v.attrs) == 0 || slices.Contains(v.attrs, a)) {
			v.attr = a
			return
		}
	}
	v.attr = v.attrs[0]
}

func (v *freqsView) fetch() tea.Cmd {
	v.reqID++
	v.loading = true
	v.err = nil
	reqID, viewID := v.reqID, v.id
	client, ctx, corpusID := v.env.Client, v.ctx, v.corpus.ID
	args := api.FreqsArgs{
		Query:     v.query,
		Attr:      v.attr,
		MatchCase: v.matchCase,
		MaxItems:  v.maxItems,
		FLimit:    v.flimit,
	}
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			resp, err := client.Freqs(ctx, corpusID, args)
			return freqsLoadedMsg{viewID: viewID, reqID: reqID, resp: resp, err: err}
		},
	)
}

func (v *freqsView) Update(msg tea.Msg) (ResultView, tea.Cmd) {
	switch msg := msg.(type) {
	case corpusAttrsMsg:
		if msg.viewID != v.id {
			return v, nil
		}
		if msg.err == nil {
			v.env.attrCache[msg.corpusID] = msg.attrs
			v.attrs = attrNames(msg.attrs)
		} else {
			// we can still try the server default, just without attr switching
			v.attrs = []string{}
		}
		v.chooseAttr()
		return v, v.fetch()

	case freqsLoadedMsg:
		if msg.viewID != v.id || msg.reqID != v.reqID {
			return v, nil
		}
		v.loading = false
		if msg.err != nil {
			if !errors.Is(msg.err, context.Canceled) {
				v.err = msg.err
			}
			return v, nil
		}
		v.resp = msg.resp
		// remember only settings which work
		v.env.freqAttr[v.corpus.ID] = v.attr
		v.env.freqMatchCase = v.matchCase
		v.env.freqMaxItems = v.maxItems
		v.env.freqFLimit = v.flimit
		v.table.valueHeader = v.attr
		v.table.setData(msg.resp.Freqs, msg.resp.ConcSize)
		return v, nil

	case spinner.TickMsg:
		if !v.loading {
			return v, nil
		}
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(msg)
		return v, cmd

	case tea.KeyMsg:
		return v.updateKeys(msg)
	}
	return v, nil
}

func (v *freqsView) updateKeys(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	v.notice = ""
	if v.picker != nil {
		return v.updatePicker(msg)
	}
	key := msg.String()
	switch key {
	case "esc":
		return v, goBack
	case "q":
		return v, tea.Quit
	}
	if v.attr == "" {
		return v, nil // still loading corpus attributes
	}
	if v.table.handleKey(key, v.tableHeight()) {
		return v, nil
	}
	switch key {
	case "r":
		return v, v.fetch()
	case "a":
		if len(v.attrs) < 2 {
			v.notice = "no other attributes available"
			return v, nil
		}
		v.openPicker(pickAttr, newSelectList(
			"Attribute", attrSelectItems(v.attrs, v.env, v.corpus.ID), v.attr))
	case "f":
		v.openPicker(pickFLimit, newFLimitPicker(v.flimit))
	case "m":
		v.openPicker(pickMaxItems, newMaxItemsPicker(v.maxItems))
	case "i":
		v.matchCase = !v.matchCase
		return v, v.fetch()
	case "+", "=", "-":
		dir := 1
		if key == "-" {
			dir = -1
		}
		if n, ok := stepMaxItems(v.maxItems, dir); ok {
			v.maxItems = n
			return v, v.fetch()
		}
	case "s":
		v.table.cycleSort()
	case "enter":
		if it, ok := v.table.selected(); ok && !v.loading {
			return v, pushResult(v.concordanceFor(it))
		}
	}
	return v, nil
}

func (v *freqsView) openPicker(p freqsPicker, s *selectList) {
	v.picker, v.pickerFor = s, p
}

func (v *freqsView) updatePicker(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	res, val := v.picker.Update(msg)
	switch res {
	case selectCancelled:
		v.picker = nil
	case selectChosen:
		v.picker = nil
		changed := false
		switch v.pickerFor {
		case pickAttr:
			changed = val != v.attr
			v.attr = val
		case pickFLimit:
			n, _ := strconv.Atoi(val) // already validated by the select list
			changed = n != v.flimit
			v.flimit = n
		case pickMaxItems:
			n, _ := strconv.Atoi(val)
			changed = n != v.maxItems
			v.maxItems = n
		}
		if changed {
			return v, v.fetch()
		}
	}
	return v, nil
}

// concordanceFor creates a concordance view showing occurrences of the item
func (v *freqsView) concordanceFor(item api.FreqItem) ResultView {
	values := strings.Split(item.Value, " ")
	q, merged := cql.RestrictByValues(v.query, v.attr, values, !v.matchCase)
	note := fmt.Sprintf("%s = %q", v.attr, item.Value)
	if !merged {
		note += " (the original query could not be combined, showing all occurrences)"
	}
	return newConcViewWithNote(v.env, v.corpus, q, note)
}

// ---------------------------------- rendering

func (v *freqsView) View() string {
	body := v.baseView()
	if v.picker != nil {
		return placeOverlay(body, v.picker.View(v.width), v.width, v.height)
	}
	return body
}

func (v *freqsView) baseView() string {
	if v.err != nil {
		return errorStyle.Render(v.err.Error()) + "\n\n" +
			mutedStyle.Render("r retry • a attribute • esc edit query")
	}
	if v.resp == nil {
		return v.spinner.View() + " calculating frequencies…"
	}
	return v.summary() + "\n\n" + v.table.view(v.width, v.tableHeight())
}

func (v *freqsView) summary() string {
	r := v.resp
	caseInfo := "ignored"
	if v.matchCase {
		caseInfo = "respected"
	}
	parts := []string{
		boldStyle.Render(formatInt(int(r.ConcSize))) + " hits",
		"attr: " + accentStyle.Render(v.attr),
		"case: " + caseInfo,
		"min freq: " + accentStyle.Render(fmt.Sprint(v.flimit)),
		itemsInfo(len(r.Freqs), v.maxItems),
		"sorted by " + v.table.sortBy().String(),
	}
	s := "  " + strings.Join(parts, mutedStyle.Render(" • "))
	if v.loading {
		s += "  " + v.spinner.View()
	}
	if v.notice != "" {
		s += "  " + errorStyle.Render(v.notice)
	}
	return ansi.Truncate(s, v.width, "…")
}

func (v *freqsView) Help() string {
	if v.picker != nil {
		return v.picker.Help()
	}
	return "↑/↓ move • enter concordance • a attribute • f min freq • m/+/- max items • i case • s sort • r reload • esc edit query • q quit"
}
