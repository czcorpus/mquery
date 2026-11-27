package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// formField is a single field of an optionsForm: either a choice
// from fixed options (changed by ←/→) or a typed value
type formField struct {
	label string
	// hint is an optional muted text shown for the focused field
	hint string

	// choice field
	options []selectItem
	choice  int

	// input field (used when options is nil); parse validates
	// and normalizes the value
	input textinput.Model
	parse func(string) (string, error)
	// visible tells whether the field is shown (nil = always)
	visible func() bool
}

func newChoiceField(label, hint string, options []selectItem, current string) *formField {
	f := &formField{label: label, hint: hint, options: options}
	for i, o := range options {
		if o.Value == current {
			f.choice = i
		}
	}
	return f
}

func newInputField(label, hint, value string, parse func(string) (string, error)) *formField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 20
	ti.Cursor.SetMode(cursor.CursorStatic) // no blinking => no commands needed
	ti.SetValue(value)
	ti.CursorEnd()
	return &formField{label: label, hint: hint, input: ti, parse: parse}
}

// withCharLimit sets the max. length of an input field (0 = no limit)
func (f *formField) withCharLimit(n int) *formField {
	f.input.CharLimit = n
	return f
}

// withPlaceholder sets a text shown in an empty input field
func (f *formField) withPlaceholder(s string) *formField {
	f.input.Placeholder = s
	return f
}

// shownIf makes the field visible only if cond returns true (it is
// evaluated on each change of the form, e.g. on a value of another field)
func (f *formField) shownIf(cond func() bool) *formField {
	f.visible = cond
	return f
}

func (f *formField) isVisible() bool {
	return f.visible == nil || f.visible()
}

// masked hides the typed value (e.g. a password)
func (f *formField) masked() *formField {
	f.input.EchoMode = textinput.EchoPassword
	f.input.EchoCharacter = '•'
	return f
}

func (f *formField) isChoice() bool {
	return f.options != nil
}

// value returns the current (for input fields: validated) value
func (f *formField) value() (string, error) {
	if f.isChoice() {
		return f.options[f.choice].Value, nil
	}
	return f.parse(strings.TrimSpace(f.input.Value()))
}

// optionsForm is a modal form for editing several options at once.
// It is meant to be embedded in a view which forwards keys to it and
// renders it via placeOverlay. Values are read via values() after
// Update returns selectChosen.
type optionsForm struct {
	title  string
	fields []*formField
	focus  int
	err    string
	// minWidth is the min. inner width (0 = as needed by the fields)
	minWidth int
}

func newOptionsForm(title string, fields ...*formField) *optionsForm {
	f := &optionsForm{title: title, fields: fields}
	f.setFocus(0)
	return f
}

func (f *optionsForm) setFocus(i int) {
	n := len(f.fields)
	f.focus = ((i % n) + n) % n
	for j, fld := range f.fields {
		if fld.isChoice() {
			continue
		}
		if j == f.focus {
			fld.input.Focus()
		} else {
			fld.input.Blur()
		}
	}
}

// moveFocus moves the focus to the next/previous (d = 1/-1) visible field
func (f *optionsForm) moveFocus(d int) {
	i := f.focus
	for range f.fields {
		i += d
		n := len(f.fields)
		if f.fields[((i%n)+n)%n].isVisible() {
			break
		}
	}
	f.setFocus(i)
}

// values returns validated values of all the fields (in the field order);
// on error, the invalid field gets focus
func (f *optionsForm) values() ([]string, error) {
	ans := make([]string, len(f.fields))
	for i, fld := range f.fields {
		v, err := fld.value()
		if err != nil {
			f.setFocus(i)
			return nil, err
		}
		ans[i] = v
	}
	return ans, nil
}

// Update processes a key. With selectChosen, all the values are valid
// (see values()).
func (f *optionsForm) Update(msg tea.KeyMsg) selectResult {
	f.err = ""
	fld := f.fields[f.focus]
	switch msg.String() {
	case "esc":
		return selectCancelled
	case "enter":
		if _, err := f.values(); err != nil {
			f.err = err.Error()
			return selectPending
		}
		return selectChosen
	case "up", "shift+tab":
		f.moveFocus(-1)
		return selectPending
	case "down", "tab":
		f.moveFocus(1)
		return selectPending
	case "left", "right", " ":
		if fld.isChoice() {
			d := 1
			if msg.String() == "left" {
				d = -1
			}
			n := len(fld.options)
			fld.choice = ((fld.choice+d)%n + n) % n
			return selectPending
		}
	}
	if !fld.isChoice() {
		fld.input, _ = fld.input.Update(msg)
	}
	return selectPending
}

// View renders the form box; maxWidth is the available screen width
func (f *optionsForm) View(maxWidth int) string {
	labelW, valueW := 0, 12
	for _, fld := range f.fields {
		labelW = max(labelW, ansi.StringWidth(fld.label))
		for _, o := range fld.options {
			valueW = max(valueW, ansi.StringWidth(o.label())+4)
		}
	}
	innerW := max(selectMinWidth, min(labelW+3+valueW, maxWidth-6, selectMaxWidth))
	if f.minWidth > innerW {
		innerW = min(f.minWidth, maxWidth-6)
		valueW = max(valueW, innerW-labelW-3)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(f.title) + "\n\n")
	for i, fld := range f.fields {
		if !fld.isVisible() {
			continue
		}
		focused := i == f.focus
		marker, labStyle := "  ", mutedStyle
		if focused {
			marker = accentStyle.Bold(true).Render("❯ ")
			labStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
		}
		var val string
		if fld.isChoice() {
			val = fld.options[fld.choice].label()
			if focused {
				val = accentStyle.Render("‹ ") + val + accentStyle.Render(" ›")
			} else {
				val = "  " + val
			}
		} else {
			// (+1 cell for the cursor at the end)
			fld.input.Width = max(4, valueW-4)
			val = "  " + fld.input.View()
		}
		row := marker + labStyle.Render(padRight(fld.label, labelW)) + " " + val
		b.WriteString(ansi.Truncate(row, innerW, "…") + "\n")
	}
	if hint := f.fields[f.focus].hint; hint != "" {
		b.WriteString("\n" + mutedStyle.Width(innerW).Render(hint) + "\n")
	}
	if f.err != "" {
		b.WriteString("\n" + errorStyle.Width(innerW).Render(f.err) + "\n")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(0, 1).
		Width(innerW + 2).
		Render(strings.TrimRight(b.String(), "\n"))
}

func (f *optionsForm) Help() string {
	if f.fields[f.focus].isChoice() {
		return "↑/↓ field • ←/→ change • enter apply • esc cancel"
	}
	return "↑/↓ field • type a value • enter apply • esc cancel"
}
