package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/czcorpus/mquery-common/concordance"
)

const (
	concSummaryHeight = 2 // summary line + empty line
	detailMaxColWidth = 40
)

// concViewSeq provides unique IDs so that responses for an already
// closed view are ignored
var concViewSeq int

type concLoadedMsg struct {
	viewID int
	reqID  int
	resp   *api.ConcordanceResponse
	err    error
}

type concAttrsMsg struct {
	viewID   int
	corpusID string
	attrs    []string
}

type concView struct {
	id     int
	env    *Env
	corpus api.CorpusItem
	query  string
	width  int
	height int

	spinner    spinner.Model
	loading    bool
	err        error
	resp       *api.ConcordanceResponse
	rowsOffset int
	reqID      int

	// infoAttrs is the corpus attribute order as provided by /info
	infoAttrs []string
	// attrs are positional attributes (except `word`) available in the result
	attrs  []string
	cursor int
	scroll int
	// dispAttr: 0 = word, i > 0 => attrs[i-1]
	dispAttr int

	detail   bool
	detailVP viewport.Model

	ctx    context.Context
	cancel context.CancelFunc
}

func newConcView(env *Env, corpus api.CorpusItem, query string) ResultView {
	concViewSeq++
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &concView{
		id:        concViewSeq,
		env:       env,
		corpus:    corpus,
		query:     query,
		spinner:   sp,
		infoAttrs: env.attrCache[corpus.ID],
		detailVP:  viewport.New(0, 0),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (v *concView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.fetch()}
	if v.infoAttrs == nil {
		cmds = append(cmds, v.fetchAttrs())
	}
	return tea.Batch(cmds...)
}

func (v *concView) Close() {
	v.cancel()
}

func (v *concView) SetSize(w, h int) {
	v.width, v.height = w, h
	v.detailVP.Width = w
	v.detailVP.Height = max(1, h-1)
	if v.detail {
		v.detailVP.SetContent(v.detailContent())
	}
	v.fixScroll()
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
	}
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			resp, err := client.Concordance(ctx, corpusID, args)
			return concLoadedMsg{viewID: viewID, reqID: reqID, resp: resp, err: err}
		},
	)
}

// fetchAttrs loads the corpus attribute order. It is not essential,
// so errors are ignored (we fall back to alphabetical order).
func (v *concView) fetchAttrs() tea.Cmd {
	client, ctx, corpusID, lang, viewID := v.env.Client, v.ctx, v.corpus.ID, v.env.Lang, v.id
	return func() tea.Msg {
		info, err := client.CorpusInfo(ctx, corpusID, lang)
		if err != nil {
			return nil
		}
		attrs := make([]string, len(info.AttrList))
		for i, a := range info.AttrList {
			attrs[i] = a.Name
		}
		return concAttrsMsg{viewID: viewID, corpusID: corpusID, attrs: attrs}
	}
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

func (v *concView) fixScroll() {
	n := len(v.lines())
	if v.cursor >= n {
		v.cursor = max(0, n-1)
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	h := v.listHeight()
	if v.cursor < v.scroll {
		v.scroll = v.cursor
	}
	if v.cursor >= v.scroll+h {
		v.scroll = v.cursor - h + 1
	}
	v.scroll = max(0, min(v.scroll, max(0, n-h)))
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
		v.cursor, v.scroll = 0, 0
		v.updateAttrs()
		return v, nil

	case concAttrsMsg:
		if msg.viewID != v.id {
			return v, nil
		}
		v.env.attrCache[msg.corpusID] = msg.attrs
		v.infoAttrs = msg.attrs
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
		if v.detail {
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
	switch msg.String() {
	case "up", "k":
		v.cursor--
	case "down", "j":
		v.cursor++
	case "pgup", "ctrl+u":
		v.cursor -= v.listHeight()
	case "pgdown", "ctrl+d":
		v.cursor += v.listHeight()
	case "home", "g":
		v.cursor = 0
	case "end", "G":
		v.cursor = len(v.lines()) - 1
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
		v.dispAttr = (v.dispAttr + 1) % (len(v.attrs) + 1)
	case "A":
		v.dispAttr = (v.dispAttr + len(v.attrs)) % (len(v.attrs) + 1)
	case "enter":
		if len(v.lines()) > 0 {
			v.openDetail()
		}
	}
	v.fixScroll()
	return v, nil
}

func (v *concView) openDetail() {
	v.detail = true
	v.detailVP.SetContent(v.detailContent())
	v.detailVP.GotoTop()
}

func (v *concView) updateDetail(msg tea.KeyMsg) (ResultView, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "backspace":
		v.detail = false
		return v, nil
	case "q":
		return v, tea.Quit
	case "left", "h":
		if v.cursor > 0 {
			v.cursor--
			v.fixScroll()
			v.openDetail()
		}
		return v, nil
	case "right", "l":
		if v.cursor < len(v.lines())-1 {
			v.cursor++
			v.fixScroll()
			v.openDetail()
		}
		return v, nil
	}
	var cmd tea.Cmd
	v.detailVP, cmd = v.detailVP.Update(msg)
	return v, cmd
}

// ---------------------------------- rendering

func (v *concView) View() string {
	if v.err != nil {
		return errorStyle.Render(v.err.Error()) + "\n\n" +
			mutedStyle.Render("r retry • esc edit query")
	}
	if v.resp == nil {
		return v.spinner.View() + " searching…"
	}
	if v.detail {
		return v.detailView()
	}
	var b strings.Builder
	b.WriteString(v.summary() + "\n\n")
	lines := v.lines()
	if len(lines) == 0 {
		b.WriteString(mutedStyle.Render("  No results."))
		return b.String()
	}
	numW := len(fmt.Sprint(v.rowsOffset + len(lines)))
	end := min(len(lines), v.scroll+v.listHeight())
	for i := v.scroll; i < end; i++ {
		b.WriteString(v.renderRow(i, lines[i], i == v.cursor, numW))
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

func (v *concView) detailView() string {
	title := fmt.Sprintf("  %s  line %s of %s",
		titleStyle.Render("Line detail"),
		formatInt(v.rowsOffset+v.cursor+1), formatInt(v.resp.ConcSize))
	return ansi.Truncate(title, v.width, "…") + "\n" + v.detailVP.View()
}

func (v *concView) detailContent() string {
	lines := v.lines()
	if v.cursor >= len(lines) {
		return ""
	}
	line := lines[v.cursor]
	var b strings.Builder
	b.WriteString("\n")
	if line.Ref != "" {
		b.WriteString("  " + mutedStyle.Render("ref: ") + line.Ref + "\n\n")
	}
	if line.ErrMsg != "" {
		b.WriteString("  " + errorStyle.Render("Error: ") +
			lipgloss.NewStyle().Width(max(20, v.width-4)).Render(line.ErrMsg) + "\n\n")
	}

	if len(line.Props) > 0 {
		b.WriteString("  " + boldStyle.Render("Text properties") + "\n")
		keys := make([]string, 0, len(line.Props))
		keyW := 0
		for k := range line.Props {
			keys = append(keys, k)
			keyW = max(keyW, ansi.StringWidth(k))
		}
		sort.Strings(keys)
		valIndent := 4 + keyW + 2
		valStyle := lipgloss.NewStyle().Width(max(20, v.width-valIndent-1))
		for _, k := range keys {
			val := valStyle.Render(line.Props[k])
			val = strings.ReplaceAll(strings.TrimRight(val, " \n"), "\n", "\n"+strings.Repeat(" ", valIndent))
			b.WriteString("    " + mutedStyle.Render(padRight(k, keyW)) + "  " + val + "\n")
		}
		b.WriteString("\n")
	}

	toks := line.Text.Tokens()
	if len(toks) == 0 {
		return b.String()
	}
	b.WriteString("  " + boldStyle.Render("Tokens") + "\n")
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
		r.WriteString("  " + marker + " ")
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
	if v.detail {
		return "↑/↓ scroll • ←/→ prev/next line • esc close detail • q quit"
	}
	return "↑/↓ move • enter line detail • a/A switch attribute • ←/→ page • r reload • esc edit query • q quit"
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
