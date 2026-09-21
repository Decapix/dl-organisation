package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

func TestAddSavesTheCurrentDirectory(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('a'))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionSet {
		t.Fatalf("action = %v, want ActionSet", got.Action)
	}
	if got.Ref != "" {
		t.Fatalf("ref = %q, want empty so the lowest free slot is used", got.Ref)
	}
}

func TestDeleteAsksFirst(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('d'))
	m = next.(Model)

	if m.mode != modeConfirm {
		t.Fatalf("mode = %v after d, want modeConfirm", m.mode)
	}
	if len(f.ran) != 0 {
		t.Fatalf("d ran %v before confirmation", f.ran)
	}
	if m.confirmPrompt == "" {
		t.Fatal("no confirmation prompt was set")
	}
}

func TestDeleteProceedsOnY(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('d'))
	next, _ = next.(Model).Update(key('y'))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionDelete || got.Ref != "1" {
		t.Fatalf("command = %+v, want delete of slot 1", got)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v after confirming, want modeList", m.mode)
	}
}

func TestDeleteCancelsOnAnythingElse(t *testing.T) {
	for _, k := range []rune{'n', 'x', 'q'} {
		f := &fakeSession{slots: threeSlots()}
		m := newTestModel(f)
		next, _ := m.Update(key('d'))
		next, _ = next.(Model).Update(key(k))
		m = next.(Model)

		if len(f.ran) != 0 {
			t.Fatalf("%q ran %v, want the delete cancelled", string(k), f.ran)
		}
		if m.mode != modeList {
			t.Fatalf("mode = %v after cancelling with %q, want modeList", m.mode, string(k))
		}
	}
}

func TestResetAsksFirstToo(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('r'))
	next, _ = next.(Model).Update(key('y'))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionReset || got.Ref != "1" {
		t.Fatalf("command = %+v, want reset of slot 1", got)
	}
}

func TestRenameTakesTheNewNameInline(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('n'))
	m = next.(Model)
	if m.mode != modeRename {
		t.Fatalf("mode = %v after n, want modeRename", m.mode)
	}
	// The input starts from the current name so a small correction is easy.
	if m.rename.Value() != "alpha" {
		t.Fatalf("rename input = %q, want the current name", m.rename.Value())
	}

	m = typeString(m, "2")
	next, _ = m.Update(special(tea.KeyEnter))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionRename || got.Ref != "1" || got.Name != "alpha2" {
		t.Fatalf("command = %+v, want rename of slot 1 to alpha2", got)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
}

func TestRenameCancelsOnEscape(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEsc))
	m = next.(Model)

	if len(f.ran) != 0 {
		t.Fatalf("escape ran %v, want nothing", f.ran)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
}

// A rejected name (duplicate, numeric, spaced) must surface the reason rather
// than silently doing nothing.
func TestARejectedRenameShowsTheError(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	m := newTestModel(f)
	next, _ := m.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEnter))
	m = next.(Model)

	if m.status == "" {
		t.Fatal("a rejected rename left no status message")
	}
}

// Every mutation reloads, so the list reflects what just happened.
func TestAMutationTriggersAReload(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	_, cmd := m.Update(key('a'))
	if cmd == nil {
		t.Fatal("add produced no command, want a reload")
	}
	msg := cmd()
	if _, ok := msg.(reloadMsg); !ok {
		t.Fatalf("command produced %T, want reloadMsg", msg)
	}
}

// Keys that act on a slot do nothing when there is no slot under the cursor.
func TestSlotKeysAreNoOpsOnAnEmptyStore(t *testing.T) {
	for _, k := range []rune{'d', 'r', 'n', 'e'} {
		f := &fakeSession{}
		m := newTestModel(f)
		next, _ := m.Update(key(k))
		m = next.(Model)
		if m.mode != modeList {
			t.Fatalf("%q changed mode to %v on an empty store", string(k), m.mode)
		}
		if len(f.ran) != 0 {
			t.Fatalf("%q ran %v on an empty store", string(k), f.ran)
		}
	}
}
