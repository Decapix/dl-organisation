package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// sized renders the model at a given terminal size.
func sized(f *fakeSession, w, h int) string {
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model).View()
}

func TestViewShowsEverySlot(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	for _, want := range []string{"alpha", "beta", "gamma", "1", "7", "12"} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

func TestViewMarksNotesAndDeadPaths(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	if !strings.Contains(got, "*") {
		t.Errorf("view has no note marker:\n%s", got)
	}

	// With the seam saying nothing exists, every row carries the dead marker
	// and the dead marker wins over the note marker.
	m := New(&fakeSession{slots: threeSlots()})
	m.exists = func(string) bool { return false }
	m.slots = threeSlots()
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	dead := updated.(Model).View()
	if !strings.Contains(dead, "x") {
		t.Errorf("view has no dead-path marker:\n%s", dead)
	}
}

func TestViewShortensThePath(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	if strings.Contains(got, "/home/u/alpha") {
		t.Errorf("view shows an unshortened path:\n%s", got)
	}
	if !strings.Contains(got, "~/alpha") {
		t.Errorf("view does not shorten the home directory:\n%s", got)
	}
}

func TestViewShowsTheKeyHints(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	if !strings.Contains(got, "cd") || !strings.Contains(got, "help") {
		t.Errorf("view has no key hints:\n%s", got)
	}
}

func TestViewOnAnEmptyStoreSaysWhatToDo(t *testing.T) {
	got := sized(&fakeSession{}, 100, 24)
	if !strings.Contains(got, "no slots yet") {
		t.Errorf("empty view does not say what to do:\n%s", got)
	}
	if !strings.Contains(got, "press a") {
		t.Errorf("empty view does not mention the add key:\n%s", got)
	}
}

func TestNarrowLayoutDropsTheSidePane(t *testing.T) {
	wide := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	narrow := sized(&fakeSession{slots: threeSlots()}, 50, 24)

	if widestLine(narrow) > 50 {
		t.Errorf("narrow view is %d columns wide, want at most 50:\n%s", widestLine(narrow), narrow)
	}
	if widestLine(wide) > 100 {
		t.Errorf("wide view is %d columns wide, want at most 100", widestLine(wide))
	}
}

// Nothing may exceed the terminal width at any size, or the display wraps and
// the layout falls apart.
func TestNoLineExceedsTheTerminalWidth(t *testing.T) {
	long := slots.Slot{
		Number: 999,
		Name:   "a-very-long-slot-name-indeed",
		Path:   "/home/u/one/two/three/four/five/six/seven/eight/nine/ten/eleven",
		Note:   strings.Repeat("a long note line that will need wrapping ", 5),
	}
	for _, w := range []int{40, 50, 60, 79, 80, 100, 200} {
		got := sized(&fakeSession{slots: []slots.Slot{long}}, w, 24)
		if widest := widestLine(got); widest > w {
			t.Errorf("at width %d the view is %d columns wide:\n%s", w, widest, got)
		}
	}
}

func TestHelpOverlayReplacesTheView(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('?'))
	got := next.(Model).View()
	if !strings.Contains(got, "ctrl-u") {
		t.Errorf("help overlay is missing keys:\n%s", got)
	}
}

func TestTheFilterQueryIsVisible(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gam")
	if !strings.Contains(m.View(), "gam") {
		t.Errorf("the filter query is not shown:\n%s", m.View())
	}
}

func TestTheConfirmPromptIsVisible(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('d'))
	if !strings.Contains(next.(Model).View(), "[y/N]") {
		t.Errorf("the confirmation prompt is not shown:\n%s", next.(Model).View())
	}
}

// widestLine is the visible width of the longest line, ANSI escapes excluded.
func widestLine(s string) int {
	widest := 0
	for _, line := range strings.Split(s, "\n") {
		if w := visibleWidth(line); w > widest {
			widest = w
		}
	}
	return widest
}
