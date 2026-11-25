package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	modalMarginX  = 2 // empty columns between the screen edge and the box
	modalMarginY  = 1 // empty rows between the screen edge and the box
	modalChromeW  = 4 // border (2) + horizontal padding (2)
	modalChromeH  = 4 // border (2) + title line + empty line
	modalMinInner = 30
)

// modal is a general scrollable window drawn over a view. The box fits its
// content (up to the available area) and the background is dimmed, so it is
// clear that the user is in a nested "screen" which `esc` closes.
//
// The owning view decides when the modal is opened/closed (and handles
// keys it wants to override); the modal itself handles scrolling only.
type modal struct {
	title    string
	subtitle string
	// render produces the content for the given max. width (in cells)
	render func(width int) string
	vp     viewport.Model
	areaW  int
	areaH  int
	// innerW/H are grow-only while the modal is open so that the box
	// does not "jump" when its content changes (e.g. browsing lines)
	innerW int
	innerH int
}

func newModal(title string, render func(width int) string) *modal {
	return &modal{title: title, render: render, vp: viewport.New(0, 0)}
}

// SetSize sets the area (usually the whole result view) the modal is centered in
func (m *modal) SetSize(w, h int) {
	m.areaW, m.areaH = w, h
	m.innerW, m.innerH = 0, 0
	m.refresh(false)
}

// SetContent replaces the title/subtitle and content renderer and scrolls to top
func (m *modal) SetContent(title, subtitle string, render func(width int) string) {
	m.title, m.subtitle, m.render = title, subtitle, render
	m.refresh(true)
}

func (m *modal) maxInner() (int, int) {
	return max(modalMinInner, m.areaW-2*modalMarginX-modalChromeW),
		max(1, m.areaH-2*modalMarginY-modalChromeH)
}

func (m *modal) refresh(toTop bool) {
	maxW, maxH := m.maxInner()
	lines := strings.Split(strings.TrimRight(m.render(maxW), "\n"), "\n")
	contentW := ansi.StringWidth(m.headerText(0))
	for i, ln := range lines {
		if ansi.StringWidth(ln) > maxW {
			lines[i] = ansi.Truncate(ln, maxW, "…")
		}
		contentW = max(contentW, ansi.StringWidth(lines[i]))
	}
	m.innerW = min(maxW, max(m.innerW, modalMinInner, contentW))
	m.innerH = min(maxH, max(m.innerH, len(lines)))
	m.vp.Width, m.vp.Height = m.innerW, m.innerH
	m.vp.SetContent(strings.Join(lines, "\n"))
	if toTop {
		m.vp.GotoTop()
	}
}

func (m *modal) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "home", "g":
		m.vp.GotoTop()
		return nil
	case "end", "G":
		m.vp.GotoBottom()
		return nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return cmd
}

// headerText renders the title line; the scroll position is
// right-aligned if `width` > 0 and the content does not fit
func (m *modal) headerText(width int) string {
	s := titleStyle.Render(m.title)
	if m.subtitle != "" {
		s += mutedStyle.Render("  " + m.subtitle)
	}
	total := m.vp.TotalLineCount()
	if width <= 0 || total <= m.vp.Height {
		return s
	}
	pos := mutedStyle.Render(fmt.Sprintf("%d–%d/%d",
		m.vp.YOffset+1, min(total, m.vp.YOffset+m.vp.Height), total))
	gap := width - ansi.StringWidth(s) - ansi.StringWidth(pos)
	if gap < 2 {
		return ansi.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", gap) + pos
}

// View renders the modal over (dimmed) `bg`
func (m *modal) View(bg string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(0, 1).
		Render(m.headerText(m.innerW) + "\n\n" + m.vp.View())
	return placeOverlay(dimmed(bg), box, m.areaW, m.areaH)
}

func (m *modal) Help() string {
	return "↑/↓/pgup/pgdn scroll • g/G top/bottom"
}

// dimmed removes styling from `s` and renders it in the muted color
func dimmed(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = mutedStyle.Faint(true).Render(ansi.Strip(ln))
	}
	return strings.Join(lines, "\n")
}
