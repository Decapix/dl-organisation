package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The bug this guards: after renaming, the status message took the footer
// over and the key hints were gone for the rest of the session.
func TestTheKeyHintsSurviveAnAction(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), output: "slot 1 is now \"alpha2\"\n"}
	m := newTestModel(f)

	next, _ := m.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEnter))
	m = next.(Model)

	got := m.View()
	if m.status == "" {
		t.Fatal("the action left no status; the test is not exercising the bug")
	}
	if !strings.Contains(got, "n name") {
		t.Errorf("the key hints are gone after an action:\n%s", got)
	}
	if !strings.Contains(got, "alpha2") {
		t.Errorf("the status message is not shown:\n%s", got)
	}
}

// The status sits on its own line, so a message appearing never shifts the
// rest of the layout.
func TestTheFooterHeightIsStable(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), output: "slot 1 is now \"alpha2\"\n"}
	quiet := newTestModel(f)
	before := lineCount(quiet.View())

	next, _ := quiet.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEnter))
	after := lineCount(next.(Model).View())

	if before != after {
		t.Errorf("the view is %d lines without a status and %d with it", before, after)
	}
}

// A message about something you did three keystrokes ago is noise, so the
// next keypress clears it.
func TestTheStatusClearsOnTheNextKeypress(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), output: "slot 1 is now \"alpha2\"\n"}
	m := newTestModel(f)

	next, _ := m.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEnter))
	m = next.(Model)
	if m.status == "" {
		t.Fatal("no status to clear")
	}

	next, _ = m.Update(key('j'))
	m = next.(Model)
	if m.status != "" {
		t.Errorf("status = %q after moving, want it cleared", m.status)
	}
	if !strings.Contains(m.View(), "n name") {
		t.Errorf("the key hints are missing once the status clears:\n%s", m.View())
	}
}

// The key that produced a status must not clear its own message.
func TestAnActionsOwnStatusSurvivesItsKeypress(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), output: "slot 4 -> ~/work\n"}
	m := newTestModel(f)
	next, _ := m.Update(key('a'))
	m = next.(Model)
	if m.status == "" {
		t.Fatal("add cleared its own status message")
	}
}

// In the prompt modes the input replaces the hints, but the status line stays
// where it is so nothing moves.
func TestThePromptModesKeepTheFooterHeight(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	base := lineCount(m.View())

	for _, k := range []rune{'/', 'n', 'd'} {
		next, _ := m.Update(key(k))
		if got := lineCount(next.(Model).View()); got != base {
			t.Errorf("after %q the view is %d lines, want %d", string(k), got, base)
		}
	}
}

// An error is a status too, and must not cost the hints either.
func TestAnErrorKeepsTheKeyHints(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	m := newTestModel(f)
	next, _ := m.Update(special(tea.KeyEnter))
	got := next.(Model).View()
	if !strings.Contains(got, "no longer exists") {
		t.Errorf("the error is not shown:\n%s", got)
	}
	if !strings.Contains(got, "n name") {
		t.Errorf("the key hints are gone after an error:\n%s", got)
	}
}

// Even on a terminal far too short for the layout, the footer survives: it is
// what tells you how to get out.
func TestTheFooterSurvivesAShortTerminal(t *testing.T) {
	f := &fakeSession{slots: manySlots(20)}
	for _, h := range []int{6, 5, 4, 3, 2} {
		got := sized(f, 100, h)
		if n := lineCount(got); n > h {
			t.Errorf("at height %d the view is %d lines:\n%s", h, n, got)
		}
		if h >= 3 && !strings.Contains(got, "⏎ cd") {
			t.Errorf("at height %d the key hints are gone:\n%s", h, got)
		}
	}
}
