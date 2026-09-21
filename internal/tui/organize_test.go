package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// space is the pick-up / put-down key.
func space() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeySpace} }

func fourInARow() []slots.Slot {
	return []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 2, Name: "b", Path: "/home/u/b"},
		{Number: 3, Name: "c", Path: "/home/u/c"},
		{Number: 4, Name: "d", Path: "/home/u/d"},
	}
}

func TestOEntersOrganizeMode(t *testing.T) {
	m := modelOver(&fakeSession{slots: fourInARow()}, 100, 24)
	next, _ := m.Update(key('o'))
	m = next.(Model)

	if m.mode != modeOrganize {
		t.Fatalf("mode = %v after o, want modeOrganize", m.mode)
	}
	if m.picked != 0 {
		t.Fatalf("picked = %d on entering, want nothing held", m.picked)
	}
	if !strings.Contains(m.View(), "space") {
		t.Errorf("the footer does not say what space does:\n%s", m.View())
	}
}

func TestSpacePicksUpAndSpaceDropsAtTheNewPosition(t *testing.T) {
	f := &fakeSession{slots: fourInARow()}
	m := modelOver(f, 100, 24)

	next, _ := m.Update(key('G')) // cursor on slot 4
	next, _ = next.(Model).Update(key('o'))
	next, _ = next.(Model).Update(space())
	m = next.(Model)
	if m.picked != 4 {
		t.Fatalf("picked = %d, want 4", m.picked)
	}
	if len(f.ran) != 0 {
		t.Fatalf("picking up ran %v, want nothing yet", f.ran)
	}

	// Up to slot 2, then drop.
	next, _ = m.Update(key('k'))
	next, _ = next.(Model).Update(key('k'))
	next, _ = next.(Model).Update(space())
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionMove || got.Ref != "4" || got.To != "2" {
		t.Fatalf("command = %+v, want move 4 -> 2", got)
	}
	if m.picked != 0 {
		t.Fatalf("picked = %d after dropping, want nothing held", m.picked)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v after dropping, want modeList", m.mode)
	}
}

// Dropping where you picked up is a no-op, not a command.
func TestDroppingOnTheSameRowDoesNothing(t *testing.T) {
	f := &fakeSession{slots: fourInARow()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('o'))
	next, _ = next.(Model).Update(space())
	next, _ = next.(Model).Update(space())
	m = next.(Model)

	if len(f.ran) != 0 {
		t.Fatalf("dropping in place ran %v, want nothing", f.ran)
	}
	if m.picked != 0 {
		t.Fatal("the slot is still held after dropping in place")
	}
}

// You can drop onto a free number: the slot simply takes it.
func TestDroppingOnAnEmptyRowUsesThatNumber(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 3, Name: "c", Path: "/home/u/c"},
	}}
	m := modelOver(f, 100, 24) // rows: 1, empty 2, 3
	next, _ := m.Update(key('G'))
	next, _ = next.(Model).Update(key('o'))
	next, _ = next.(Model).Update(space())
	next, _ = next.(Model).Update(key('k')) // the empty row 2
	next, _ = next.(Model).Update(space())

	got := f.lastCommand()
	if got.Action != cli.ActionMove || got.Ref != "3" || got.To != "2" {
		t.Fatalf("command = %+v, want move 3 -> 2", got)
	}
}

// A collapsed run stands for many numbers, so there is no single place to
// drop into.
func TestDroppingOnACollapsedRunIsRefused(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 500, Name: "z", Path: "/home/u/z"},
	}}
	m := modelOver(f, 100, 24) // rows: 1, run 2-499, 500
	next, _ := m.Update(key('o'))
	next, _ = next.(Model).Update(space()) // pick up slot 1
	next, _ = next.(Model).Update(key('j'))
	next, _ = next.(Model).Update(space())
	m = next.(Model)

	if len(f.ran) != 0 {
		t.Fatalf("dropping on a run ran %v, want nothing", f.ran)
	}
	if m.picked != 1 {
		t.Fatal("the slot was dropped onto a collapsed run")
	}
	if m.status == "" {
		t.Fatal("refusing to drop said nothing")
	}
}

func TestPickingUpAnEmptyRowIsRefused(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "a", Path: "/home/u/a"},
		{Number: 3, Name: "c", Path: "/home/u/c"},
	}}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('j')) // the empty row 2
	next, _ = next.(Model).Update(key('o'))
	next, _ = next.(Model).Update(space())
	m = next.(Model)

	if m.picked != 0 {
		t.Fatalf("picked = %d, want nothing: an empty row holds no slot", m.picked)
	}
	if m.status == "" {
		t.Fatal("refusing to pick up said nothing")
	}
}

func TestEscapeCancelsTheMove(t *testing.T) {
	f := &fakeSession{slots: fourInARow()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('o'))
	next, _ = next.(Model).Update(space())
	next, _ = next.(Model).Update(key('j'))
	next, _ = next.(Model).Update(special(tea.KeyEsc))
	m = next.(Model)

	if len(f.ran) != 0 {
		t.Fatalf("escape ran %v, want the move cancelled", f.ran)
	}
	if m.picked != 0 || m.mode != modeList {
		t.Fatalf("picked = %d, mode = %v after escape", m.picked, m.mode)
	}
}

// o leaves the mode again when nothing is held, so it is a toggle.
func TestOLeavesOrganizeMode(t *testing.T) {
	m := modelOver(&fakeSession{slots: fourInARow()}, 100, 24)
	next, _ := m.Update(key('o'))
	next, _ = next.(Model).Update(key('o'))
	if next.(Model).mode != modeList {
		t.Fatalf("mode = %v, want modeList", next.(Model).mode)
	}
}

// The held slot is marked, or you lose track of what you are carrying.
func TestTheHeldSlotIsMarked(t *testing.T) {
	m := modelOver(&fakeSession{slots: fourInARow()}, 100, 24)
	next, _ := m.Update(key('o'))
	next, _ = next.(Model).Update(space())
	next, _ = next.(Model).Update(key('j'))
	m = next.(Model)

	if !strings.Contains(m.View(), "↕") {
		t.Errorf("the held slot carries no marker:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "moving slot 1") {
		t.Errorf("the footer does not say what is held:\n%s", m.View())
	}
}

// Movement still works while carrying a slot; that is the whole point.
func TestTheCursorMovesWhileHolding(t *testing.T) {
	m := modelOver(&fakeSession{slots: fourInARow()}, 100, 24)
	next, _ := m.Update(key('o'))
	next, _ = next.(Model).Update(space())
	for _, k := range []rune{'j', 'j', 'G', 'g', 'j'} {
		next, _ = next.(Model).Update(key(k))
	}
	m = next.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	if m.picked != 1 {
		t.Fatal("moving dropped the held slot")
	}
}

func TestTheHelpOverlayMentionsOrganize(t *testing.T) {
	m := modelOver(&fakeSession{slots: fourInARow()}, 100, 24)
	next, _ := m.Update(key('?'))
	if !strings.Contains(next.(Model).View(), "organize") {
		t.Errorf("the help overlay does not mention organize:\n%s", next.(Model).View())
	}
}
