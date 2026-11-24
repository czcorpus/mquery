package ui

import (
	"strings"
	"testing"

	"mquery/cmd/mymquery/api"

	"github.com/charmbracelet/x/ansi"
)

func TestHeaderCell(t *testing.T) {
	cases := []struct {
		label string
		w     int
		right bool
		dir   sortDir
		want  string
	}{
		{"freq", 8, true, sortNone, "    freq"},
		{"freq", 8, true, sortDesc, "  freq ▼"},
		{"lemma", 8, false, sortAsc, "lemma ▲ "},
		{"doc.txtype", 6, false, sortAsc, "doc… ▲"},
	}
	for _, c := range cases {
		got := ansi.Strip(headerCell(c.label, c.w, c.right, c.dir))
		if got != c.want {
			t.Errorf("headerCell(%q, %d, %v, %v) = %q, want %q", c.label, c.w, c.right, c.dir, got, c.want)
		}
	}
}

func TestFreqTableSortMarkAlignment(t *testing.T) {
	tbl := newFreqTable(freqSortByFreq, freqSortByIPM, freqSortByValue)
	tbl.valueHeader = "lemma"
	tbl.baseHeader = "text size"
	tbl.setData([]api.FreqItem{{Value: "a", Freq: 12345, Base: 100000, IPM: 1.5}}, 20000)
	for range tbl.sortModes {
		lines := strings.Split(ansi.Strip(tbl.view(100, 10)), "\n")
		hdr, row := lines[0], lines[1]
		if n := strings.Count(hdr, "▼") + strings.Count(hdr, "▲"); n != 1 {
			t.Fatalf("sort %v: expected exactly one sort mark, got header %q", tbl.sortBy(), hdr)
		}
		// the end of the freq column must be aligned in the header and the row
		freqEnd := strings.Index(row, "12,345") + len("12,345")
		hdrFreqEnd := strings.Index(hdr, "freq") + len("freq")
		if tbl.sortBy() == freqSortByFreq {
			hdrFreqEnd += len(" ▼")
		}
		if ansi.StringWidth(row[:freqEnd]) != ansi.StringWidth(hdr[:hdrFreqEnd]) {
			t.Errorf("sort %v: misaligned freq column\n%s\n%s", tbl.sortBy(), hdr, row)
		}
		tbl.cycleSort()
	}
}
