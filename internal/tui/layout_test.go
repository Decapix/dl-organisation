package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// manySlots builds a list far longer than any terminal.
func manySlots(n int) []slots.Slot {
	out := make([]slots.Slot, n)
	for i := range out {
		out[i] = slots.Slot{
			Number: i + 1,
			Name:   fmt.Sprintf("slot%02d", i+1),
			Path:   fmt.Sprintf("/home/u/projects/number-%02d", i+1),
			Note:   strings.Repeat(fmt.Sprintf("note line for slot %d\n", i+1), 30),
		}
	}
	return out
}

// lineCount is how many rows a rendered view occupies.
func lineCount(s string) int { return len(strings.Split(s, "\n")) }

// The whole point of a full-screen program: it must fit the screen. A view
// taller than the terminal scrolls the footer away and the layout is lost.
func TestTheViewFitsTheTerminalHeight(t *testing.T) {
	f := &fakeSession{slots: manySlots(50)}
	for _, size := range []struct{ w, h int }{
		{100, 24}, {100, 10}, {100, 6},
		{70, 24}, {70, 16}, {70, 8},
		{50, 24}, {50, 14}, {50, 6},
	} {
		got := sized(f, size.w, size.h)
		if n := lineCount(got); n > size.h {
			t.Errorf("at %dx%d the view is %d lines:\n%s", size.w, size.h, n, got)
		}
	}
}

// An empty store and a one-slot store must fit too; the sizing arithmetic is
// where off-by-ones live.
func TestSmallStoresFitTheTerminalHeight(t *testing.T) {
	for _, n := range []int{0, 1, 2} {
		f := &fakeSession{slots: manySlots(n)}
		for _, h := range []int{24, 10, 6, 4} {
			got := sized(f, 100, h)
			if lines := lineCount(got); lines > h {
				t.Errorf("%d slots at height %d render %d lines:\n%s", n, h, lines, got)
			}
		}
	}
}

// The cursor must stay on screen, or moving down past the fold would leave
// you steering something you cannot see.
func TestTheListScrollsToKeepTheCursorVisible(t *testing.T) {
	f := &fakeSession{slots: manySlots(50)}
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(Model)

	// Jump to the end: the last slot must be rendered.
	next, _ := m.Update(key('G'))
	m = next.(Model)
	if !strings.Contains(m.View(), "slot50") {
		t.Errorf("after G the last slot is off screen:\n%s", m.View())
	}
	if lineCount(m.View()) > 20 {
		t.Errorf("after G the view is %d lines, want at most 20", lineCount(m.View()))
	}

	// And back to the top.
	next, _ = m.Update(key('g'))
	m = next.(Model)
	if !strings.Contains(m.View(), "slot01") {
		t.Errorf("after g the first slot is off screen:\n%s", m.View())
	}
}

// Stepping down one row at a time must scroll too, not just the g/G jumps.
func TestSteppingDownScrollsTheWindow(t *testing.T) {
	f := &fakeSession{slots: manySlots(50)}
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(Model)

	for i := 0; i < 30; i++ {
		next, _ := m.Update(key('j'))
		m = next.(Model)
	}
	if !strings.Contains(m.View(), "slot31") {
		t.Errorf("the selected slot is off screen after 30 steps:\n%s", m.View())
	}
}

// Filtering to a short list must rewind the window, or the list would render
// blank because the window is still scrolled past the end.
func TestFilteringRewindsTheWindow(t *testing.T) {
	f := &fakeSession{slots: manySlots(50)}
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(Model)

	next, _ := m.Update(key('G'))
	next, _ = next.(Model).Update(key('/'))
	m = typeString(next.(Model), "slot07")

	if !strings.Contains(m.View(), "slot07") {
		t.Errorf("the only match is not rendered:\n%s", m.View())
	}
}
