package tui

import (
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Compacting renumbers everything, so it asks first: the numbers are what you
// have in your fingers.
func TestCompactAsksFirst(t *testing.T) {
	f := &fakeSession{slots: gappySlots()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('c'))
	m = next.(Model)

	if m.mode != modeConfirm {
		t.Fatalf("mode = %v after c, want modeConfirm", m.mode)
	}
	if len(f.ran) != 0 {
		t.Fatalf("c ran %v before confirmation", f.ran)
	}
	if !strings.Contains(m.confirmPrompt, "renumber") {
		t.Errorf("prompt = %q, want it to say what will happen", m.confirmPrompt)
	}
}

func TestCompactProceedsOnY(t *testing.T) {
	f := &fakeSession{slots: gappySlots()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('c'))
	next, _ = next.(Model).Update(key('y'))

	got := f.lastCommand()
	if got.Action != cli.ActionCompact {
		t.Fatalf("action = %v, want ActionCompact", got.Action)
	}
}

func TestCompactCancels(t *testing.T) {
	f := &fakeSession{slots: gappySlots()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('c'))
	next, _ = next.(Model).Update(key('n'))
	if len(f.ran) != 0 {
		t.Fatalf("cancelling ran %v", f.ran)
	}
}

// It works from an empty row too: compacting is about the whole store, not
// about whatever the cursor happens to be on.
func TestCompactWorksFromAnEmptyRow(t *testing.T) {
	f := &fakeSession{slots: gappySlots()}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('j')) // the 2-6 run
	next, _ = next.(Model).Update(key('c'))
	next, _ = next.(Model).Update(key('y'))

	if f.lastCommand().Action != cli.ActionCompact {
		t.Fatalf("command = %+v, want compact", f.lastCommand())
	}
}

// Nothing to compact: still harmless, and the action says so.
func TestCompactOnACompactStore(t *testing.T) {
	f := &fakeSession{slots: []slots.Slot{{Number: 1, Name: "a", Path: "/home/u/a"}}}
	m := modelOver(f, 100, 24)
	next, _ := m.Update(key('c'))
	next, _ = next.(Model).Update(key('y'))
	if f.lastCommand().Action != cli.ActionCompact {
		t.Fatalf("command = %+v, want compact", f.lastCommand())
	}
}

func TestTheHelpOverlayMentionsCompact(t *testing.T) {
	m := modelOver(&fakeSession{slots: gappySlots()}, 100, 24)
	next, _ := m.Update(key('?'))
	if !strings.Contains(next.(Model).View(), "renumber") {
		t.Errorf("the help overlay does not mention compacting:\n%s", next.(Model).View())
	}
}
