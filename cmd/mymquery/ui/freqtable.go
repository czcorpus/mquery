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
	valW := ansi.StringWidth(t.valueHeader)
	var maxFreq, maxBase int64
	var maxIPM float64
	for _, it := range t.items {
		valW = max(valW, ansi.StringWidth(freqValueLabel(it.Value)))
		maxFreq = max(maxFreq, it.Freq)
		maxBase = max(maxBase, it.Base)
		maxIPM = max(maxIPM, it.IPM)
	}
	freqW := max(4, len(formatInt(int(maxFreq))))
	baseW := 0
	if t.baseHeader != "" {
		baseW = max(ansi.StringWidth(t.baseHeader), len(formatInt(int(maxBase))))
	}
	ipmW := max(8, len(fmt.Sprintf("%.2f", maxIPM)))
	const pctW = 6
	fixedW := 2 + numW + 2 + 2 + freqW + 2 + ipmW + 2 + pctW + 2
	if baseW > 0 {
		fixedW += baseW + 2
	}
	valW = min(valW, max(10, width-fixedW-freqTableMinBarWidth))
	barW := max(0, width-fixedW-valW-1)

	cell := func(w int, s string) string { return fmt.Sprintf("%*s  ", w, s) }
	var hdr strings.Builder
	hdr.WriteString(fmt.Sprintf("  %*s  %s  ", numW, "#", padRight(t.valueHeader, valW)))
	hdr.WriteString(cell(freqW, "freq"))
	if baseW > 0 {
		hdr.WriteString(cell(baseW, t.baseHeader))
	}
	hdr.WriteString(cell(ipmW, "ipm"))
	hdr.WriteString(cell(pctW, "%"))
	if byIPM {
		hdr.WriteString("(bar: ipm)")
	}

	var b strings.Builder
	b.WriteString(mutedStyle.Underline(true).Render(hdr.String()) + "\n")
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
		var pct float64
		if t.concSize > 0 {
			pct = float64(it.Freq) / float64(t.concSize) * 100
		}
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

// bar renders a horizontal bar with 1/8 cell resolution
func bar(val, maxVal float64, width int) string {
	if maxVal <= 0 || width <= 0 {
		return ""
	}
	eighths := int(math.Round(val / maxVal * float64(width*8)))
	if eighths == 0 && val > 0 {
		eighths = 1
	}
	partial := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	return strings.Repeat("█", eighths/8) + partial[eighths%8]
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
