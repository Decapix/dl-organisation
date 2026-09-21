package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Row markers.
const (
	markerNote = "*" // the slot carries a note
	markerDead = "x" // the path no longer exists
	markerNone = " "
)

// See lists every slot in number order.
func See(env *Env, cmd cli.Command) error {
	all := env.Store.All()

	// --quiet is for pipes: full paths, nothing else, no header, and no
	// "nothing here" message that a downstream command would choke on.
	if cmd.Quiet {
		for _, sl := range all {
			fmt.Fprintln(env.Out, sl.Path)
		}
		return nil
	}

	if len(all) == 0 {
		fmt.Fprintln(env.Out, "no slots yet — save one with: dl -z")
		return nil
	}

	fmt.Fprintf(env.Out, "%d slot%s\n\n", len(all), plural(len(all)))

	numWidth, nameWidth := columnWidths(all)
	for i, sl := range all {
		fmt.Fprintf(env.Out, "  %s %*d  %-*s  %s\n",
			marker(sl, env.exists(sl)),
			numWidth, sl.Number,
			nameWidth, sl.DisplayName(),
			slots.ShortPath(sl.Path, env.Home),
		)
		if !cmd.Long {
			continue
		}
		// In long form the note sits under its row, indented past the number
		// column: 2 (margin) + 1 (marker) + 1 (space) + numWidth + 2.
		if sl.HasNote() {
			for _, line := range strings.Split(sl.Note, "\n") {
				fmt.Fprintf(env.Out, "%*s%s\n", numWidth+6, "", line)
			}
		}
		if i < len(all)-1 {
			fmt.Fprintln(env.Out)
		}
	}
	return nil
}

// marker is the one-character state flag at the start of a row. A dead path
// outranks a note: the broken thing is the one worth spotting.
func marker(sl slots.Slot, exists bool) string {
	switch {
	case !exists:
		return markerDead
	case sl.HasNote():
		return markerNote
	default:
		return markerNone
	}
}

// columnWidths measures the number and name columns so the table lines up
// whatever the data.
func columnWidths(all []slots.Slot) (numWidth, nameWidth int) {
	for _, sl := range all {
		if w := len(strconv.Itoa(sl.Number)); w > numWidth {
			numWidth = w
		}
		if w := len([]rune(sl.DisplayName())); w > nameWidth {
			nameWidth = w
		}
	}
	return numWidth, nameWidth
}

// plural returns the "s" for a count.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
