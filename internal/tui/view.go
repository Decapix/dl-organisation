package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// View renders the browser: a header, the list, the note pane beside or below
// it depending on the width, and a footer that is either the key hints, the
// filter input, the rename input, a confirmation or the last status message.
func (m Model) View() string {
	if m.mode == modeHelp {
		return helpOverlay
	}
	if m.width == 0 {
		return "" // no size yet; bubbletea sends one immediately
	}

	return strings.Join([]string{
		m.renderHeader(),
		"",
		m.renderBody(),
		m.renderFooter(),
	}, "\n")
}

// renderHeader is the title line with the slot count.
func (m Model) renderHeader() string {
	count := fmt.Sprintf("%d slot%s", len(m.view), plural(len(m.view)))
	if len(m.view) != len(m.slots) {
		count = fmt.Sprintf("%d of %d slots", len(m.view), len(m.slots))
	}
	left := styleHeader.Render("dl")
	right := styleDim.Render(count)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left, m.width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderBody is the list, with the note pane beside it when there is room.
func (m Model) renderBody() string {
	if len(m.view) == 0 {
		return m.renderEmpty()
	}
	list := m.renderList()
	if m.width < wideMin {
		// Too narrow for two panes: put the note underneath, or drop it.
		if m.width < narrowMin {
			return list
		}
		return list + "\n" + styleDim.Render(truncate(m.note.View(), m.width))
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(listPaneWidth(m.width)).Render(list),
		styleNotePane.Width(notePaneWidth(m.width)).Render(m.note.View()),
	)
}

// renderEmpty is what a fresh install sees.
func (m Model) renderEmpty() string {
	return styleDim.Render("  no slots yet — press a to save the current directory")
}

// renderList draws one row per visible slot, fitted to the pane width.
func (m Model) renderList() string {
	width := listPaneWidth(m.width)
	numWidth, nameWidth := m.columnWidths()

	// Only the window is drawn. `top` is kept in range by scrollToCursor.
	rows := listRows(m.width, m.height)
	end := m.top + rows
	if end > len(m.view) {
		end = len(m.view)
	}
	window := m.view[m.top:end]

	var b strings.Builder
	for j, sl := range window {
		i := m.top + j
		cursor := "  "
		if i == m.cursor {
			cursor = styleCursor.Render("▸ ")
		}
		mark := " "
		if sl.Note != "" {
			mark = "*"
		}
		if !m.slotExists(sl) {
			mark = styleDead.Render("x")
		}

		number := styleNumber.Render(fmt.Sprintf("%*d", numWidth, sl.Number))
		name := fmt.Sprintf("%-*s", nameWidth, sl.DisplayName())

		// Elide the path rather than letting truncate cut its tail: the tail
		// is the part that identifies the project.
		used := 2 + 1 + 1 + numWidth + 2 + nameWidth + 2
		short := slots.ShortPath(sl.Path, m.session.HomeDir())
		path := styleDim.Render(elidePath(short, width-used))

		row := cursor + mark + " " + number + "  " + name + "  " + path
		b.WriteString(truncate(row, width))
		if j < len(window)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// columnWidths measures the number and name columns, capping names so that a
// single long name cannot squeeze every path off the screen.
func (m Model) columnWidths() (numWidth, nameWidth int) {
	const maxName = 20
	for _, sl := range m.view {
		if w := len(strconv.Itoa(sl.Number)); w > numWidth {
			numWidth = w
		}
		if w := lipgloss.Width(sl.DisplayName()); w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > maxName {
		nameWidth = maxName
	}
	return numWidth, nameWidth
}

// renderFooter shows whichever prompt the current mode needs, falling back to
// the last status message and then to the key hints.
func (m Model) renderFooter() string {
	switch m.mode {
	case modeFilter:
		return truncate(m.filter.View(), m.width)
	case modeRename:
		return truncate(m.rename.View(), m.width)
	case modeConfirm:
		return truncate(styleStatus.Render(m.confirmPrompt), m.width)
	}
	if m.status != "" {
		return truncate(styleStatus.Render(m.status), m.width)
	}
	return truncate(styleDim.Render(keyHints), m.width)
}

// truncate cuts a rendered string to a visible width. It measures with
// lipgloss so ANSI escapes are not counted against the budget.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// plural returns the "s" for a count.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
