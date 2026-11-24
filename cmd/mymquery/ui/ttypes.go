package ui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/cql"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// preferred configured text property used as the default attribute
const ttDefaultTextProperty = "text-type"

var structAttrRegexp = regexp.MustCompile(`^[\w-]+\.[\w-]+$`)

// ttAttrOptions are structural attributes offered for text type frequencies
type ttAttrOptions struct {
	items       []selectItem
	defaultAttr string
}

type ttAttrOptionsMsg struct {
	viewID   int
	corpusID string
	opts     ttAttrOptions
	err      error
}

type ttLoadedMsg struct {
	viewID int
	reqID  int
	args   api.TextTypesArgs
	resp   *api.FreqsResponse
	err    error
}

// loadTTAttrOptions combines configured text properties (incl. attributes
// with many values, e.g. authors) with all the attributes listed by
// /text-types-avail-values (which skips attributes with too many values).
func loadTTAttrOptions(ctx context.Context, client *api.Client, corpusID string) (ttAttrOptions, error) {
	props, propsErr := client.CorpusTextProperties(ctx, corpusID)
	avail, availErr := client.TextTypesAvailValues(ctx, corpusID)
	if propsErr != nil && availErr != nil {
		return ttAttrOptions{}, propsErr
	}
	numValues := make(map[string]string)
	for _, a := range avail {
		n := formatInt(len(a.Values)) + " values"
		if a.IsTruncated {
			n = "> " + n
		}
		numValues[a.FullName()] = n
	}
	sort.Slice(props, func(i, j int) bool { return props[i].ID < props[j].ID })

	var opts ttAttrOptions
	used := make(map[string]bool)
	var firstOverview string
	for _, p := range props {
		if used[p.Attr] {
			continue
		}
		used[p.Attr] = true
		hint := p.ID
		if n, ok := numValues[p.Attr]; ok {
			hint += " · " + n
		}
		opts.items = append(opts.items, selectItem{Value: p.Attr, Hint: hint})
		if p.ID == ttDefaultTextProperty {
			opts.defaultAttr = p.Attr
		}
		if p.IsInOverview && firstOverview == "" {
			firstOverview = p.Attr
		}
	}
	for _, a := range avail {
		if used[a.FullName()] {
			continue
		}
		used[a.FullName()] = true
		opts.items = append(opts.items, selectItem{Value: a.FullName(), Hint: numValues[a.FullName()]})
	}
	if opts.defaultAttr == "" {
		opts.defaultAttr = firstOverview
	}
	if opts.defaultAttr == "" && len(opts.items) > 0 {
		opts.defaultAttr = opts.items[0].Value
	}
	return opts, nil
}

func parseStructAttr(s string) (string, error) {
	if !structAttrRegexp.MatchString(s) {
		return "", fmt.Errorf("expected struct.attr (e.g. doc.author)")
	}
	return s, nil
}

type ttypesView struct {
	id     int
	env    *Env
	corpus api.CorpusItem
	query  string
	width  int
	height int

	spinner spinner.Model
	// loadingAttrs is true until structural attributes are known
	loadingAttrs bool
	loading      bool
	err          error
	resp         *api.FreqsResponse
	// respArgs are the arguments `resp` was calculated with
	respArgs api.TextTypesArgs
	reqID    int
	// notice is a short-lived error message, info a success one
	notice string
	info   string

	attrOpts ttAttrOptions
	attr     string
	maxItems int
	flimit   int

	picker    *selectList
	pickerFor pickerID
	saveDlg   *csvDialog

	table freqTable

	ctx    context.Context
	cancel context.CancelFunc
}

func newTTypesView(env *Env, corpus api.CorpusItem, query string) ResultView {
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	table := newFreqTable(freqSortByFreq, freqSortByIPM, freqSortByValue)
	table.baseHeader = "text size"
	return &ttypesView{
		id:       nextViewID(),
		env:      env,
		corpus:   corpus,
		query:    query,
		spinner:  sp,
		maxItems: env.ttMaxItems,
		flimit:   env.ttFLimit,
		table:    table,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (v *ttypesView) Init() tea.Cmd {
	if opts, ok := v.env.ttAttrOptions[v.corpus.ID]; ok {
		return v.setAttrOptions(opts)
	}
	v.loadingAttrs = true
	client, ctx, corpusID, viewID := v.env.Client, v.ctx, v.corpus.ID, v.id
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			opts, err := loadTTAttrOptions(ctx, client, corpusID)
			return ttAttrOptionsMsg{viewID: viewID, corpusID: corpusID, opts: opts, err: err}
		},
	)
}

// setAttrOptions chooses the attribute (last used => configured default) and
// starts the calculation. With no known attribute, the picker is opened.
func (v *ttypesView) setAttrOptions(opts ttAttrOptions) tea.Cmd {
	v.attrOpts = opts
	v.attr = v.env.ttAttr[v.corpus.ID]
	if v.attr == "" {
		v.attr = opts.defaultAttr
	}
	if v.attr == "" {
		v.openAttrPicker()
		return nil
	}
	return v.fetch()
}

func (v *ttypesView) Title() string { return "Text type frequencies" }

func (v *ttypesView) Query() string { return v.query }

func (v *ttypesView) Close() { v.cancel() }

func (v *ttypesView) SetSize(w, h int) {
	v.width, v.height = w, h
	v.table.fix(v.tableHeight())
}

func (v *ttypesView) tableHeight() int {
	return max(2, v.height-freqsSummaryHeight)
}

func (v *ttypesView) fetch() tea.Cmd {
	v.reqID++
	v.loading = true
	v.err = nil
	reqID, viewID := v.reqID, v.id
	client, ctx, corpusID := v.env.Client, v.ctx, v.corpus.ID
	args := api.TextTypesArgs{
		Query:    v.query,
		Attr:     v.attr,
		MaxItems: v.maxItems,
		FLimit:   v.flimit,
	}
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			resp, err := client.TextTypes(ctx, corpusID, args)
			return ttLoadedMsg{viewID: viewID, reqID: reqID, args: args, resp: resp, err: err}
		},
	)
}

func (v *ttypesView) Update(msg tea.Msg) (ResultView, tea.Cmd) {
	switch msg := msg.(type) {
	case ttAttrOptionsMsg:
		if msg.viewID != v.id {
			return v, nil
		}
		v.loadingAttrs = false
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				return v, nil
			}
			// the user can still type an attribute manually
			v.err = fmt.Errorf("failed to load structural attributes: %w", msg.err)
			return v, v.setAttrOptions(ttAttrOptions{})
		}
		v.env.ttAttrOptions[msg.corpusID] = msg.opts
		return v, v.setAttrOptions(msg.opts)

	case ttLoadedMsg:
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
		v.respArgs = msg.args
		// remember only settings which work
		v.env.ttAttr[v.corpus.ID] = v.attr
		v.env.ttMaxItems = v.maxItems
		v.env.ttFLimit = v.flimit
		v.table.valueHeader = v.attr
		v.table.setData(msg.resp.Freqs, msg.resp.ConcSize)
		return v, nil

	case spinner.TickMsg:
		if !v.loading && !v.loadingAttrs {
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

func (v *ttypesView) updateKeys(msg tea.KeyMsg) (ResultView, tea.Cmd) {
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
	if v.loadingAttrs {
		return v, nil
	}
	if v.table.handleKey(key, v.tableHeight()) {
		return v, nil
	}
	switch key {
	case "r":
		if v.attr != "" {
			return v, v.fetch()
		}
	case "a":
		v.openAttrPicker()
	case "f":
		v.picker, v.pickerFor = newFLimitPicker(v.flimit), pickFLimit
	case "m":
		v.picker, v.pickerFor = newMaxItemsPicker(v.maxItems), pickMaxItems
	case "+", "=", "-":
		dir := 1
		if key == "-" {
			dir = -1
		}
		if n, ok := stepMaxItems(v.maxItems, dir); ok && v.attr != "" {
			v.maxItems = n
			return v, v.fetch()
		}
	case "s":
		v.table.cycleSort()
	case "e":
		if v.resp == nil || v.err != nil {
			v.notice = "no result to save"
			return v, nil
		}
		v.saveDlg = newCSVDialog("Save text type frequencies as CSV",
			csvFileName("text-types", v.corpus.ID, v.respArgs), v.table.csvRows())
	case "enter":
		if it, ok := v.table.selected(); ok && !v.loading {
			return v, v.concordanceFor(it)
		}
	}
	return v, nil
}

func (v *ttypesView) openAttrPicker() {
	v.picker = newSelectList("Structural attribute", v.attrOpts.items, v.attr).
		withCustom("filter or type struct.attr", parseStructAttr)
	v.pickerFor = pickAttr
}

func (v *ttypesView) updatePicker(msg tea.KeyMsg) (ResultView, tea.Cmd) {
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
		// also retry after an error (e.g. a wrongly typed attribute)
		if v.attr != "" && (changed || v.err != nil) {
			return v, v.fetch()
		}
	}
	return v, nil
}

func (v *ttypesView) concordanceFor(item api.FreqItem) tea.Cmd {
	q, err := cql.WithinStructAttr(v.query, v.attr, item.Value)
	if err != nil {
		v.err = err
		return nil
	}
	note := fmt.Sprintf("%s = %q", v.attr, item.Value)
	return pushResult(newConcViewWithNote(v.env, v.corpus, q, note))
}

// ---------------------------------- rendering

func (v *ttypesView) View() string {
	body := v.baseView()
	if v.saveDlg != nil {
		return placeOverlay(body, v.saveDlg.View(v.width), v.width, v.height)
	}
	if v.picker != nil {
		return placeOverlay(body, v.picker.View(v.width), v.width, v.height)
	}
	return body
}

func (v *ttypesView) baseView() string {
	if v.err != nil {
		return errorStyle.Render(v.err.Error()) + "\n\n" +
			mutedStyle.Render("a choose attribute • r retry • esc edit query")
	}
	if v.loadingAttrs {
		return v.spinner.View() + " loading structural attributes…"
	}
	if v.attr == "" {
		return mutedStyle.Render("  press a to choose a structural attribute")
	}
	if v.resp == nil {
		return v.spinner.View() + " calculating text type frequencies…"
	}
	return v.summary() + "\n\n" + v.table.view(v.width, v.tableHeight())
}

func (v *ttypesView) summary() string {
	r := v.resp
	parts := []string{
		boldStyle.Render(formatInt(int(r.ConcSize))) + " hits",
		"attr: " + accentStyle.Render(v.attr),
		"min freq: " + accentStyle.Render(fmt.Sprint(v.flimit)),
		itemsInfo(len(r.Freqs), v.maxItems),
		"sorted by " + v.table.sortBy().String(),
		mutedStyle.Render("ipm = per million tokens of the text type"),
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

func (v *ttypesView) Help() string {
	if v.saveDlg != nil {
		return v.saveDlg.Help()
	}
	if v.picker != nil {
		return v.picker.Help()
	}
	return "↑/↓ move • enter concordance • a attribute • f min freq • m/+/- max items • s sort • e save CSV • r reload • esc edit query • q quit"
}
