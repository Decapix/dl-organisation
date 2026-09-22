package actions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// lockIsFree reports whether the store's write lock could be taken right now,
// without waiting. It is how a test observes what another shell would see
// while an editor is open.
func lockIsFree(t *testing.T, dir string) bool {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, slots.LockFileName), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
		return true
	}
	if err != unix.EWOULDBLOCK {
		t.Fatalf("probe flock: %v", err)
	}
	return false
}

// seedSlot writes one slot to a session's store the way another shell would.
func seedSlot(t *testing.T, s *Session, sl slots.Slot) {
	t.Helper()
	st, err := slots.OpenForUpdate(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	st.Put(sl)
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()
}

func slotNote(t *testing.T, s *Session, n int) (string, bool) {
	t.Helper()
	st, err := slots.Open(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sl, ok := st.Get(n)
	return sl.Note, ok
}

// The regression test for the hang: `dl 6 -e` in one shell blocked for as
// long as an editor opened from another dl stayed open, with no message,
// because the exclusive lock was taken before the editor ran and released
// only after it exited. The editor is interactive and may stay open for an
// hour; the lock must only cover the write that follows it.
func TestEditRunsTheEditorWithoutTheLock(t *testing.T) {
	s := testSession(t)
	seedSlot(t, s, slots.Slot{Number: 6, Path: "/tmp/x", Note: "before"})

	var freeWhileOpen bool
	s.Editor = &editor.Fake{Hook: func(initial string) (string, error) {
		freeWhileOpen = lockIsFree(t, s.Dir)
		return "after", nil
	}}

	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionEdit, Ref: "6"}, &out); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if !freeWhileOpen {
		t.Fatal("the store lock was held while the editor was open; every other dl would block")
	}
	if note, _ := slotNote(t, s, 6); note != "after" {
		t.Fatalf("Note = %q after the edit, want %q", note, "after")
	}
	if !strings.Contains(out.String(), "slot 6 note saved") {
		t.Fatalf("output = %q", out.String())
	}
}

// `dl -z 6 -e` runs the same editor and had the same hang.
func TestSetWithEditRunsTheEditorWithoutTheLock(t *testing.T) {
	s := testSession(t)
	seedSlot(t, s, slots.Slot{Number: 6, Path: "/tmp/x", Note: "before"})

	var freeWhileOpen bool
	var seen string
	s.Editor = &editor.Fake{Hook: func(initial string) (string, error) {
		seen = initial
		freeWhileOpen = lockIsFree(t, s.Dir)
		return "after", nil
	}}

	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "6", Edit: true}, &out); err != nil {
		t.Fatalf("set -e: %v", err)
	}
	if !freeWhileOpen {
		t.Fatal("the store lock was held while the editor was open; every other dl would block")
	}
	if seen != "before" {
		t.Fatalf("editor was handed %q, want the existing note", seen)
	}
	st, _ := slots.Open(s.Dir)
	got, _ := st.Get(6)
	if got.Note != "after" || got.Path != s.Cwd {
		t.Fatalf("slot 6 = %+v, want note %q and path %q", got, "after", s.Cwd)
	}
}

// Without the lock, the world can change while the editor is open. The slot
// being deleted underneath is the one case where saving would resurrect it;
// refuse, and show the text so it is not lost.
func TestEditRefusesToSaveIntoASlotDeletedWhileEditing(t *testing.T) {
	s := testSession(t)
	seedSlot(t, s, slots.Slot{Number: 6, Path: "/tmp/x", Note: "before"})

	s.Editor = &editor.Fake{Hook: func(initial string) (string, error) {
		// Another shell runs `dl -d 6` meanwhile. If the lock were held this
		// would deadlock and the test would time out.
		other := testSession(t)
		other.Dir = s.Dir
		if err := other.Run(cli.Command{Action: cli.ActionDelete, Ref: "6"}, &bytes.Buffer{}); err != nil {
			t.Fatalf("delete from another shell: %v", err)
		}
		return "the text I typed", nil
	}}

	var out bytes.Buffer
	err := s.Run(cli.Command{Action: cli.ActionEdit, Ref: "6"}, &out)
	if err == nil {
		t.Fatal("edit = nil error on a slot deleted meanwhile, want an error")
	}
	if !strings.Contains(err.Error(), "deleted") {
		t.Fatalf("error = %q, want it to say the slot was deleted", err)
	}
	if _, ok := slotNote(t, s, 6); ok {
		t.Fatal("slot 6 was recreated by the edit, want it to stay deleted")
	}
	if errb := s.Err.(*bytes.Buffer).String(); !strings.Contains(errb, "the text I typed") {
		t.Fatalf("stderr = %q, want the unsaved text so the user can recover it", errb)
	}
}

// A note changed elsewhere while the editor was open is replaced — the user
// just typed this version and expects it saved — but they are told.
func TestEditWarnsWhenTheNoteChangedWhileEditing(t *testing.T) {
	s := testSession(t)
	seedSlot(t, s, slots.Slot{Number: 6, Path: "/tmp/x", Note: "before"})

	s.Editor = &editor.Fake{Hook: func(initial string) (string, error) {
		other := testSession(t)
		other.Dir = s.Dir
		cmd := cli.Command{Action: cli.ActionSetNote, Ref: "6", Note: "changed elsewhere"}
		if err := other.Run(cmd, &bytes.Buffer{}); err != nil {
			t.Fatalf("set note from another shell: %v", err)
		}
		return "mine", nil
	}}

	if err := s.Run(cli.Command{Action: cli.ActionEdit, Ref: "6"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if note, _ := slotNote(t, s, 6); note != "mine" {
		t.Fatalf("Note = %q, want the editor's version %q", note, "mine")
	}
	if errb := s.Err.(*bytes.Buffer).String(); !strings.Contains(errb, "changed") {
		t.Fatalf("stderr = %q, want a warning that the note changed meanwhile", errb)
	}
}

// A shell that has to wait for the lock says so on stderr. Before this, the
// wait was a frozen prompt with nothing on it.
func TestSessionSaysSoWhenWaitingForTheLock(t *testing.T) {
	s := testSession(t)
	holder, err := slots.OpenForUpdate(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		holder.Close()
	}()

	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "1"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if errb := s.Err.(*bytes.Buffer).String(); !strings.Contains(errb, "waiting") {
		t.Fatalf("stderr = %q, want a line saying dl is waiting for the lock", errb)
	}
}

// The write after the editor goes through the same notice.
func TestEditSaysSoWhenWaitingForTheLockAfterTheEditor(t *testing.T) {
	s := testSession(t)
	seedSlot(t, s, slots.Slot{Number: 6, Path: "/tmp/x", Note: "before"})
	s.Editor = &editor.Fake{Hook: func(string) (string, error) {
		holder, err := slots.OpenForUpdate(s.Dir)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			time.Sleep(50 * time.Millisecond)
			holder.Close()
		}()
		return "after", nil
	}}

	if err := s.Run(cli.Command{Action: cli.ActionEdit, Ref: "6"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if errb := s.Err.(*bytes.Buffer).String(); !strings.Contains(errb, "waiting") {
		t.Fatalf("stderr = %q, want a line saying dl is waiting for the lock", errb)
	}
}
