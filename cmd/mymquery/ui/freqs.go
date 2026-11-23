package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	freqsChromeHeight = 3 // summary line + empty line + table header
	freqsMinBarWidth  = 10
)

var (
	freqMaxItemsSteps = []int{20, 50, 100, 200, 500, 1000}
	freqFLimitPresets = []int{1, 2, 3, 5, 10, 20, 50, 100, 500, 1000}
)

const (
	freqMaxItemsLimit = 10000
	freqFLimitLimit   = 1_000_000_000
)

// freqsPicker identifies which option is being edited via a selectList
type freqsPicker int

const (
	pickNone freqsPicker = iota
	pickAttr
	pickFLimit
	pickMaxItems
)

type freqSort int

const (
	freqSortByFreq freqSort = iota
	freqSortByValue
)

func (s freqSort) String() string {
	if s == freqSortByValue {
		return "value"
	}
	return "freq"
}

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

	// items are response items in the current sort order
	items  []api.FreqItem
	sortBy freqSort
	nav    listNav

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
		ctx:       ctx,
		cancel:    cancel,
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
	v.nav.fix(len(v.items), v.listHeight())
}

func (v *freqsView) listHeight() int {
	return max(1, v.height-freqsChromeHeight)
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
	v.env.freqAttr[v.corpus.ID] = v.attr
	v.env.freqMatchCase = v.matchCase
	v.env.freqMaxItems = v.maxItems
	v.env.freqFLimit = v.flimit
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

func (v *freqsView) sortItems() {
	if v.resp == nil {
		v.items = nil
		return
	}
	v.items = slices.Clone(v.resp.Freqs)
	switch v.sortBy {
	case freqSortByFreq:
		sort.SliceStable(v.items, func(i, j int) bool { return v.items[i].Freq > v.items[j].Freq })
	case freqSortByValue:
		sort.SliceStable(v.items, func(i, j int) bool { return v.items[i].Value < v.items[j].Value })
	}
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
		v.sortItems()
		v.nav.reset()
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
	if v.nav.handleKey(key, len(v.items), v.listHeight()) {
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
		v.openPicker(pickFLimit, newSelectList(
			"Minimum frequency", intSelectItems(freqFLimitPresets), fmt.Sprint(v.flimit)).
			withCustom("filter or type a number", parsePositiveInt(1, freqFLimitLimit)))
	case "m":
		v.openPicker(pickMaxItems, newSelectList(
			"Max. items", intSelectItems(freqMaxItemsSteps), fmt.Sprint(v.maxItems)).
			withCustom("filter or type a number", parsePositiveInt(1, freqMaxItemsLimit)))
	case "i":
		v.matchCase = !v.matchCase
		return v, v.fetch()
	case "+", "=":
		// a custom maxItems may lie between the steps
		for _, s := range freqMaxItemsSteps {
			if s > v.maxItems {
				v.maxItems = s
				return v, v.fetch()
			}
		}
	case "-":
		for i := len(freqMaxItemsSteps) - 1; i >= 0; i-- {
			if s := freqMaxItemsSteps[i]; s < v.maxItems {
				v.maxItems = s
				return v, v.fetch()
			}
		}
	case "s":
		v.sortBy = (v.sortBy + 1) % 2
		v.sortItems()
		v.nav.reset()
	case "enter":
		if len(v.items) > 0 && !v.loading {
			return v, pushResult(v.concordanceFor(v.items[v.nav.cursor]))
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
			mutedStyle.Render("r retry • a switch attribute • esc edit query")
	}
	if v.resp == nil {
		return v.spinner.View() + " calculating frequencies…"
	}
	var b strings.Builder
	b.WriteString(v.summary() + "\n\n")
	if len(v.items) == 0 {
		b.WriteString(mutedStyle.Render("  No results."))
		return b.String()
	}

	numW := len(fmt.Sprint(len(v.items)))
	valW := ansi.StringWidth(v.attr)
	var maxFreq int64
	for _, it := range v.items {
		valW = max(valW, ansi.StringWidth(it.Value))
		maxFreq = max(maxFreq, it.Freq)
	}
	freqW := max(4, len(formatInt(int(maxFreq))))
	const ipmW, pctW = 10, 6
	fixedW := 2 + numW + 2 + 2 + freqW + 2 + ipmW + 2 + pctW + 2
	valW = min(valW, max(10, v.width-fixedW-freqsMinBarWidth))
	barW := max(0, v.width-fixedW-valW-1)

	header := fmt.Sprintf("  %*s  %s  %*s  %*s  %*s  ",
		numW, "#", padRight(v.attr, valW), freqW, "freq", ipmW, "ipm", pctW, "%")
	b.WriteString(mutedStyle.Underline(true).Render(header) + "\n")

	from, to := v.nav.visibleRange(len(v.items), v.listHeight())
	for i := from; i < to; i++ {
		it := v.items[i]
		sel := i == v.nav.cursor
		base := lipgloss.NewStyle()
		marker := "  "
		if sel {
			base = base.Background(colSelBg)
			marker = accentStyle.Bold(true).Render("❯ ")
		}
		var pct float64
		if v.resp.ConcSize > 0 {
			pct = float64(it.Freq) / float64(v.resp.ConcSize) * 100
		}
		val := it.Value
		if ansi.StringWidth(val) > valW {
			val = ansi.Truncate(val, valW, "…")
		}
		row := base.Foreground(colMuted).Render(fmt.Sprintf("%*d  ", numW, i+1)) +
			base.Bold(sel).Render(padRight(val, valW)) +
			base.Render(fmt.Sprintf("  %*s  %*.2f  %*.1f  ",
				freqW, formatInt(int(it.Freq)), ipmW, it.IPM, pctW, pct)) +
			base.Foreground(colAccent).Render(padRight(bar(it.Freq, maxFreq, barW), barW))
		b.WriteString(marker + row)
		if i < to-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (v *freqsView) summary() string {
	r := v.resp
	caseInfo := "ignored"
	if v.matchCase {
		caseInfo = "respected"
	}
	items := fmt.Sprintf("%d items", len(r.Freqs))
	if len(r.Freqs) >= v.maxItems {
		items += fmt.Sprintf(" (limit %d, + for more)", v.maxItems)
	}
	parts := []string{
		boldStyle.Render(formatInt(int(r.ConcSize))) + " hits",
		"attr: " + accentStyle.Render(v.attr),
		"case: " + caseInfo,
		"min freq: " + accentStyle.Render(fmt.Sprint(v.flimit)),
		items,
		"sorted by " + v.sortBy.String(),
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

// bar renders a horizontal bar with 1/8 cell resolution
func bar(val, maxVal int64, width int) string {
	if maxVal <= 0 || width <= 0 {
		return ""
	}
	eighths := int(math.Round(float64(val) / float64(maxVal) * float64(width*8)))
	if eighths == 0 && val > 0 {
		eighths = 1
	}
	partial := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	return strings.Repeat("█", eighths/8) + partial[eighths%8]
}

func (v *freqsView) Help() string {
	if v.picker != nil {
		return v.picker.Help()
	}
	return "↑/↓ move • enter concordance • a attribute • f min freq • m/+/- max items • i case • s sort • r reload • esc edit query • q quit"
}
