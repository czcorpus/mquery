package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/cql"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	collDefaultMeasure = "logDice"
	collMaxWindow      = 50
)

// collMeasures are measures supported by MQuery (see /collocations docs)
var collMeasures = []selectItem{
	{Value: "logDice", Hint: "salience, comparable across corpora (default)"},
	{Value: "tScore", Hint: "certainty of collocation, favours frequent words"},
	{Value: "mutualInfo", Hint: "MI, favours rare collocates"},
	{Value: "mutualInfo3", Hint: "MI³, suppresses MI's preference of rare words"},
	{Value: "mutualInfoLogF", Hint: "MI·log(f)"},
	{Value: "logLikelihood", Hint: "log-likelihood"},
	{Value: "minSensitivity", Hint: "minimum sensitivity"},
	{Value: "relFreq", Hint: "relative frequency"},
	{Value: "absFreq", Hint: "absolute frequency"},
}

var collWindowPresets = [][2]int{
	{1, 1}, {2, 2}, {3, 3}, {5, 5}, {10, 10},
	{1, 0}, {3, 0}, {5, 0}, {0, 1}, {0, 3}, {0, 5},
}

func windowValue(w [2]int) string {
	return fmt.Sprintf("%d,%d", w[0], w[1])
}

func windowLabel(w [2]int) string {
	switch {
	case w[0] == 0:
		return fmt.Sprintf("right %d", w[1])
	case w[1] == 0:
		return fmt.Sprintf("left %d", w[0])
	default:
		return fmt.Sprintf("-%d … +%d", w[0], w[1])
	}
}

// parseWindow accepts `L,R`, `L R` or a single `N` (= N,N)
func parseWindow(s string) (string, error) {
	w, err := parseWindowPair(s)
	if err != nil {
		return "", err
	}
	return windowValue(w), nil
}

// parseWindowPair is like parseWindow but returns the parsed [left, right] pair
func parseWindowPair(s string) ([2]int, error) {
	var w [2]int
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	if len(fields) == 1 {
		fields = append(fields, fields[0])
	}
	if len(fields) != 2 {
		return w, fmt.Errorf("expected left,right (e.g. 3,5)")
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 || n > collMaxWindow {
			return w, fmt.Errorf("values must be integers between 0 and %d", collMaxWindow)
		}
		w[i] = n
	}
	if w[0] == 0 && w[1] == 0 {
		return w, fmt.Errorf("at least one side must be greater than 0")
	}
	return w, nil
}

func windowSelectItems() []selectItem {
	items := make([]selectItem, len(collWindowPresets))
	for i, w := range collWindowPresets {
		items[i] = selectItem{Value: windowValue(w), Label: windowLabel(w)}
	}
	return items
}

// collConcRange converts a collocation window to a concordance `collRange`
// (relative to KWIC, KWIC itself excluded)
func collConcRange(w [2]int) (int, int) {
	left, right := -w[0], w[1]
	if w[0] == 0 {
		left = 1
	}
	if w[1] == 0 {
		right = -1
	}
	return left, right
}

type collSort int

const (
	collSortByScore collSort = iota
	collSortByFreq
	collSortByValue
)

func (s collSort) String() string {
	switch s {
	case collSortByFreq:
		return "freq"
	case collSortByValue:
		return "value"
	default:
		return "score"
	}
}

type collsLoadedMsg struct {
	viewID int
	reqID  int
	args   api.CollocationsArgs
	resp   *api.CollocationsResponse
	err    error
	// preloaded is true for data not fetched by the view itself
	preloaded bool
}

type collsView struct {
	id     int
	env    *Env
	corpus searchTarget
	query  string
	width  int
	height int

	spinner spinner.Model
	loading bool
	err     error
	resp    *api.CollocationsResponse
	// respAttr is the attribute `resp` was calculated for
	respAttr string
	// respArgs are the arguments `resp` was calculated with
	respArgs api.CollocationsArgs
	reqID    int
	// notice is a short-lived error message, info a success one
	notice string
	info   string

	attrs    []string
	attr     string
	measure  string
	window   [2]int
	minFreq  int
	maxItems int

	picker    *selectList
	pickerFor pickerID
	saveDlg   *csvDialog

	items  []api.CollItem
	sortBy collSort
	nav    listNav

	// preloaded is a response to be used instead of the first fetch
	preloaded *api.CollocationsResponse

	ctx    context.Context
	cancel context.CancelFunc
}

func newCollsView(env *Env, corpus searchTarget, query string) ResultView {
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &collsView{
		id:       nextViewID(),
		env:      env,
		corpus:   corpus,
		query:    query,
		spinner:  sp,
		attrs:    attrNames(env.attrCache[corpus.ID]),
		measure:  env.collMeasure,
		window:   env.collWindow,
		minFreq:  env.collMinFreq,
		maxItems: env.collMaxItems,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (v *collsView) Init() tea.Cmd {
	v.loading = true
	if v.attrs == nil {
		return tea.Batch(v.spinner.Tick, fetchCorpusAttrs(v.env, v.ctx, v.corpus.ID, v.id))
	}
	v.chooseAttr()
	return v.fetch()
}

func (v *collsView) Title() string { return "Collocations" }

func (v *collsView) Query() string { return v.query }

func (v *collsView) Close() { v.cancel() }

func (v *collsView) SetSize(w, h int) {
	v.width, v.height = w, h
	v.nav.fix(len(v.items), v.rowsHeight())
}

// rowsHeight is the number of table data rows (summary, empty line and table header excluded)
func (v *collsView) rowsHeight() int {
	return max(1, v.height-3)
}

// chooseAttr picks the preset attribute (if any), the last used attribute
// for the corpus, then `lemma` (= the server default), then `word`
func (v *collsView) chooseAttr() {
	for _, a := range []string{v.attr, v.env.collAttr[v.corpus.ID], "lemma", "word"} {
		if a != "" && (len(v.attrs) == 0 || slices.Contains(v.attrs, a)) {
			v.attr = a
			return
		}
	}
	v.attr = v.attrs[0]
}

func (v *collsView) fetch() tea.Cmd {
	v.reqID++
	v.loading = true
	v.err = nil
	reqID, viewID := v.reqID, v.id
	client, ctx, corpusID := v.env.Client, v.ctx, v.corpus.ID
	args := api.CollocationsArgs{
		Query:       v.query,
		Subcorpus:   v.corpus.Subc,
		Measure:     v.measure,
		SrchLeft:    v.window[0],
		SrchRight:   v.window[1],
		SrchAttr:    v.attr,
		MinCollFreq: v.minFreq,
		MaxItems:    v.maxItems,
	}
	if resp := v.preloaded; resp != nil {
		v.preloaded = nil
		return func() tea.Msg {
			return collsLoadedMsg{viewID: viewID, reqID: reqID, args: args, resp: resp, preloaded: true}
		}
	}
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			resp, err := client.Collocations(ctx, corpusID, args)
			return collsLoadedMsg{viewID: viewID, reqID: reqID, args: args, resp: resp, err: err}
		},
	)
}

func (v *collsView) applySort() {
	if v.resp == nil {
		v.items = nil
		return
	}
	v.items = slices.Clone(v.resp.Colls)
	var less func(a, b api.CollItem) bool
	switch v.sortBy {
	case collSortByScore:
		less = func(a, b api.CollItem) bool { return a.Score > b.Score }
	case collSortByFreq:
		less = func(a, b api.CollItem) bool { return a.Freq > b.Freq }
	case collSortByValue:
		less = func(a, b api.CollItem) bool { return a.Value < b.Value }
	}
	sort.SliceStable(v.items, func(i, j int) bool { return less(v.items[i], v.items[j]) })
	v.nav.reset()
}

func (v *collsView) Update(msg tea.Msg) (ResultView, tea.Cmd) {
	switch msg := msg.(type) {
	case corpusAttrsMsg:
		if msg.viewID != v.id {
			return v, nil
		}
		if msg.err == nil {
			v.env.storeCorpusInfo(msg)
			v.attrs = attrNames(msg.attrs)
		} else {
			v.attrs = []string{}
		}
		v.chooseAttr()
		return v, v.fetch()

	case collsLoadedMsg:
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
		v.respAttr = v.attr
		v.respArgs = msg.args
		// remember only settings which work (and which were chosen by the user)
		if !msg.preloaded {
			v.env.collAttr[v.corpus.ID] = v.attr
			v.env.collMeasure = v.measure
			v.env.collWindow = v.window
			v.env.collMinFreq = v.minFreq
			v.env.collMaxItems = v.maxItems
		}
		v.applySort()
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

func (v *collsView) updateKeys(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	v.notice, v.info = "", ""
	if v.saveDlg != nil {
		res, path := v.saveDlg.Update(msg)
		switch res {
		case selectCancelled:
			v.saveDlg = nil
		case selectChosen:
			v.info = savedInfo(path, len(v.saveDlg.rows)-1)
			v.saveDlg = nil
		}
		return v, nil
	}
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
	if v.nav.handleKey(key, len(v.items), v.rowsHeight()) {
		return v, nil
	}
	switch key {
	case "r":
		return v, v.fetch()
	case "c":
		v.openPicker(pickMeasure, newSelectList("Collocation measure", collMeasures, v.measure))
	case "a":
		if len(v.attrs) < 2 {
			v.notice = "no other attributes available"
			return v, nil
		}
		v.openPicker(pickAttr, newSelectList(
			"Attribute", attrSelectItems(v.attrs, v.env, v.corpus.ID), v.attr))
	case "w":
		v.openPicker(pickWindow, newSelectList(
			"Window (tokens left, right)", windowSelectItems(), windowValue(v.window)).
			withCustom("filter or type left,right", parseWindow))
	case "f":
		v.openPicker(pickFLimit, newMinFreqPicker("Min. collocate frequency", v.minFreq))
	case "m":
		v.openPicker(pickMaxItems, newMaxItemsPicker(v.maxItems))
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
		v.sortBy = (v.sortBy + 1) % 3
		v.applySort()
	case "e":
		if v.resp == nil || v.err != nil {
			v.notice = "no result to save"
			return v, nil
		}
		v.saveDlg = newCSVDialog("Save collocations as CSV",
			csvFileName("colls", v.corpus.ID, v.respArgs), v.csvRows())
	case "enter":
		if len(v.items) > 0 && !v.loading && v.err == nil {
			return v, pushResult(v.concordanceFor(v.items[v.nav.cursor]))
		}
	}
	return v, nil
}

func (v *collsView) openPicker(p pickerID, s *selectList) {
	v.picker, v.pickerFor = s, p
}

func (v *collsView) updatePicker(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	res, val := v.picker.Update(msg)
	switch res {
	case selectCancelled:
		v.picker = nil
	case selectChosen:
		v.picker = nil
		changed := false
		switch v.pickerFor {
		case pickMeasure:
			changed = val != v.measure
			v.measure = val
		case pickAttr:
			changed = val != v.attr
			v.attr = val
		case pickWindow:
			l, r, _ := strings.Cut(val, ",") // already validated/normalized by parseWindow
			w := [2]int{atoiOrZero(l), atoiOrZero(r)}
			changed = w != v.window
			v.window = w
		case pickFLimit:
			n := atoiOrZero(val)
			changed = n != v.minFreq
			v.minFreq = n
		case pickMaxItems:
			n := atoiOrZero(val)
			changed = n != v.maxItems
			v.maxItems = n
		}
		if changed || v.err != nil {
			return v, v.fetch()
		}
	}
	return v, nil
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// concordanceFor shows KWIC lines with the collocate within the current window
func (v *collsView) concordanceFor(item api.CollItem) ResultView {
	// the window and attr are taken from the response (= the settings
	// the collocates were calculated with)
	window := v.resp.SrchRange
	left, right := collConcRange(window)
	coll := "[" + cql.AttrCond(v.respAttr, item.Value, false) + "]"
	note := fmt.Sprintf("collocate %s = %q, %s", v.respAttr, item.Value, windowLabel(window))
	return newConcViewWithNote(v.env, v.corpus, v.query, note).withColl(coll, left, right)
}

// csvRows returns the collocates (in the current order) incl. a header row
func (v *collsView) csvRows() [][]string {
	rows := [][]string{{v.respArgs.SrchAttr, v.respArgs.Measure, "freq"}}
	for _, it := range v.items {
		rows = append(rows, []string{it.Value, fmtCSVFloat(it.Score), fmtCSVInt(it.Freq)})
	}
	return rows
}

// ---------------------------------- rendering

func (v *collsView) View() string {
	body := v.baseView()
	if v.saveDlg != nil {
		return placeOverlay(body, v.saveDlg.View(v.width), v.width, v.height)
	}
	if v.picker != nil {
		return placeOverlay(body, v.picker.View(v.width), v.width, v.height)
	}
	return body
}

func (v *collsView) baseView() string {
	if v.err != nil {
		return errorStyle.Render(v.err.Error()) + "\n\n" +
			mutedStyle.Render("r retry • c measure • a attribute • w window • esc edit query")
	}
	if v.resp == nil {
		return v.spinner.View() + " calculating collocations…"
	}
	return v.summary() + "\n\n" + v.table()
}

func (v *collsView) summary() string {
	r := v.resp
	parts := []string{
		boldStyle.Render(formatInt(int(r.ConcSize))) + " hits",
		"measure: " + accentStyle.Render(v.measure),
		"attr: " + accentStyle.Render(v.attr),
		"window: " + accentStyle.Render(windowLabel(v.window)),
		"min freq: " + accentStyle.Render(fmt.Sprint(v.minFreq)),
		itemsInfo(len(r.Colls), v.maxItems),
		"sorted by " + v.sortBy.String(),
	}
	s := "  " + strings.Join(parts, mutedStyle.Render(" • "))
	if v.loading {
		s += "  " + v.spinner.View()
	}
	if v.notice != "" {
		s += "  " + errorStyle.Render(v.notice)
	}
	if v.info != "" {
		s += "  " + accentStyle.Render(v.info)
	}
	return ansi.Truncate(s, v.width, "…")
}

// table renders collocates; the bar shows the freq if sorted by freq,
// otherwise the score (negative scores have no bar)
func (v *collsView) table() string {
	if len(v.items) == 0 {
		return mutedStyle.Render("  No results.")
	}
	byFreq := v.sortBy == collSortByFreq
	numW := len(fmt.Sprint(len(v.items)))
	valW := sortedLabelW(v.attr, true)
	var maxFreq int64
	var maxScore float64
	scoreW := sortedLabelW(v.measure, true)
	for _, it := range v.items {
		valW = max(valW, ansi.StringWidth(it.Value))
		maxFreq = max(maxFreq, it.Freq)
		maxScore = max(maxScore, it.Score)
		scoreW = max(scoreW, len(fmt.Sprintf("%.3f", it.Score)))
	}
	freqW := max(sortedLabelW("freq", true), len(formatInt(int(maxFreq))))
	fixedW := 2 + numW + 2 + 2 + scoreW + 2 + freqW + 2
	valW = min(valW, max(10, v.width-fixedW-freqTableMinBarWidth))
	barW := max(0, v.width-fixedW-valW-1)

	dir := func(s collSort) sortDir {
		switch {
		case v.sortBy != s:
			return sortNone
		case s == collSortByValue:
			return sortAsc
		default:
			return sortDesc
		}
	}
	sep := colHeadStyle.Render("  ")
	header := colHeadStyle.Render(fmt.Sprintf("  %*s  ", numW, "#")) +
		headerCell(v.attr, valW, false, dir(collSortByValue)) + sep +
		headerCell(v.measure, scoreW, true, dir(collSortByScore)) + sep +
		headerCell("freq", freqW, true, dir(collSortByFreq)) + sep
	if byFreq {
		header += colHeadStyle.Render("(bar: freq)")
	}
	var b strings.Builder
	b.WriteString(header + "\n")
	from, to := v.nav.visibleRange(len(v.items), v.rowsHeight())
	for i := from; i < to; i++ {
		it := v.items[i]
		sel := i == v.nav.cursor
		base := lipgloss.NewStyle()
		marker := "  "
		if sel {
			base = base.Background(colSelBg)
			marker = accentStyle.Bold(true).Render("❯ ")
		}
		val := it.Value
		if ansi.StringWidth(val) > valW {
			val = ansi.Truncate(val, valW, "…")
		}
		var barStr string
		if byFreq {
			barStr = bar(float64(it.Freq), float64(maxFreq), barW)
		} else {
			barStr = bar(max(0, it.Score), maxScore, barW)
		}
		row := base.Foreground(colMuted).Render(fmt.Sprintf("%*d  ", numW, i+1)) +
			base.Bold(sel).Render(padRight(val, valW)) +
			base.Render(fmt.Sprintf("  %*.3f  %*s  ", scoreW, it.Score, freqW, formatInt(int(it.Freq)))) +
			base.Foreground(colAccent).Render(padRight(barStr, barW))
		b.WriteString(marker + row)
		if i < to-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (v *collsView) Help() string {
	if v.saveDlg != nil {
		return v.saveDlg.Help()
	}
	if v.picker != nil {
		return v.picker.Help()
	}
	return "↑/↓ move • enter concordance • c measure • a attribute • w window • f min freq • m/+/- max items • s sort • e save CSV • r reload • esc edit query • q quit"
}
