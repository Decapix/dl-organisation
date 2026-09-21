package tui

import (
	"strings"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// matches reports whether a slot satisfies a query.
//
// Case-insensitive substring across the display name, the path and the note.
// Substring rather than fuzzy: on a list of a dozen entries fuzzy mostly
// produces surprises, and a predictable filter is worth more than a clever
// one. Swapping in fuzzy later means replacing this function and nothing else.
//
// Searching the note is what lets you find a project by what you were doing
// rather than by where it lives.
func matches(sl slots.Slot, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	for _, field := range []string{sl.DisplayName(), sl.Path, sl.Note} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// filterSlots returns the slots a query admits, in the order given.
func filterSlots(all []slots.Slot, query string) []slots.Slot {
	if query == "" {
		return all
	}
	out := make([]slots.Slot, 0, len(all))
	for _, sl := range all {
		if matches(sl, query) {
			out = append(out, sl)
		}
	}
	return out
}
