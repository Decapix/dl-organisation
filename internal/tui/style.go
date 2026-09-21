package tui

import "github.com/charmbracelet/lipgloss"

// Styles. Every colour is an AdaptiveColor so the browser reads on a light
// terminal as well as a dark one, and lipgloss drops colour entirely when the
// terminal cannot do it or when NO_COLOR is set.
var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#5a4fcf", Dark: "#b4a7ff"})

	styleCursor = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#ffffff"})

	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6c6c6c", Dark: "#8a8a8a"})

	styleNumber = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#2f6f4f", Dark: "#7fd1a8"})

	styleDead = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#a02020", Dark: "#ff8a8a"})

	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#8a5a00", Dark: "#e8c07d"})

	styleNotePane = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#4a4a4a"}).
			PaddingLeft(1)
)

// visibleWidth is the rendered width of a string, ignoring ANSI escapes. It
// is what the layout tests measure with.
func visibleWidth(s string) int { return lipgloss.Width(s) }
