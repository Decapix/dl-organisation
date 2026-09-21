package actions

import (
	"errors"
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestSetSavesTheCurrentDirectory(t *testing.T) {
	env, out, _ := testEnv(t)
	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7"})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := env.Store.Get(7)
	if !ok {
		t.Fatal("slot 7 was not created")
	}
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the cwd", got.Path)
	}
	if got.Updated != env.Now() {
		t.Fatalf("Updated = %v, want the injected clock value", got.Updated)
	}
	if !strings.Contains(out.String(), "slot 7") {
		t.Fatalf("output = %q, want a confirmation naming the slot", out.String())
	}
}

func TestSetWithNoRefUsesTheFirstFreeSlot(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Path: "/a"})
	env.Store.Put(slots.Slot{Number: 2, Path: "/b"})

	if err := Run(env, cli.Command{Action: cli.ActionSet}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Store.Get(3); !ok {
		t.Fatal("slot 3 was not chosen as the first free slot")
	}
}

// The regression test for the destructive bug in the zsh version: overwriting
// a slot must keep the name and the note.
func TestSetKeepsTheNameAndNoteOnOverwrite(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/old/path", Note: "where I left off"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the new cwd", got.Path)
	}
	if got.Name != "exam42" {
		t.Fatalf("Name = %q, want it kept", got.Name)
	}
	if got.Note != "where I left off" {
		t.Fatalf("Note = %q, want it kept", got.Note)
	}
	// The user must be told what moved and that the note survived.
	if !strings.Contains(out.String(), "/old/path") {
		t.Errorf("output = %q, want the previous path", out.String())
	}
	if !strings.Contains(out.String(), "kept") {
		t.Errorf("output = %q, want a note-kept line", out.String())
	}
}

// Re-saving the same directory is routine; it should not print a "was" line.
func TestSetOnTheSamePathIsQuiet(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/cwd", Note: "n"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "was") {
		t.Fatalf("output = %q, want no \"was\" line when the path is unchanged", out.String())
	}
}

func TestSetWithReset(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/old", Note: "stale"})

	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Reset: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Name != "" || got.Note != "" {
		t.Fatalf("after -r: Name = %q, Note = %q, want both empty", got.Name, got.Note)
	}
}

func TestSetWithName(t *testing.T) {
	env, _, _ := testEnv(t)
	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Name: "exam42"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Name != "exam42" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestSetWithNote(t *testing.T) {
	env, _, _ := testEnv(t)
	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Note: "fix the loop"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Note != "fix the loop" {
		t.Fatalf("Note = %q", got.Note)
	}
}

// -r runs before -n and -m, so `dl -z 7 -r -n new` leaves the new name in
// place rather than wiping it.
func TestSetResetThenNameKeepsTheNewName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "old", Path: "/old", Note: "stale"})

	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Reset: true, Name: "new"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Name != "new" {
		t.Fatalf("Name = %q, want %q", got.Name, "new")
	}
	if got.Note != "" {
		t.Fatalf("Note = %q, want empty", got.Note)
	}
}

func TestSetWithEditOpensTheEditorOnTheCurrentNote(t *testing.T) {
	env, _, _ := testEnv(t)
	fake := &editor.Fake{Result: "brand new note"}
	env.Editor = fake
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/cwd", Note: "existing"})

	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	if fake.Seen != "existing" {
		t.Fatalf("editor was handed %q, want the existing note", fake.Seen)
	}
	if got, _ := env.Store.Get(7); got.Note != "brand new note" {
		t.Fatalf("Note = %q", got.Note)
	}
}

// If the editor fails, the path was still saved but the note must survive.
func TestSetWithEditKeepsTheNoteWhenTheEditorFails(t *testing.T) {
	env, _, errb := testEnv(t)
	env.Editor = &editor.Fake{Err: errors.New("editor exploded")}
	env.Store.Put(slots.Slot{Number: 7, Path: "/old", Note: "precious"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Edit: true}); err != nil {
		t.Fatalf("set should not fail because the editor did: %v", err)
	}
	got, _ := env.Store.Get(7)
	if got.Note != "precious" {
		t.Fatalf("Note = %q, want it untouched", got.Note)
	}
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the save to have happened anyway", got.Path)
	}
	if !strings.Contains(errb.String(), "editor") {
		t.Fatalf("stderr = %q, want a warning about the editor", errb.String())
	}
}

func TestSetRejectsAnOutOfRangeSlot(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "0"}); err == nil {
		t.Fatal("slot 0 was accepted")
	}
	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "1001"}); err == nil {
		t.Fatal("slot 1001 was accepted")
	}
}

// A name may be used as the ref for --set, so `dl -z exam42` re-points an
// existing named slot without having to remember its number.
func TestSetAcceptsANameAsTheRef(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/old"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "exam42"}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the cwd", got.Path)
	}
}

func TestSetRejectsAnUnknownName(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "nope"}); err == nil {
		t.Fatal("--set with an unknown name was accepted")
	}
}
