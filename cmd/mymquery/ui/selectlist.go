package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	selectMaxVisible = 12
	selectMinWidth   = 30
	selectMaxWidth   = 80
)

type selectItem struct {
	Value string
	// Label is displayed instead of Value if set
	Label string
	// Hint is an optional additional (muted) info
	Hint string
}

func (it selectItem) label() string {
	if it.Label != "" {
		return it.Label
	}
	return it.Value
}

type selectResult int

const (
	selectPending selectResult = iota
	selectChosen
	selectCancelled
)

// selectList is a general modal "pick one value" component with filtering.
// Optionally, it accepts custom (typed) values. It is meant to be embedded
// in a view which forwards keys to it and renders it via placeOverlay.
type selectList struct {
	title    string
	items    []selectItem
	current  string
	input    textinput.Model
	filtered []int
	nav      listNav
	// parseCustom validates and normalizes a typed value;
	// nil means custom values are not allowed
	parseCustom func(string) (string, error)
	err         string
}

func newSelectList(title string, items []selectItem, current string) *selectList {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = accentStyle
	ti.Placeholder = "type to filter"
	ti.Cursor.SetMode(cursor.CursorStatic) // no blinking => no commands needed
	ti.Focus()
	s := &selectList{title: title, items: items, current: current, input: ti}
	s.refilter()
	for i, idx := range s.filtered {
		if items[idx].Value == current {
			s.nav.cursor = i
		}
	}
	s.nav.fix(len(s.filtered), selectMaxVisible)
	return s
}

// withCustom allows typing a value which is not in the list
func (s *selectList) withCustom(placeholder string, parse func(string) (string, error)) *selectList {
	s.parseCustom = parse
	s.input.Placeholder = placeholder
	return s
}

func (s *selectList) refilter() {
	q := strings.ToLower(strings.TrimSpace(s.input.Value()))
	s.filtered = s.filtered[:0]
	var rest []int
	for i, it := range s.items {
		val, lab := strings.ToLower(it.Value), strings.ToLower(it.label())
		switch {
		case q == "":
			s.filtered = append(s.filtered, i)
		case val == q || lab == q:
			s.filtered = append(s.filtered, i) // exact matches go first
		case strings.Contains(val, q) || strings.Contains(lab, q):
			rest = append(rest, i)
		}
	}
	s.filtered = append(s.filtered, rest...)
	s.nav.reset()
}

// Update processes a key and reports whether the selection is finished.
// For selectChosen, the selected value is returned.
func (s *selectList) Update(msg tea.KeyMsg) (selectResult, string) {
	s.err = ""
	switch msg.String() {
	case "esc":
		return selectCancelled, ""
	case "enter":
		if len(s.filtered) > 0 {
			return selectChosen, s.items[s.filtered[s.nav.cursor]].Value
		}
		typed := strings.TrimSpace(s.input.Value())
		if s.parseCustom != nil && typed != "" {
			val, err := s.parseCustom(typed)
			if err != nil {
				s.err = err.Error()
				return selectPending, ""
			}
			return selectChosen, val
		}
		return selectPending, ""
	case "up", "down", "pgup", "pgdown", "home", "end":
		s.nav.handleKey(msg.String(), len(s.filtered), selectMaxVisible)
		return selectPending, ""
	}
	prev := s.input.Value()
	s.input, _ = s.input.Update(msg)
	if s.input.Value() != prev {
		s.refilter()
	}
	return selectPending, ""
}

// View renders the box; maxWidth is the available screen width
func (s *selectList) View(maxWidth int) string {
	labelW, hintW := 0, 0
	for _, it := range s.items {
		labelW = max(labelW, ansi.StringWidth(it.label()))
		hintW = max(hintW, ansi.StringWidth(it.Hint))
	}
	innerW := 2 + labelW + 2 // marker + label + current value mark
	if hintW > 0 {
		innerW += 2 + hintW
	}
	innerW = max(selectMinWidth, min(innerW, maxWidth-6, selectMaxWidth))
	s.input.Width = innerW - 3

	var b strings.Builder
	b.WriteString(titleStyle.Render(s.title) + "\n")
	b.WriteString(s.input.View() + "\n\n")

	from, to := s.nav.visibleRange(len(s.filtered), selectMaxVisible)
	for i := from; i < to; i++ {
		it := s.items[s.filtered[i]]
		marker := "  "
		lab := it.label()
		if it.Value == s.current {
			lab += " ●"
		}
		labStyle := lipgloss.NewStyle()
		if i == s.nav.cursor {
			marker = accentStyle.Bold(true).Render("❯ ")
			labStyle = labStyle.Bold(true).Foreground(colAccent)
		}
		row := marker + labStyle.Render(padRight(lab, labelW+2))
		if it.Hint != "" {
			row += "  " + mutedStyle.Render(it.Hint)
		}
		b.WriteString(ansi.Truncate(row, innerW, "…") + "\n")
	}
	if len(s.filtered) == 0 {
		if s.parseCustom != nil && strings.TrimSpace(s.input.Value()) != "" {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ↵ use %q", strings.TrimSpace(s.input.Value()))) + "\n")
		} else {
			b.WriteString(mutedStyle.Render("  no matching items") + "\n")
		}
	}
	if len(s.filtered) > selectMaxVisible {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("  %d–%d of %d", from+1, to, len(s.filtered))) + "\n")
	}
	if s.err != "" {
		b.WriteString(errorStyle.Render(ansi.Truncate(s.err, innerW, "…")) + "\n")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(0, 1).
		Width(innerW + 2).
		Render(strings.TrimRight(b.String(), "\n"))
}

func (s *selectList) Help() string {
	h := "type to filter • ↑/↓ move • enter select • esc cancel"
	if s.parseCustom != nil {
		h = "type to filter or enter a custom value • ↑/↓ move • enter select • esc cancel"
	}
	return h
}

// placeOverlay draws `fg` centered over `bg` (which is padded to width × height)
func placeOverlay(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	fgLines := strings.Split(fg, "\n")
	fgW := lipgloss.Width(fg)
	x := max(0, (width-fgW)/2)
	y := max(0, (height-len(fgLines))/2)
	for len(bgLines) < y+len(fgLines) {
		bgLines = append(bgLines, "")
	}
	for i, fl := range fgLines {
		bl := bgLines[y+i]
		left := ansi.Truncate(bl, x, "")
		if w := ansi.StringWidth(left); w < x {
			left += strings.Repeat(" ", x-w)
		}
		right := ""
		if ansi.StringWidth(bl) > x+fgW {
			right = ansi.TruncateLeft(bl, x+fgW, "")
		}
		bgLines[y+i] = left + "\x1b[0m" + fl + "\x1b[0m" + right
	}
	return strings.Join(bgLines, "\n")
}

// ---- common pickers

// attrSelectItems creates items for positional attributes with
// descriptions/sizes from the corpus info (if available)
func attrSelectItems(names []string, env *Env, corpusID string) []selectItem {
	info := make(map[string]corpusAttr)
	for _, a := range env.attrCache[corpusID] {
		info[a.Name] = a
	}
	items := make([]selectItem, len(names))
	for i, n := range names {
		var hint []string
		if a, ok := info[n]; ok {
			if a.Size > 0 {
				hint = append(hint, formatInt(a.Size)+" values")
			}
			if a.Description != "" {
				hint = append(hint, a.Description)
			}
		}
		items[i] = selectItem{Value: n, Hint: strings.Join(hint, " · ")}
	}
	return items
}

// parsePositiveInt creates a custom-value parser for integers within [minVal, maxVal]
func parsePositiveInt(minVal, maxVal int) func(string) (string, error) {
	return func(s string) (string, error) {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil || fmt.Sprint(n) != s {
			return "", fmt.Errorf("%q is not an integer", s)
		}
		if n < minVal || n > maxVal {
			return "", fmt.Errorf("value must be between %d and %d", minVal, maxVal)
		}
		return fmt.Sprint(n), nil
	}
}

func intSelectItems(values []int) []selectItem {
	items := make([]selectItem, len(values))
	for i, v := range values {
		items[i] = selectItem{Value: fmt.Sprint(v)}
	}
	return items
}
