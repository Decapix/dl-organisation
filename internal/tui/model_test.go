package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// key builds a KeyMsg for a single rune.
func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// special builds a KeyMsg for a named key.
func special(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// newTestModel builds a model at a size wide enough for two panes.
func newTestModel(f *fakeSession) Model {
	m := New(f)
	// Fixture paths are not on disk; pin the marker column so tests assert on
	// behaviour rather than on what happens to exist.
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return updated.(Model)
}

func TestCursorMoves(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	if m.cursor != 0 {
		t.Fatalf("cursor starts at %d, want 0", m.cursor)
	}

	next, _ := m.Update(key('j'))
	m = next.(Model)
	if m.cursor != 1 {
		t.Fatalf("after j, cursor = %d, want 1", m.cursor)
	}

	next, _ = m.Update(special(tea.KeyDown))
	m = next.(Model)
	if m.cursor != 2 {
		t.Fatalf("after down, cursor = %d, want 2", m.cursor)
	}

	// The cursor stops at the end rather than wrapping: wrapping makes it
	// easy to overshoot onto the wrong slot and press enter.
	next, _ = m.Update(key('j'))
	m = next.(Model)
	if m.cursor != 2 {
		t.Fatalf("after j at the end, cursor = %d, want 2", m.cursor)
	}

	next, _ = m.Update(key('k'))
	m = next.(Model)
	if m.cursor != 1 {
		t.Fatalf("after k, cursor = %d, want 1", m.cursor)
	}
}

func TestGAndShiftGJumpToTheEnds(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('G'))
	m = next.(Model)
	if m.cursor != 2 {
		t.Fatalf("after G, cursor = %d, want 2", m.cursor)
	}
	next, _ = m.Update(key('g'))
	m = next.(Model)
	if m.cursor != 0 {
		t.Fatalf("after g, cursor = %d, want 0", m.cursor)
	}
}

func TestCursorOnAnEmptyStore(t *testing.T) {
	m := newTestModel(&fakeSession{})
	// Moving around an empty list must not panic or go negative.
	for _, k := range []rune{'j', 'k', 'g', 'G'} {
		next, _ := m.Update(key(k))
		m = next.(Model)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor = %d on an empty store, want 0", m.cursor)
	}
	if m.selected() != nil {
		t.Fatal("selected() on an empty store should be nil")
	}
}

func TestQuitDoesNotJump(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	_, cmd := m.Update(key('q'))
	if cmd == nil {
		t.Fatal("q produced no command, want tea.Quit")
	}
	if len(f.ran) != 0 {
		t.Fatalf("q ran %v, want nothing", f.ran)
	}
}

func TestEnterJumpsAndQuits(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('j')) // select slot 7
	m = next.(Model)

	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter produced no command, want tea.Quit")
	}
	got := f.lastCommand()
	if got.Action != cli.ActionCD {
		t.Fatalf("action = %v, want ActionCD", got.Action)
	}
	if got.Ref != "7" {
		t.Fatalf("ref = %q, want %q", got.Ref, "7")
	}
}

// A failed jump keeps the browser open and shows why, rather than quitting
// into an error the user cannot act on.
func TestEnterOnAFailureStaysOpen(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	m := newTestModel(f)
	next, cmd := m.Update(special(tea.KeyEnter))
	m = next.(Model)
	if cmd != nil {
		t.Fatal("a failed jump should not quit")
	}
	if m.status == "" {
		t.Fatal("a failed jump should show a status message")
	}
}

// errDead is a stand-in for "that directory is gone".
type errDead struct{}

func (errDead) Error() string { return "slot 1 points at /gone, which no longer exists" }

func TestEnterOnAnEmptyStoreDoesNothing(t *testing.T) {
	f := &fakeSession{}
	m := newTestModel(f)
	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter on an empty store should not quit")
	}
	if len(f.ran) != 0 {
		t.Fatalf("enter on an empty store ran %v, want nothing", f.ran)
	}
}
