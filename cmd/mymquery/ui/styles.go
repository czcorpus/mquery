package ui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent   = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#9D8CFF"}
	colMuted    = lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#8A8A8A"}
	colError    = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF6B6B"}
	colKWIC     = lipgloss.AdaptiveColor{Light: "#B03A00", Dark: "#FFB86C"}
	colColl     = lipgloss.AdaptiveColor{Light: "#00796B", Dark: "#5FD7AF"}
	colSelBg    = lipgloss.AdaptiveColor{Light: "#E4DEFF", Dark: "#3A3260"}
	colHeaderBg = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#4B3C99"}
	colHeaderFg = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}

	headerStyle    = lipgloss.NewStyle().Background(colHeaderBg).Foreground(colHeaderFg).Padding(0, 1)
	headerKeyStyle = headerStyle.Padding(0).Bold(true)
	footerStyle    = lipgloss.NewStyle().Foreground(colMuted).Padding(0, 1)
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	mutedStyle     = lipgloss.NewStyle().Foreground(colMuted)
	errorStyle     = lipgloss.NewStyle().Foreground(colError)
	accentStyle    = lipgloss.NewStyle().Foreground(colAccent)
	boldStyle      = lipgloss.NewStyle().Bold(true)

	kwicStyle = lipgloss.NewStyle().Bold(true).Foreground(colKWIC)
	collStyle = lipgloss.NewStyle().Bold(true).Foreground(colColl)
)
