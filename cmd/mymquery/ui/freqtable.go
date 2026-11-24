package ui

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Shared parts of frequency based views (token freqs, text types)

const freqTableMinBarWidth = 10

var (
	freqMaxItemsSteps = []int{20, 50, 100, 200, 500, 1000}
	freqFLimitPresets = []int{1, 2, 3, 5, 10, 20, 50, 100, 500, 1000}
)

const (
	freqMaxItemsLimit = 10000
	freqFLimitLimit   = 1_000_000_000
)

func newFLimitPicker(current int) *selectList {
	return newMinFreqPicker("Minimum frequency", current)
}

func newMinFreqPicker(title string, current int) *selectList {
	return newSelectList(title, intSelectItems(freqFLimitPresets), fmt.Sprint(current)).
		withCustom("filter or type a number", parsePositiveInt(1, freqFLimitLimit))
}

func newMaxItemsPicker(current int) *selectList {
	return newSelectList("Max. items", intSelectItems(freqMaxItemsSteps), fmt.Sprint(current)).
		withCustom("filter or type a number", parsePositiveInt(1, freqMaxItemsLimit))
}

// stepMaxItems returns the next (dir > 0) or previous (dir < 0) preset
// relative to `current` (which may be a custom value between the presets)
func stepMaxItems(current, dir int) (int, bool) {
	if dir > 0 {
		for _, s := range freqMaxItemsSteps {
			if s > current {
				return s, true
			}
		}
		return current, false
	}
	for i := len(freqMaxItemsSteps) - 1; i >= 0; i-- {
		if s := freqMaxItemsSteps[i]; s < current {
			return s, true
		}
	}
	return current, false
}

type freqSort int

const (
	freqSortByFreq freqSort = iota
	freqSortByIPM
	freqSortByValue
)

func (s freqSort) String() string {
	switch s {
	case freqSortByIPM:
		return "ipm"
	case freqSortByValue:
		return "value"
	default:
		return "freq"
	}
}

// ---------------------------------- sortable table headers

type sortDir int

const (
	sortNone sortDir = iota
	sortDesc
	sortAsc
)

// sortMarkW is the width of the sort mark incl. the separating space
const sortMarkW = 2

var (
	colHeadStyle       = mutedStyle.Underline(true)
	colHeadSortedStyle = accentStyle.Bold(true).Underline(true)
)

// sortedLabelW returns the width a column label needs (sortable columns
// always reserve space for the mark so the layout does not jump when
// the sorting changes)
func sortedLabelW(label string, sortable bool) int {
	w := ansi.StringWidth(label)
	if sortable {
		w += sortMarkW
	}
	return w
}

// headerCell renders a column label padded to `w` cells (right aligned
// if alignRight). A sorted column is highlighted and gets a direction mark.
func headerCell(label string, w int, alignRight bool, dir sortDir) string {
	style := colHeadStyle
	if dir != sortNone {
		style = colHeadSortedStyle
		mark := " ▼"
		if dir == sortAsc {
			mark = " ▲"
		}
		if ansi.StringWidth(label)+sortMarkW > w {
			label = ansi.Truncate(label, max(1, w-sortMarkW), "…")
		}
		label += mark
	} else if ansi.StringWidth(label) > w {
		label = ansi.Truncate(label, w, "…")
	}
	pad := colHeadStyle.Render(strings.Repeat(" ", max(0, w-ansi.StringWidth(label))))
	if alignRight {
		return pad + style.Render(label)
	}
	return style.Render(label) + pad
}

// freqTable renders a scrollable frequency table with a bar chart
type freqTable struct {
	valueHeader string
	// baseHeader (if set) adds a column with the item base (e.g. text type size)
	baseHeader string
	sortModes  []freqSort
	sortIdx    int
	concSize   int64
	raw        []api.FreqItem
	items      []api.FreqItem
	nav        listNav
}

func newFreqTable(sortModes ...freqSort) freqTable {
	return freqTable{sortModes: sortModes}
}

func (t *freqTable) sortBy() freqSort {
	return t.sortModes[t.sortIdx]
}

func (t *freqTable) sortable(s freqSort) bool {
	return slices.Contains(t.sortModes, s)
}

// sortDir returns the sort direction mark for a column sorted by `s`
func (t *freqTable) sortDir(s freqSort) sortDir {
	switch {
	case t.sortBy() != s:
		return sortNone
	case s == freqSortByValue:
		return sortAsc
	default:
		return sortDesc
	}
}

func (t *freqTable) setData(items []api.FreqItem, concSize int64) {
	t.raw = items
	t.concSize = concSize
	t.applySort()
}

func (t *freqTable) cycleSort() {
	t.sortIdx = (t.sortIdx + 1) % len(t.sortModes)
	t.applySort()
}

func (t *freqTable) applySort() {
	t.items = slices.Clone(t.raw)
	var less func(a, b api.FreqItem) bool
	switch t.sortBy() {
	case freqSortByFreq:
		less = func(a, b api.FreqItem) bool { return a.Freq > b.Freq }
	case freqSortByIPM:
		less = func(a, b api.FreqItem) bool { return a.IPM > b.IPM }
	case freqSortByValue:
		less = func(a, b api.FreqItem) bool { return a.Value < b.Value }
	}
	sort.SliceStable(t.items, func(i, j int) bool { return less(t.items[i], t.items[j]) })
	t.nav.reset()
}

func (t *freqTable) selected() (api.FreqItem, bool) {
	if len(t.items) == 0 {
		return api.FreqItem{}, false
	}
	return t.items[t.nav.cursor], true
}

// rowsHeight is the number of data rows for the total table height (incl. header)
func (t *freqTable) rowsHeight(height int) int {
	return max(1, height-1)
}

func (t *freqTable) handleKey(key string, height int) bool {
	return t.nav.handleKey(key, len(t.items), t.rowsHeight(height))
}

func (t *freqTable) fix(height int) {
	t.nav.fix(len(t.items), t.rowsHeight(height))
}

// csvRows returns the items (in the current order) incl. a header row
// with raw (unformatted) numbers
func (t *freqTable) csvRows() [][]string {
	hdr := []string{t.valueHeader, "freq"}
	if t.baseHeader != "" {
		hdr = append(hdr, t.baseHeader)
	}
	rows := [][]string{append(hdr, "ipm", "pct_of_hits")}
	for _, it := range t.items {
		row := []string{it.Value, fmtCSVInt(it.Freq)}
		if t.baseHeader != "" {
			row = append(row, fmtCSVInt(it.Base))
		}
		rows = append(rows, append(row, fmtCSVFloat(it.IPM), fmtCSVFloat(pctOf(it.Freq, t.concSize))))
	}
	return rows
}

func freqValueLabel(v string) string {
	if v == "" {
		return "(empty)"
	}
	return v
}

// view renders the header and visible rows. The bar shows ipm if the table
// is sorted by ipm, otherwise the absolute frequency.
func (t *freqTable) view(width, height int) string {
	if len(t.items) == 0 {
		return mutedStyle.Render("  No results.")
	}
	byIPM := t.sortBy() == freqSortByIPM
	numW := len(fmt.Sprint(len(t.items)))
	valW := sortedLabelW(t.valueHeader, t.sortable(freqSortByValue))
	var maxFreq, maxBase int64
	var maxIPM float64
	for _, it := range t.items {
		valW = max(valW, ansi.StringWidth(freqValueLabel(it.Value)))
		maxFreq = max(maxFreq, it.Freq)
		maxBase = max(maxBase, it.Base)
		maxIPM = max(maxIPM, it.IPM)
	}
	freqW := max(sortedLabelW("freq", t.sortable(freqSortByFreq)), len(formatInt(int(maxFreq))))
	baseW := 0
	if t.baseHeader != "" {
		baseW = max(ansi.StringWidth(t.baseHeader), len(formatInt(int(maxBase))))
	}
	ipmW := max(8, sortedLabelW("ipm", t.sortable(freqSortByIPM)), len(fmt.Sprintf("%.2f", maxIPM)))
	const pctW = 6
	fixedW := 2 + numW + 2 + 2 + freqW + 2 + ipmW + 2 + pctW + 2
	if baseW > 0 {
		fixedW += baseW + 2
	}
	valW = min(valW, max(10, width-fixedW-freqTableMinBarWidth))
	barW := max(0, width-fixedW-valW-1)

	cell := func(w int, s string) string { return fmt.Sprintf("%*s  ", w, s) }
	sep := colHeadStyle.Render("  ")
	var hdr strings.Builder
	hdr.WriteString(colHeadStyle.Render(fmt.Sprintf("  %*s  ", numW, "#")))
	hdr.WriteString(headerCell(t.valueHeader, valW, false, t.sortDir(freqSortByValue)) + sep)
	hdr.WriteString(headerCell("freq", freqW, true, t.sortDir(freqSortByFreq)) + sep)
	if baseW > 0 {
		hdr.WriteString(headerCell(t.baseHeader, baseW, true, sortNone) + sep)
	}
	hdr.WriteString(headerCell("ipm", ipmW, true, t.sortDir(freqSortByIPM)) + sep)
	hdr.WriteString(headerCell("%", pctW, true, sortNone) + sep)
	if byIPM {
		hdr.WriteString(colHeadStyle.Render("(bar: ipm)"))
	}

	var b strings.Builder
	b.WriteString(hdr.String() + "\n")
	from, to := t.nav.visibleRange(len(t.items), t.rowsHeight(height))
	for i := from; i < to; i++ {
		it := t.items[i]
		sel := i == t.nav.cursor
		base := lipgloss.NewStyle()
		marker := "  "
		if sel {
			base = base.Background(colSelBg)
			marker = accentStyle.Bold(true).Render("❯ ")
		}
		pct := pctOf(it.Freq, t.concSize)
		valStyle := base.Bold(sel)
		if it.Value == "" {
			valStyle = valStyle.Foreground(colMuted).Italic(true)
		}
		val := freqValueLabel(it.Value)
		if ansi.StringWidth(val) > valW {
			val = ansi.Truncate(val, valW, "…")
		}
		var nums strings.Builder
		nums.WriteString("  " + cell(freqW, formatInt(int(it.Freq))))
		if baseW > 0 {
			nums.WriteString(cell(baseW, formatInt(int(it.Base))))
		}
		nums.WriteString(cell(ipmW, fmt.Sprintf("%.2f", it.IPM)))
		nums.WriteString(cell(pctW, fmt.Sprintf("%.1f", pct)))
		var barStr string
		if byIPM {
			barStr = bar(it.IPM, maxIPM, barW)
		} else {
			barStr = bar(float64(it.Freq), float64(maxFreq), barW)
		}
		row := base.Foreground(colMuted).Render(fmt.Sprintf("%*d  ", numW, i+1)) +
			valStyle.Render(padRight(val, valW)) +
			base.Render(nums.String()) +
			base.Foreground(colAccent).Render(padRight(barStr, barW))
		b.WriteString(marker + row)
		if i < to-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// bar renders a half-height horizontal bar (so it does not fill the whole
// row height) with 1/2 cell resolution
func bar(val, maxVal float64, width int) string {
	if maxVal <= 0 || width <= 0 {
		return ""
	}
	halves := int(math.Round(val / maxVal * float64(width*2)))
	if halves == 0 && val > 0 {
		halves = 1
	}
	ans := strings.Repeat("▄", halves/2)
	if halves%2 == 1 {
		ans += "▖"
	}
	return ans
}

// itemsInfo describes the number of items with respect to the limit
func itemsInfo(n, maxItems int) string {
	s := fmt.Sprintf("%d items", n)
	if n == 1 {
		s = "1 item"
	}
	if n >= maxItems {
		s += fmt.Sprintf(" (limit %d, + for more)", maxItems)
	}
	return s
}
