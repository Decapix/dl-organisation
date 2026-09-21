package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// gappySlots leaves holes at 2-6 and 8-11.
func gappySlots() []slots.Slot {
	return []slots.Slot{
		{Number: 1, Name: "alpha", Path: "/home/u/alpha", Note: "first note"},
		{Number: 7, Name: "beta", Path: "/home/u/beta"},
		{Number: 12, Name: "gamma", Path: "/home/u/gamma", Note: "gamma note"},
	}
}

func modelOver(f *fakeSession, w, h int) Model {
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model)
}

// A hole left by --delete stays visible: with numbered slots, seeing which
// numbers are free is half the point.
func TestASingleGapIsShownAsAnEmptyRow(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 3, Name: "c", Path: "/home/u/c"},
	}}
	m := modelOver(f, 100, 24)

	if len(m.view) != 3 {
		t.Fatalf("view = %d rows, want 3 (slot 1, empty 2, slot 3)", len(m.view))
	}
	if !m.view[1].empty() || m.view[1].from != 2 {
		t.Fatalf("row 2 = %+v, want the empty slot 2", m.view[1])
	}
	if !strings.Contains(m.View(), "empty") {
		t.Errorf("the empty row is not rendered:\n%s", m.View())
	}
}

// The list stops at the last occupied slot. Showing free numbers past the end
// would mean showing 988 of them, and the lowest free slot is what `a` picks
// anyway.
func TestTheListStopsAtTheLastOccupiedSlot(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 3, Name: "c", Path: "/home/u/c"},
	}}
	m := modelOver(f, 100, 24)

	last := m.view[len(m.view)-1]
	if last.empty() || last.from != 3 {
		t.Fatalf("last row = %+v, want slot 3", last)
	}
}

// Slots run to 1000, so a long run of empties collapses rather than filling
// the screen with blanks.
func TestALongRunOfEmptiesCollapses(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 500, Name: "far", Path: "/home/u/far"},
	}}
	m := modelOver(f, 100, 24)

	if len(m.view) != 3 {
		t.Fatalf("view = %d rows, want 3 (slot 1, run 2-499, slot 500)", len(m.view))
	}
	run := m.view[1]
	if !run.empty() || !run.collapsed() || run.from != 2 || run.to != 499 {
		t.Fatalf("row 2 = %+v, want the collapsed run 2-499", run)
	}
	if !strings.Contains(m.View(), "2–499") {
		t.Errorf("the collapsed run does not show its range:\n%s", m.View())
	}
}

// Two consecutive empties are still worth showing one by one; three is where
// a list becomes a wall.
func TestShortRunsAreNotCollapsed(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 4, Name: "d", Path: "/home/u/d"},
	}}
	m := modelOver(f, 100, 24)
	// 1, 2, 3, 4 -> the run 2-3 is short enough to list.
	if len(m.view) != 4 {
		t.Fatalf("view = %d rows, want 4", len(m.view))
	}
	for _, i := range []int{1, 2} {
		if m.view[i].collapsed() {
			t.Fatalf("row %d = %+v, want it listed rather than collapsed", i, m.view[i])
		}
	}
}

// Filtering is for finding something; an empty slot is not something.
func TestFilteringHidesTheEmptyRows(t *testing.T) {
	m := modelOver(&fakeSession{slots: gappySlots()}, 100, 24)
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "a")

	for _, r := range m.view {
		if r.empty() {
			t.Fatalf("an empty row survived filtering: %+v", r)
		}
	}
}

// The keys that act on a slot do nothing on an empty row, and say why.
func TestSlotKeysOnAnEmptyRowExplainThemselves(t *testing.T) {
	for _, k := range []rune{'e', 'n', 'd', 'r'} {
		f := &fakeSession{slots: gappySlots()}
		m := modelOver(f, 100, 24)
		next, _ := m.Update(key('j')) // onto the 2-6 run
		next, _ = next.(Model).Update(key(k))
		m = next.(Model)

		if len(f.ran) != 0 {
			t.Errorf("%q ran %v on an empty row", string(k), f.ran)
		}
		if m.status == "" {
			t.Errorf("%q on an empty row said nothing", string(k))
		}
	}
}

func TestEnterOnAnEmptyRowDoesNotQuit(t *testing.T) {
	f := &fakeSession{slots: gappySlots()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('j'))
	m = next.(Model)

	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter on an empty row should not quit")
	}
	if len(f.ran) != 0 {
		t.Fatalf("enter on an empty row ran %v", f.ran)
	}
}

// Showing an empty slot is only half the value; being able to fill it is the
// other half. On an empty row `a` saves into that slot rather than the
// lowest free one.
func TestAddOnAnEmptyRowSavesIntoThatSlot(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 3, Name: "c", Path: "/home/u/c"},
	}}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('j')) // the empty slot 2
	next, _ = next.(Model).Update(key('a'))

	got := f.lastCommand()
	if got.Action != cli.ActionSet || got.Ref != "2" {
		t.Fatalf("command = %+v, want --set into slot 2", got)
	}
}

// On a real slot `a` keeps its old meaning: the lowest free slot.
func TestAddOnARealRowStillUsesTheLowestFreeSlot(t *testing.T) {
	f := &fakeSession{slots: gappySlots()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('a'))
	_ = next

	got := f.lastCommand()
	if got.Action != cli.ActionSet || got.Ref != "" {
		t.Fatalf("command = %+v, want --set with no ref", got)
	}
}

// A collapsed run is informational only; there is no single slot to fill.
func TestAddOnACollapsedRunUsesTheLowestFreeSlot(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 500, Name: "far", Path: "/home/u/far"},
	}}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('j')) // the 2-499 run
	next, _ = next.(Model).Update(key('a'))

	got := f.lastCommand()
	if got.Ref != "" {
		t.Fatalf("ref = %q on a collapsed run, want the lowest free slot", got.Ref)
	}
}

// The note pane says what an empty row is and how to use it.
func TestTheNotePaneExplainsAnEmptyRow(t *testing.T) {
	m := modelOver(&fakeSession{slots: gappySlots()}, 100, 24)
	next, _ := m.Update(key('j'))
	m = next.(Model)
	if !strings.Contains(m.note.View(), "empty") {
		t.Errorf("note pane = %q, want it to explain the empty row", m.note.View())
	}
}

// The header counts slots, not rows: empty rows are not slots.
func TestTheHeaderCountsRealSlots(t *testing.T) {
	m := modelOver(&fakeSession{slots: gappySlots()}, 100, 24)
	if !strings.Contains(m.View(), "3 slots") {
		t.Errorf("header does not report 3 slots:\n%s", m.View())
	}
}

// Empty rows must not break the height and width invariants.
func TestEmptyRowsRespectTheLayout(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 9, Name: "i", Path: "/home/u/i"},
		{Number: 900, Name: "z", Path: "/home/u/z"},
	}}
	for _, size := range []struct{ w, h int }{{100, 24}, {70, 16}, {50, 10}, {40, 6}} {
		got := sized(f, size.w, size.h)
		if n := lineCount(got); n > size.h {
			t.Errorf("at %dx%d the view is %d lines:\n%s", size.w, size.h, n, got)
		}
		if w := widestLine(got); w > size.w {
			t.Errorf("at %dx%d the view is %d columns:\n%s", size.w, size.h, w, got)
		}
	}
}
