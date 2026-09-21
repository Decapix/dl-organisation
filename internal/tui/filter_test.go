package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// typeString feeds a string to the model one rune at a time.
func typeString(m Model, s string) Model {
	for _, r := range s {
		next, _ := m.Update(key(r))
		m = next.(Model)
	}
	return m
}

func TestMatchesNamePathAndNote(t *testing.T) {
	sl := slots.Slot{Number: 1, Name: "alpha", Path: "/home/u/projects/api", Note: "retry loop"}
	for _, q := range []string{"alpha", "ALPHA", "api", "projects", "retry", "loop"} {
		if !matches(sl, q) {
			t.Errorf("matches(%q) = false, want true", q)
		}
	}
	for _, q := range []string{"zzz", "beta"} {
		if matches(sl, q) {
			t.Errorf("matches(%q) = true, want false", q)
		}
	}
}

// An unnamed slot is matched on its display name, exactly as the ref grammar
// does on the command line.
func TestMatchesTheDisplayNameOfAnUnnamedSlot(t *testing.T) {
	sl := slots.Slot{Number: 1, Path: "/home/u/scraper"}
	if !matches(sl, "scrap") {
		t.Error("an unnamed slot is not matched on its base name")
	}
}

func TestSlashFiltersTheList(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = next.(Model)
	if m.mode != modeFilter {
		t.Fatalf("mode = %v after /, want modeFilter", m.mode)
	}

	m = typeString(m, "gam")
	if len(m.view) != 1 || m.view[0].Number != 12 {
		t.Fatalf("view = %+v, want only slot 12", m.view)
	}
}

// The filter reads notes, which is how you find a project you remember by
// what you were doing rather than by where it lives.
func TestFilterSearchesNotes(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "second")
	if len(m.view) != 1 || m.view[0].Number != 1 {
		t.Fatalf("view = %+v, want only slot 1 (matched in its note)", m.view)
	}
}

func TestEnterKeepsTheFilterAndReturnsToTheList(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gam")

	next, _ = m.Update(special(tea.KeyEnter))
	m = next.(Model)
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
	if len(m.view) != 1 {
		t.Fatalf("view = %d entries, want the filter kept", len(m.view))
	}
}

func TestEscapeClearsTheFilter(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gam")

	next, _ = m.Update(special(tea.KeyEsc))
	m = next.(Model)
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
	if len(m.view) != 3 {
		t.Fatalf("view = %d entries, want all 3 back", len(m.view))
	}
}

// Filtering down to fewer entries than the cursor's index must not leave the
// cursor pointing past the end.
func TestFilteringClampsTheCursor(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('G')) // cursor on the last entry
	m = next.(Model)

	next, _ = m.Update(key('/'))
	m = typeString(next.(Model), "alpha")
	if m.cursor != 0 {
		t.Fatalf("cursor = %d after filtering to one entry, want 0", m.cursor)
	}
	if m.selected() == nil {
		t.Fatal("selected() is nil after filtering")
	}
}

func TestFilterWithNoMatches(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "zzzz")
	if len(m.view) != 0 {
		t.Fatalf("view = %+v, want empty", m.view)
	}
	if m.selected() != nil {
		t.Fatal("selected() should be nil when nothing matches")
	}
	// Enter on nothing must not quit.
	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter with no matches should not quit")
	}
}

func TestBackspaceWidensTheFilter(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gamm")
	for i := 0; i < 4; i++ {
		next, _ = m.Update(special(tea.KeyBackspace))
		m = next.(Model)
	}
	if len(m.view) != 3 {
		t.Fatalf("view = %d entries after clearing the query, want 3", len(m.view))
	}
}
