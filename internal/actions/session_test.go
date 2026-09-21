package actions

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func testSession(t *testing.T) *Session {
	t.Helper()
	home := t.TempDir()
	return &Session{
		Dir:    filepath.Join(home, "store"),
		Home:   home,
		Cwd:    filepath.Join(home, "work"),
		Editor: &editor.Fake{},
		Now:    func() time.Time { return time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) },
		Err:    &bytes.Buffer{},
	}
}

func TestSessionRunPersistsBetweenCalls(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "7"}, &out); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.Slots()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Number != 7 {
		t.Fatalf("Slots() = %+v, want slot 7", got)
	}
}

// The point of the type: no lock is held between calls, so a second session
// (a shell running dl while the browser is open) can write.
func TestSessionHoldsNoLockBetweenCalls(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "1"}, &out); err != nil {
		t.Fatal(err)
	}
	// If Run leaked the lock this would block forever; the test would time out.
	other, err := slots.OpenForUpdate(s.Dir)
	if err != nil {
		t.Fatalf("a second writer could not take the lock: %v", err)
	}
	other.Close()
}

// Slots() must see what another process wrote, so the browser picks up a save
// made in another shell on its next reload.
func TestSlotsRereadsFromDisk(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	s.Run(cli.Command{Action: cli.ActionSet, Ref: "1"}, &out)

	other, err := slots.OpenForUpdate(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Put(slots.Slot{Number: 2, Path: "/elsewhere"})
	other.Save()
	other.Close()

	got, _ := s.Slots()
	if len(got) != 2 {
		t.Fatalf("Slots() = %d entries, want 2 (an external write was missed)", len(got))
	}
}

func TestSessionRunReportsActionOutput(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "7"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("Run wrote nothing; the caller needs the action's output")
	}
}

func TestSessionRunPropagatesErrors(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionCD, Ref: "999"}, &out); err == nil {
		t.Fatal("cd into an empty slot = nil error, want an error")
	}
}

// Slots() on a store that was never written is empty, not an error.
func TestSlotsOnAFreshStore(t *testing.T) {
	s := testSession(t)
	got, err := s.Slots()
	if err != nil {
		t.Fatalf("Slots() on a fresh store: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Slots() = %v, want empty", got)
	}
}

func TestSessionAccessors(t *testing.T) {
	s := testSession(t)
	if s.CurrentDir() != s.Cwd {
		t.Errorf("CurrentDir() = %q, want %q", s.CurrentDir(), s.Cwd)
	}
	if s.HomeDir() != s.Home {
		t.Errorf("HomeDir() = %q, want %q", s.HomeDir(), s.Home)
	}
}
