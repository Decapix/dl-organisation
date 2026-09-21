package actions

import (
	"errors"
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestEditOpensTheEditorAndStoresTheResult(t *testing.T) {
	env, _, _ := testEnv(t)
	fake := &editor.Fake{Result: "rewritten"}
	env.Editor = fake
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "before"})

	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if fake.Seen != "before" {
		t.Fatalf("editor was handed %q, want the existing note", fake.Seen)
	}
	if got, _ := env.Store.Get(7); got.Note != "rewritten" {
		t.Fatalf("Note = %q", got.Note)
	}
}

// --edit must not touch the path; only the note changes.
func TestEditDoesNotMoveTheSlot(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Editor = &editor.Fake{Result: "x"}
	env.Store.Put(slots.Slot{Number: 7, Path: "/original", Note: ""})

	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Path != "/original" {
		t.Fatalf("Path = %q, want it unchanged", got.Path)
	}
}

func TestEditFailsWhenTheEditorFails(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Editor = &editor.Fake{Err: errors.New("no editor")}
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "precious"})

	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err == nil {
		t.Fatal("edit = nil error after an editor failure, want an error")
	}
	if got, _ := env.Store.Get(7); got.Note != "precious" {
		t.Fatalf("Note = %q, want it untouched", got.Note)
	}
}

func TestEditOnAnEmptySlotFails(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err == nil {
		t.Fatal("editing an empty slot = nil error, want an error")
	}
}

func TestResetClearsTheNameAndNoteButKeepsThePath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/keep/me", Note: "stale"})

	if err := Run(env, cli.Command{Action: cli.ActionReset, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	got, ok := env.Store.Get(7)
	if !ok {
		t.Fatal("the slot was removed; --reset must keep it")
	}
	if got.Path != "/keep/me" {
		t.Fatalf("Path = %q, want it kept", got.Path)
	}
	if got.Name != "" || got.Note != "" {
		t.Fatalf("Name = %q, Note = %q, want both empty", got.Name, got.Note)
	}
	if !strings.Contains(out.String(), "slot 7") {
		t.Fatalf("output = %q, want a confirmation", out.String())
	}
}

func TestDeleteRemovesTheSlot(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionDelete, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Store.Get(7); ok {
		t.Fatal("slot 7 is still there")
	}
	if !strings.Contains(out.String(), "deleted") {
		t.Fatalf("output = %q, want a confirmation", out.String())
	}
}

func TestDeleteOnAnEmptySlotFails(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionDelete, Ref: "7"}); err == nil {
		t.Fatal("deleting an empty slot = nil error, want an error")
	}
}

func TestRename(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "exam42"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Name != "exam42" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestRenameWithAnEmptyNameDropsTheName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: ""}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Name != "" {
		t.Fatalf("Name = %q, want empty", got.Name)
	}
}

// A name that looks like a slot number would make the ref grammar ambiguous
// for the user even though the parser handles it, so it is rejected outright.
func TestRenameRejectsANumericName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x"})
	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "42"}); err == nil {
		t.Fatal("a purely numeric name was accepted")
	}
}

func TestRenameRejectsADuplicateName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 8, Path: "/b"})

	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "8", Name: "EXAM42"}); err == nil {
		t.Fatal("a duplicate name was accepted; refs would become ambiguous")
	}
}

// Renaming a slot to the name it already has is a no-op, not a duplicate.
func TestRenameToTheSameNameSucceeds(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/a"})
	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "exam42"}); err != nil {
		t.Fatalf("renaming to the same name failed: %v", err)
	}
}

func TestRenameRejectsANameWithSpaces(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/a"})
	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "two words"}); err == nil {
		t.Fatal("a name with a space was accepted")
	}
}

func TestSetNoteReplacesTheNote(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "old"})

	err := Run(env, cli.Command{Action: cli.ActionSetNote, Ref: "7", Note: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Note != "new" {
		t.Fatalf("Note = %q", got.Note)
	}
}

func TestSetNoteWithEmptyTextClearsIt(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "old"})

	if err := Run(env, cli.Command{Action: cli.ActionSetNote, Ref: "7", Note: ""}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Note != "" {
		t.Fatalf("Note = %q, want empty", got.Note)
	}
}
