package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
)

// Markdown tables are rendered by us instead of glamour, which always
// stretches them to the full width (with almost empty columns in a chat).

// mdSegment is a part of a markdown text: either a table or other content
type mdSegment struct {
	text    string
	isTable bool
}

var tableSepRegexp = regexp.MustCompile(`^\s*\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?\s*$`)

func isTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// splitMDTables splits a markdown text into tables and the other parts.
// A table is a row starting with `|` followed by a separator row (`|---|`)
// and more rows starting with `|` (tables inside code blocks are respected).
func splitMDTables(text string) []mdSegment {
	lines := strings.Split(text, "\n")
	var ans []mdSegment
	var other []string
	flush := func() {
		if len(other) > 0 {
			ans = append(ans, mdSegment{text: strings.Join(other, "\n")})
			other = nil
		}
	}
	inCode := false
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			inCode = !inCode
		}
		if !inCode && isTableRow(lines[i]) && i+1 < len(lines) && tableSepRegexp.MatchString(lines[i+1]) &&
			strings.Contains(lines[i+1], "-") {
			j := i + 2
			for j < len(lines) && isTableRow(lines[j]) {
				j++
			}
			flush()
			ans = append(ans, mdSegment{text: strings.Join(lines[i:j], "\n"), isTable: true})
			i = j - 1
			continue
		}
		other = append(other, lines[i])
	}
	flush()
	return ans
}

// splitTableRow splits `| a | b |` into cells (`\|` is an escaped pipe)
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	line = strings.ReplaceAll(line, `\|`, "\x00")
	cells := strings.Split(line, "|")
	for i, c := range cells {
		cells[i] = strings.ReplaceAll(strings.TrimSpace(c), "\x00", "|")
	}
	return cells
}

var inlineMDRegexp = regexp.MustCompile("\\*\\*(.+?)\\*\\*|__(.+?)__|`([^`]+)`|\\*([^*]+)\\*")

// renderInlineMD renders the basic inline markup of a table cell
// (bold, code, italics) and highlights result references
func renderInlineMD(s string) string {
	s = inlineMDRegexp.ReplaceAllStringFunc(s, func(m string) string {
		sub := inlineMDRegexp.FindStringSubmatch(m)
		switch {
		case sub[1] != "":
			return boldStyle.Render(sub[1])
		case sub[2] != "":
			return boldStyle.Render(sub[2])
		case sub[3] != "":
			return accentStyle.Render(sub[3])
		default:
			return lipgloss.NewStyle().Italic(true).Render(sub[4])
		}
	})
	return resultRefRegexp.ReplaceAllStringFunc(s, func(r string) string {
		return kwicStyle.Render(r)
	})
}

// renderMDTable renders a markdown table at its natural width
// (or wrapped to maxWidth if it is wider)
func renderMDTable(text string, maxWidth int) string {
	lines := strings.Split(text, "\n")
	header := splitTableRow(lines[0])
	aligns := make([]lipgloss.Position, len(header))
	for i, spec := range splitTableRow(lines[1]) {
		if i >= len(aligns) {
			break
		}
		switch {
		case strings.HasPrefix(spec, ":") && strings.HasSuffix(spec, ":"):
			aligns[i] = lipgloss.Center
		case strings.HasSuffix(spec, ":"):
			aligns[i] = lipgloss.Right
		default:
			aligns[i] = lipgloss.Left
		}
	}
	for i, h := range header {
		header[i] = renderInlineMD(h)
	}
	var rows [][]string
	for _, line := range lines[2:] {
		cells := splitTableRow(line)
		row := make([]string, len(header))
		for i := range row {
			if i < len(cells) {
				row[i] = renderInlineMD(cells[i])
			}
		}
		rows = append(rows, row)
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderStyle(mutedStyle).
		Headers(header...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			st := lipgloss.NewStyle().Padding(0, 1)
			if col < len(aligns) {
				st = st.Align(aligns[col])
			}
			if row == table.HeaderRow {
				st = st.Bold(true)
			}
			return st
		})
	out := t.String()
	if ansi.StringWidth(strings.SplitN(out, "\n", 2)[0]) > maxWidth {
		out = t.Width(maxWidth).String()
	}
	lines = strings.Split(out, "\n")
	for i, line := range lines {
		lines[i] = trimRightSpaceANSI(line)
	}
	return strings.Join(lines, "\n")
}
