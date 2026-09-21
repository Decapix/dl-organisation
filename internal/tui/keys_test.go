package tui

import (
	"strings"
	"testing"
)

// boundKeys is every key the list keymap acts on, with the word each one
// should be advertised by. This table is the contract: adding a key to the
// keymap without adding it here, or here without the hints, fails the build's
// tests rather than shipping a feature nobody can find.
var boundKeys = []struct {
	key  string // as it appears in the hints line
	word string // the label beside it
}{
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

// The footer line is the only thing most people will ever read, so every key
// has to be on it. `o` shipped missing from it once.
func TestEveryBoundKeyIsInTheHints(t *testing.T) {
	for _, b := range boundKeys {
		if !strings.Contains(keyHints, b.key+" "+b.word) {
			t.Errorf("the hints line is missing %q %q:\n%s", b.key, b.word, keyHints)
		}
	}
}

func TestEveryBoundKeyIsInTheHelpOverlay(t *testing.T) {
	for _, b := range boundKeys {
		if !strings.Contains(helpOverlay, b.key) {
			t.Errorf("the help overlay is missing %q", b.key)
		}
	}
}

// The hints must fit a standard terminal, or the last keys — which include
// the one that opens the full list — get truncated away.
func TestTheHintsFitEightyColumns(t *testing.T) {
	if w := visibleWidth(keyHints); w > 80 {
		t.Fatalf("the hints line is %d columns:\n%s", w, keyHints)
	}
}
