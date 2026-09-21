package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestTheNotePaneFollowsTheCursor(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	if !strings.Contains(m.note.View(), "first note") {
		t.Fatalf("note pane = %q, want slot 1's note", m.note.View())
	}

	next, _ := m.Update(key('G')) // slot 3
	m = next.(Model)
	if !strings.Contains(m.note.View(), "gamma note") {
		t.Fatalf("note pane = %q, want slot 3's note", m.note.View())
	}
}

func TestTheNotePaneIsEmptyForASlotWithoutOne(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('j')) // slot 2, no note
	m = next.(Model)
	if strings.TrimSpace(m.note.View()) != "" {
		t.Fatalf("note pane = %q, want empty", m.note.View())
	}
}

func TestALongNoteScrolls(t *testing.T) {
	long := strings.Repeat("a line of the note\n", 80)
	m := newTestModel(&fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "long", Path: "/home/u/x", Note: long},
	}})
	if m.note.AtBottom() {
		t.Fatal("an 80-line note fits on a 24-row screen? the viewport is not sized")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = next.(Model)
	if m.note.YOffset == 0 {
		t.Fatal("ctrl-d did not scroll the note")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(Model)
	if m.note.YOffset != 0 {
		t.Fatalf("ctrl-u left the offset at %d, want back at 0", m.note.YOffset)
	}
}

// Moving to another slot resets the scroll, so you never read slot B's note
// starting halfway down because slot A's was long.
func TestMovingResetsTheScroll(t *testing.T) {
	long := strings.Repeat("line\n", 80)
	m := newTestModel(&fakeSession{slots: []slots.Slot{
		{Number: 1, Path: "/home/u/a", Note: long},
		{Number: 2, Path: "/home/u/b", Note: "short"},
	}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	next, _ = next.(Model).Update(key('j'))
	m = next.(Model)
	if m.note.YOffset != 0 {
		t.Fatalf("YOffset = %d after moving, want 0", m.note.YOffset)
	}
}
