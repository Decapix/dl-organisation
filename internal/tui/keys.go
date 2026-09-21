package tui

import (
	"fmt"
	"strings"
)

// hint is one entry of the footer legend: a key and the word for what it does.
type hint struct {
	key   string
	label string
}

// hints is the footer legend — every key the list keymap acts on.
//
// It is data rather than a formatted string so that the footer and the ?
// overlay are built from one list. They used to be written out separately,
// which is how `o organize` reached the overlay and not the footer.
//
// The arrows carry no label because they need none, and that is what keeps
// the rendered line under 80 columns; past that the keys at the end get
// truncated away, `? help` among them.
var hints = []hint{
	{"↑↓", ""},
	{"⏎", "cd"},
	{"/", "find"},
	{"e", "edit"},
	{"n", "name"},
	{"o", "organize"},
	{"a", "add"},
	{"d", "del"},
	{"c", "compact"},
	{"?", "help"},
}

// helpSection is one block of the ? overlay.
type helpSection struct {
	title string
	rows  []helpRow
}

// helpRow is one key, or set of equivalent keys, and what it does.
type helpRow struct {
	keys string
	what string
}

// helpSections is the full keymap, grouped by what you are trying to do.
var helpSections = []helpSection{
	{"MOVE", []helpRow{
		{"↑ ↓", "move the cursor"},
		{"j k", "move the cursor"},
		{"g G", "first / last slot"},
		{"ctrl-u ctrl-d", "scroll the note"},
	}},
	{"ACT ON THE SELECTED SLOT", []helpRow{
		{"⏎", "cd into it and quit"},
		{"e", "edit its note in $EDITOR"},
		{"n", "rename it"},
		{"r", "clear its name and note"},
		{"d", "delete it"},
	}},
	{"REARRANGE", []helpRow{
		{"o", "organize: space picks a slot up, move, space drops"},
		{"", "it there and the rest shift to suit"},
		{"c", "compact: renumber every slot to close the gaps"},
	}},
	{"OTHER", []helpRow{
		{"/", "filter by name, path or note"},
		{"a", "save the current directory to a free slot"},
		{"", "(on an empty row, into that slot)"},
		{"?", "this page"},
		{"q ctrl-c", "quit without moving"},
	}},
}

// renderHints is the footer line, with the keys picked out so the eye can
// find them without reading every word.
func renderHints() string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		if h.label == "" {
			parts = append(parts, styleKey.Render(h.key))
			continue
		}
		parts = append(parts, styleKey.Render(h.key)+" "+styleDim.Render(h.label))
	}
	return strings.Join(parts, styleDim.Render("  "))
}

// renderHelpOverlay is the ? page, laid out as a key column and a description
// column so the keys read as a list rather than as prose.
func renderHelpOverlay() string {
	const keyColumn = 15

	var b strings.Builder
	for i, section := range helpSections {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("  " + styleHeader.Render(section.title) + "\n")
		for _, r := range section.rows {
			key := fmt.Sprintf("%-*s", keyColumn, r.keys)
			if r.keys != "" {
				// Pad first, colour after: an escape sequence has no width
				// but every padding calculation would count it.
				key = styleKey.Render(r.keys) +
					strings.Repeat(" ", keyColumn-visibleWidth(r.keys))
			}
			b.WriteString("    " + key + styleDim.Render(r.what) + "\n")
		}
	}
	b.WriteString("\n  " + styleDim.Render("press any key to go back"))
	return b.String()
}
