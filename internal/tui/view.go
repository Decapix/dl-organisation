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
		return renderHelpOverlay()
	}
	if m.width == 0 {
		return "" // no size yet; bubbletea sends one immediately
	}

	lines := []string{m.renderHeader(), ""}
	lines = append(lines, strings.Split(m.renderBody(), "\n")...)
	lines = append(lines, strings.Split(m.renderFooter(), "\n")...)

	// On a terminal too short for even the chrome, give up body rows rather
	// than the header and the footer: those are what tell you where you are
	// and what you can press.
	if m.height > 0 && len(lines) > m.height {
		const headerLines, footerLines = 2, 2
		body := len(lines) - headerLines - footerLines
		drop := len(lines) - m.height
		if drop > body {
			drop = body
		}
		lines = append(lines[:headerLines], lines[headerLines+drop:]...)
		// Still too tall means there is no room for the chrome either.
		if len(lines) > m.height {
			lines = lines[len(lines)-m.height:]
		}
	}
	return strings.Join(lines, "\n")
}

// renderHeader is the title line with the slot count.
func (m Model) renderHeader() string {
	shown := m.slotCount()
	count := fmt.Sprintf("%d slot%s", shown, plural(shown))
	if shown != len(m.slots) {
		count = fmt.Sprintf("%d of %d slots", shown, len(m.slots))
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
	for j, r := range window {
		i := m.top + j
		cursor := "  "
		if i == m.cursor {
			cursor = styleCursor.Render("▸ ")
		}
		b.WriteString(truncate(cursor+m.renderRow(r, numWidth, nameWidth), width))
		if j < len(window)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderRow draws one line: a slot, or the free number(s) in its place.
func (m Model) renderRow(r row, numWidth, nameWidth int) string {
	if r.slot == nil {
		// A collapsed run has no single number to sit in the number column,
		// so it gets an ellipsis there and states its range instead.
		if r.collapsed() {
			return "  " + fmt.Sprintf("%*s", numWidth, "⋯") + "  " + styleDim.Render(r.label())
		}
		return "  " + styleDim.Render(fmt.Sprintf("%*d", numWidth, r.from)) + "  " +
			styleDim.Render(r.label())
	}

	sl := *r.slot
	mark := " "
	if sl.Note != "" {
		mark = "*"
	}
	if m.picked == sl.Number {
		mark = styleStatus.Render("↕") // the slot currently in hand
	}
	if !m.slotExists(sl) {
		mark = styleDead.Render("x")
	}

	number := styleNumber.Render(fmt.Sprintf("%*d", numWidth, sl.Number))
	name := fmt.Sprintf("%-*s", nameWidth, sl.DisplayName())

	// Elide the path rather than letting truncate cut its tail: the tail is
	// the part that identifies the project.
	used := 2 + 1 + 1 + numWidth + 2 + nameWidth + 2
	short := slots.ShortPath(sl.Path, m.session.HomeDir())
	path := styleDim.Render(elidePath(short, listPaneWidth(m.width)-used))

	return mark + " " + number + "  " + name + "  " + path
}

// columnWidths measures the number and name columns, capping names so that a
// single long name cannot squeeze every path off the screen.
func (m Model) columnWidths() (numWidth, nameWidth int) {
	const maxName = 20
	for _, r := range m.view {
		if w := len(strconv.Itoa(r.to)); w > numWidth {
			numWidth = w
		}
		if r.slot == nil {
			continue // an empty row has no name to measure
		}
		if w := lipgloss.Width(r.slot.DisplayName()); w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > maxName {
		nameWidth = maxName
	}
	return numWidth, nameWidth
}

// renderFooter is always exactly two lines: what just happened, and what you
// can do next.
//
// Giving the status its own line is what keeps the key hints on screen. An
// earlier version let a status message take the footer over, so renaming one
// slot cost you the hints for the rest of the session. The line is rendered
// blank rather than omitted so that a message appearing never shifts the
// layout.
func (m Model) renderFooter() string {
	status := ""
	if m.status != "" {
		status = truncate(styleStatus.Render(m.status), m.width)
	}

	var prompt string
	switch m.mode {
	case modeFilter:
		prompt = m.filter.View()
	case modeRename:
		prompt = m.rename.View()
	case modeConfirm:
		prompt = styleStatus.Render(m.confirmPrompt)
	case modeOrganize:
		if m.picked == 0 {
			prompt = styleDim.Render("organize — space picks a slot up, esc leaves")
		} else {
			prompt = styleStatus.Render(fmt.Sprintf(
				"moving slot %d — space drops it here, esc cancels", m.picked))
		}
	default:
		prompt = renderHints()
	}
	return status + "\n" + truncate(prompt, m.width)
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
