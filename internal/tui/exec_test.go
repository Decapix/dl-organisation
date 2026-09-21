package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

func TestEEditsTheSelectedSlotsNote(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	_, cmd := m.Update(key('e'))
	if cmd == nil {
		t.Fatal("e produced no command, want a tea.Exec")
	}
	// The exec wrapper runs the edit when bubbletea invokes it; calling Run
	// directly is what the terminal handover amounts to.
	ec := &editCommand{session: f, ref: "1"}
	if err := ec.Run(); err != nil {
		t.Fatalf("editCommand.Run: %v", err)
	}
	got := f.lastCommand()
	if got.Action != cli.ActionEdit || got.Ref != "1" {
		t.Fatalf("command = %+v, want edit of slot 1", got)
	}
}

func TestEditCommandReportsFailure(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	ec := &editCommand{session: f, ref: "1"}
	if err := ec.Run(); err == nil {
		t.Fatal("editCommand.Run = nil, want the session's error")
	}
}

// Coming back from the editor must reload, or the pane would still show the
// note as it was before the edit.
func TestReturningFromTheEditorReloads(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, cmd := m.Update(editFinishedMsg{})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("returning from the editor produced no command, want a reload")
	}
	if _, ok := cmd().(reloadMsg); !ok {
		t.Fatal("returning from the editor did not reload")
	}
}

func TestAFailedEditIsReported(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(editFinishedMsg{err: errDead{}})
	m = next.(Model)
	if m.status == "" {
		t.Fatal("a failed edit left no status message")
	}
}

var _ tea.ExecCommand = (*editCommand)(nil)
