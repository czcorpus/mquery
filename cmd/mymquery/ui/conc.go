package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/czcorpus/mquery-common/concordance"
)

const (
	concSummaryHeight = 2 // summary line + empty line
	detailMaxColWidth = 40
)

type concLoadedMsg struct {
	viewID int
	reqID  int
	resp   *api.ConcordanceResponse
	err    error
}

type concView struct {
	id     int
	env    *Env
	corpus api.CorpusItem
	query  string
	// note is an optional info shown in the summary line
	note string
	// coll is an optional collocate filter (CQL) with a range relative to KWIC
	coll      string
	collRange [2]int
	width     int
	height    int

	spinner    spinner.Model
	loading    bool
	err        error
	resp       *api.ConcordanceResponse
	rowsOffset int
	reqID      int

	// infoAttrs is the corpus attribute order as provided by /info
	infoAttrs []string
	// attrs are positional attributes (except `word`) available in the result
	attrs []string
	nav   listNav
	// dispAttr: 0 = word, i > 0 => attrs[i-1]
	dispAttr int
	// attrPicker is open when selecting the displayed attribute
	attrPicker *selectList

	// detail is a modal with the selected line's tokens or text properties
	detail     *modal
	detailMode detailMode

	ctx    context.Context
	cancel context.CancelFunc
}

func newConcView(env *Env, corpus api.CorpusItem, query string) ResultView {
	return newConcViewWithNote(env, corpus, query, "")
}

func newConcViewWithNote(env *Env, corpus api.CorpusItem, query, note string) *concView {
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &concView{
		id:        nextViewID(),
		env:       env,
		corpus:    corpus,
		query:     query,
		note:      note,
		spinner:   sp,
		infoAttrs: attrNames(env.attrCache[corpus.ID]),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (v *concView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.fetch()}
	if v.infoAttrs == nil {
		cmds = append(cmds, fetchCorpusAttrs(v.env, v.ctx, v.corpus.ID, v.id))
	}
	return tea.Batch(cmds...)
}

// withColl restricts the concordance to lines with a collocate matching `coll`
// within the range [left, right] (relative to KWIC, negative = left)
func (v *concView) withColl(coll string, left, right int) *concView {
	v.coll = coll
	v.collRange = [2]int{left, right}
	return v
}

func (v *concView) Title() string { return "Concordance" }

func (v *concView) Query() string { return v.query }

func (v *concView) Close() {
	v.cancel()
}

func (v *concView) SetSize(w, h int) {
	v.width, v.height = w, h
	if v.detail != nil {
		v.detail.SetSize(w, h)
	}
	v.nav.fix(len(v.lines()), v.listHeight())
}

func (v *concView) fetch() tea.Cmd {
	v.reqID++
	v.loading = true
	v.err = nil
	reqID, viewID := v.reqID, v.id
	client, ctx, corpusID := v.env.Client, v.ctx, v.corpus.ID
	args := api.ConcordanceArgs{
		Query:         v.query,
		RowsOffset:    v.rowsOffset,
		MaxRows:       v.env.PageSize,
		ContextWidth:  v.env.ContextWidth,
		ShowTextProps: 2,
		Coll:          v.coll,
		CollRange:     v.collRange,
	}
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			resp, err := client.Concordance(ctx, corpusID, args)
			return concLoadedMsg{viewID: viewID, reqID: reqID, resp: resp, err: err}
		},
	)
}

func (v *concView) lines() []concordance.Line {
	if v.resp == nil {
		return nil
	}
	return v.resp.Lines
}

func (v *concView) listHeight() int {
	return max(1, v.height-concSummaryHeight)
}

func (v *concView) updateAttrs() {
	present := make(map[string]bool)
	for _, line := range v.lines() {
		for _, t := range line.Text.Tokens() {
			for k := range t.Attrs {
				present[k] = true
			}
		}
	}
	delete(present, "word")
	ans := make([]string, 0, len(present))
	for _, a := range v.infoAttrs {
		if present[a] {
			ans = append(ans, a)
			delete(present, a)
		}
	}
	rest := make([]string, 0, len(present))
	for a := range present {
		rest = append(rest, a)
	}
	sort.Strings(rest)
	v.attrs = append(ans, rest...)
	if v.dispAttr > len(v.attrs) {
		v.dispAttr = 0
	}
}

func (v *concView) dispAttrName() string {
	if v.dispAttr == 0 {
		return "word"
	}
	return v.attrs[v.dispAttr-1]
}

func (v *concView) hasNextPage() bool {
	return v.resp != nil && v.rowsOffset+len(v.resp.Lines) < v.resp.ConcSize
}

func (v *concView) Update(msg tea.Msg) (ResultView, tea.Cmd) {
	switch msg := msg.(type) {
	case concLoadedMsg:
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
		v.nav.reset()
		v.updateAttrs()
		return v, nil

	case corpusAttrsMsg:
		if msg.viewID != v.id || msg.err != nil {
			return v, nil // attr order is not essential; we fall back to alphabetical
		}
		v.env.attrCache[msg.corpusID] = msg.attrs
		v.infoAttrs = attrNames(msg.attrs)
		v.updateAttrs()
		return v, nil

	case spinner.TickMsg:
		if !v.loading {
			return v, nil
		}
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(msg)
		return v, cmd

	case tea.KeyMsg:
		if v.attrPicker != nil {
			return v.updateAttrPicker(msg)
		}
		if v.detail != nil {
			return v.updateDetail(msg)
		}
		return v.updateList(msg)
	}
	return v, nil
}

func (v *concView) updateList(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return v, goBack
	case "q":
		return v, tea.Quit
	case "r":
		return v, v.fetch()
	}
	if v.loading && v.resp == nil {
		return v, nil
	}
	if v.nav.handleKey(msg.String(), len(v.lines()), v.listHeight()) {
		return v, nil
	}
	switch msg.String() {
	case "right", "l", "n":
		if v.hasNextPage() && !v.loading {
			v.rowsOffset += v.env.PageSize
			return v, v.fetch()
		}
	case "left", "h", "p":
		if v.rowsOffset > 0 && !v.loading {
			v.rowsOffset = max(0, v.rowsOffset-v.env.PageSize)
			return v, v.fetch()
		}
	case "a":
		names := append([]string{"word"}, v.attrs...)
		v.attrPicker = newSelectList(
			"Displayed attribute", attrSelectItems(names, v.env, v.corpus.ID), v.dispAttrName())
	case "enter":
		v.openDetail(detailTokens)
	case "t":
		v.openDetail(detailTextTypes)
	}
	return v, nil
}

func (v *concView) openDetail(mode detailMode) {
	if len(v.lines()) == 0 {
		return
	}
	if v.detail == nil {
		v.detail = newModal("", func(int) string { return "" })
		v.detail.SetSize(v.width, v.height)
	}
	v.detailMode = mode
	v.refreshDetail()
}

func (v *concView) refreshDetail() {
	pos := fmt.Sprintf("line %s of %s",
		formatInt(v.rowsOffset+v.nav.cursor+1), formatInt(v.resp.ConcSize))
	switch v.detailMode {
	case detailTextTypes:
		v.detail.SetContent("Text types", pos, v.textTypesContent)
	default:
		v.detail.SetContent("Tokens", pos, v.tokensContent)
	}
}

func (v *concView) updateDetail(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		v.detail = nil
		return v, nil
	case "enter", "t":
		// the key which opens a detail also closes it (or switches to it from the other one)
		mode := detailTokens
		if msg.String() == "t" {
			mode = detailTextTypes
		}
		if mode == v.detailMode {
			v.detail = nil
		} else {
			v.openDetail(mode)
		}
		return v, nil
	case "q":
		return v, tea.Quit
	case "left", "h":
		if v.nav.cursor > 0 {
			v.nav.cursor--
			v.nav.fix(len(v.lines()), v.listHeight())
			v.refreshDetail()
		}
		return v, nil
	case "right", "l":
		if v.nav.cursor < len(v.lines())-1 {
			v.nav.cursor++
			v.nav.fix(len(v.lines()), v.listHeight())
			v.refreshDetail()
		}
		return v, nil
	}
	return v, v.detail.Update(msg)
}

// ---------------------------------- rendering

func (v *concView) updateAttrPicker(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	res, val := v.attrPicker.Update(msg)
	if res == selectChosen {
		v.dispAttr = 0
		for i, a := range v.attrs {
			if a == val {
				v.dispAttr = i + 1
			}
		}
	}
	if res != selectPending {
		v.attrPicker = nil
	}
	return v, nil
}

func (v *concView) View() string {
	body := v.baseView()
	if v.attrPicker != nil {
		return placeOverlay(body, v.attrPicker.View(v.width), v.width, v.height)
	}
	if v.detail != nil {
		return v.detail.View(body)
	}
	return body
}

func (v *concView) baseView() string {
	if v.err != nil {
		return errorStyle.Render(v.err.Error()) + "\n\n" +
			mutedStyle.Render("r retry • esc edit query")
	}
	if v.resp == nil {
		return v.spinner.View() + " searching…"
	}
	var b strings.Builder
	b.WriteString(v.summary() + "\n\n")
	lines := v.lines()
	if len(lines) == 0 {
		b.WriteString(mutedStyle.Render("  No results."))
		return b.String()
	}
	numW := len(fmt.Sprint(v.rowsOffset + len(lines)))
	from, end := v.nav.visibleRange(len(lines), v.listHeight())
	for i := from; i < end; i++ {
		b.WriteString(v.renderRow(i, lines[i], i == v.nav.cursor, numW))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (v *concView) summary() string {
	r := v.resp
	pages := (r.ConcSize + v.env.PageSize - 1) / v.env.PageSize
	page := v.rowsOffset/v.env.PageSize + 1
	parts := []string{
		boldStyle.Render(formatInt(r.ConcSize)) + " hits",
		fmt.Sprintf("%.2f ipm", r.IPM),
		fmt.Sprintf("lines %d–%d", v.rowsOffset+min(1, len(r.Lines)), v.rowsOffset+len(r.Lines)),
		fmt.Sprintf("page %d/%d", page, max(1, pages)),
		"showing: " + accentStyle.Render(v.dispAttrName()),
	}
	if v.note != "" {
		parts = append(parts, mutedStyle.Render(v.note))
	}
	s := "  " + strings.Join(parts, mutedStyle.Render(" • "))
	if v.loading {
		s += "  " + v.spinner.View()
	}
	return ansi.Truncate(s, v.width, "…")
}

func (v *concView) renderRow(idx int, line concordance.Line, selected bool, numW int) string {
	base := lipgloss.NewStyle()
	marker := base.Render("  ")
	if selected {
		base = base.Background(colSelBg)
		marker = accentStyle.Bold(true).Render("❯ ")
	}
	num := base.Foreground(colMuted).Render(fmt.Sprintf("%*d ", numW, v.rowsOffset+idx+1))
	avail := max(20, v.width-2-numW-2)
	var content string
	if line.ErrMsg != "" {
		content = fitRight(base.Foreground(colError).Render("⚠ "+line.ErrMsg), avail, base)

	} else {
		content = v.kwicLayout(line, base, avail)
	}
	return marker + num + content
}

// kwicLayout renders a line with KWIC starting at a fixed column
// (left context is right-aligned, right context is left-aligned).
func (v *concView) kwicLayout(line concordance.Line, base lipgloss.Style, avail int) string {
	toks := line.Text.Tokens()
	kFirst, kLast := -1, -1
	for i, t := range toks {
		if t.MatchType == concordance.MatchTypeKWIC {
			if kFirst < 0 {
				kFirst = i
			}
			kLast = i
		}
	}
	attr := v.dispAttrName()
	if kFirst < 0 {
		return fitRight(renderTokens(toks, base, attr), avail, base)
	}
	leftW := max(5, (avail-2)*2/5)
	kwic := renderTokens(toks[kFirst:kLast+1], base, attr)
	maxKwic := max(1, avail-leftW-2-5)
	if ansi.StringWidth(kwic) > maxKwic {
		kwic = ansi.Truncate(kwic, maxKwic, "…")
	}
	rightW := max(0, avail-leftW-2-ansi.StringWidth(kwic))
	sp := base.Render(" ")
	return fitLeft(renderTokens(toks[:kFirst], base, attr), leftW, base) +
		sp + kwic + sp +
		fitRight(renderTokens(toks[kLast+1:], base, attr), rightW, base)
}

func tokenValue(t *concordance.Token, attr string) string {
	if attr == "word" {
		return t.Word
	}
	if val, ok := t.Attrs[attr]; ok {
		return val
	}
	return t.Word
}

func renderTokens(toks []*concordance.Token, base lipgloss.Style, attr string) string {
	var b strings.Builder
	sp := base.Render(" ")
	for i, t := range toks {
		if i > 0 {
			b.WriteString(sp)
		}
		st := base
		switch t.MatchType {
		case concordance.MatchTypeKWIC:
			st = kwicStyle.Background(base.GetBackground())
		case concordance.MatchTypeColl:
			st = collStyle.Background(base.GetBackground())
		}
		b.WriteString(st.Render(tokenValue(t, attr)))
	}
	return b.String()
}

// fitLeft makes `s` exactly `w` cells wide - it is padded/truncated on the left side
func fitLeft(s string, w int, base lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		s = base.Render("…") + ansi.TruncateLeft(s, sw-w+1, "")
		sw = w
	}
	return base.Render(strings.Repeat(" ", w-sw)) + s
}

// fitRight makes `s` exactly `w` cells wide - it is padded/truncated on the right side
func fitRight(s string, w int, base lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		s = ansi.Truncate(s, w-1, "") + base.Render("…")
		sw = w
	}
	return s + base.Render(strings.Repeat(" ", w-sw))
}

type detailMode int

const (
	detailTokens detailMode = iota
	detailTextTypes
)

func (v *concView) selectedLine() (concordance.Line, bool) {
	lines := v.lines()
	if v.nav.cursor >= len(lines) {
		return concordance.Line{}, false
	}
	return lines[v.nav.cursor], true
}

func (v *concView) textTypesContent(width int) string {
	line, ok := v.selectedLine()
	if !ok {
		return ""
	}
	var b strings.Builder
	if line.Ref != "" {
		b.WriteString(mutedStyle.Render("ref: ") + line.Ref + "\n\n")
	}
	if len(line.Props) == 0 {
		b.WriteString(mutedStyle.Render("No text properties available for this line."))
		return b.String()
	}
	keys := make([]string, 0, len(line.Props))
	keyW := 0
	for k := range line.Props {
		keys = append(keys, k)
		keyW = max(keyW, ansi.StringWidth(k))
	}
	sort.Strings(keys)
	valIndent := keyW + 2
	valStyle := lipgloss.NewStyle().Width(max(20, width-valIndent))
	for _, k := range keys {
		val := valStyle.Render(line.Props[k])
		val = strings.ReplaceAll(strings.TrimRight(val, " \n"), "\n", "\n"+strings.Repeat(" ", valIndent))
		b.WriteString(mutedStyle.Render(padRight(k, keyW)) + "  " + val + "\n")
	}
	return b.String()
}

func (v *concView) tokensContent(width int) string {
	line, ok := v.selectedLine()
	if !ok {
		return ""
	}
	var b strings.Builder
	if line.ErrMsg != "" {
		b.WriteString(errorStyle.Render("Error: ") +
			lipgloss.NewStyle().Width(max(20, width-7)).Render(line.ErrMsg) + "\n\n")
	}
	toks := line.Text.Tokens()
	if len(toks) == 0 {
		return b.String()
	}
	cols := append([]string{"word"}, v.attrs...)
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = ansi.StringWidth(c)
	}
	for _, t := range toks {
		for i, c := range cols {
			widths[i] = min(detailMaxColWidth, max(widths[i], ansi.StringWidth(tokenValue(t, c))))
		}
	}
	numW := len(fmt.Sprint(len(toks)))
	row := func(cells []string, st lipgloss.Style, marker string) string {
		var r strings.Builder
		r.WriteString(marker + " ")
		for i, c := range cells {
			if i > 0 {
				r.WriteString("  ")
			}
			r.WriteString(st.Render(padRight(ansi.Truncate(c, widths[i], "…"), widths[i])))
		}
		return strings.TrimRight(r.String(), " ") + "\n"
	}
	b.WriteString(row(cols, mutedStyle.Underline(true), strings.Repeat(" ", numW+1)))
	for i, t := range toks {
		cells := make([]string, len(cols))
		for j, c := range cols {
			cells[j] = tokenValue(t, c)
		}
		st := lipgloss.NewStyle()
		marker := mutedStyle.Render(fmt.Sprintf("%*d ", numW, i+1))
		switch t.MatchType {
		case concordance.MatchTypeKWIC:
			st = kwicStyle
			marker = kwicStyle.Render(fmt.Sprintf("%*d▶", numW, i+1))
		case concordance.MatchTypeColl:
			st = collStyle
		}
		b.WriteString(row(cells, st, marker))
	}
	return b.String()
}

func (v *concView) Help() string {
	if v.attrPicker != nil {
		return v.attrPicker.Help()
	}
	if v.detail != nil {
		other := "t text types"
		if v.detailMode == detailTextTypes {
			other = "enter tokens"
		}
		return v.detail.Help() + " • ←/→ prev/next line • " + other + " • esc close • q quit"
	}
	return "↑/↓ move • enter tokens • t text types • a displayed attribute • ←/→ page • r reload • esc edit query • q quit"
}

// ---------------------------------- helpers

func padRight(s string, w int) string {
	if sw := ansi.StringWidth(s); sw < w {
		return s + strings.Repeat(" ", w-sw)
	}
	return s
}

func formatInt(n int) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + formatInt(-n)
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
