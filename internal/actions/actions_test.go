package actions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// testEnv builds an Env backed by a temp store, with buffers for output and a
// fake editor. It returns the env and the two buffers.
func testEnv(t *testing.T) (*Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	st, err := slots.OpenForUpdate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	var out, errb bytes.Buffer
	env := &Env{
		Store:  st,
		Out:    &out,
		Err:    &errb,
		Cwd:    "/tmp/cwd",
		Home:   "/home/u",
		Editor: &editor.Fake{},
		Now:    func() time.Time { return time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) },
	}
	return env, &out, &errb
}

func TestCDWritesTheTargetToTheCDFile(t *testing.T) {
	env, out, _ := testEnv(t)
	dir := t.TempDir()
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: dir})

	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err != nil {
		t.Fatalf("cd: %v", err)
	}
	raw, err := os.ReadFile(env.CDFile)
	if err != nil {
		t.Fatalf("cd file not written: %v", err)
	}
	if string(raw) != dir {
		t.Fatalf("cd file = %q, want %q", string(raw), dir)
	}
	if out.String() != "" {
		t.Fatalf("cd printed %q on a slot with no note, want nothing", out.String())
	}
}

// Jumping into a slot prints its note. This is the whole point of the tool:
// you arrive and immediately see where you left off.
func TestCDPrintsTheNote(t *testing.T) {
	env, out, _ := testEnv(t)
	dir := t.TempDir()
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Path: dir, Note: "fix the retry loop\nthen export to csv"})

	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fix the retry loop") {
		t.Fatalf("cd output = %q, want the note", out.String())
	}
	if !strings.Contains(out.String(), "then export to csv") {
		t.Fatalf("cd output = %q, want the whole note", out.String())
	}
}

// Without the shell integration there is no cd file, so print the path and say
// how to fix it rather than failing silently.
func TestCDWithoutTheShellIntegrationPrintsThePath(t *testing.T) {
	env, out, errb := testEnv(t)
	dir := t.TempDir()
	env.CDFile = ""
	env.Store.Put(slots.Slot{Number: 7, Path: dir})

	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != dir {
		t.Fatalf("stdout = %q, want the path", out.String())
	}
	if !strings.Contains(errb.String(), "dl init") {
		t.Fatalf("stderr = %q, want a hint about dl init", errb.String())
	}
}

func TestCDOnADeadPathFails(t *testing.T) {
	env, _, _ := testEnv(t)
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Path: "/definitely/not/here"})

	err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"})
	if err == nil {
		t.Fatal("cd into a dead path = nil error, want an error")
	}
	if _, statErr := os.Stat(env.CDFile); statErr == nil {
		t.Fatal("the cd file was written despite the failure")
	}
}

func TestCDOnAnEmptySlotFails(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err == nil {
		t.Fatal("cd into an empty slot = nil error, want an error")
	}
}

func TestPathPrintsOnlyThePath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "a note that must not be printed"})

	if err := Run(env, cli.Command{Action: cli.ActionPath, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "/tmp/x\n" {
		t.Fatalf("path output = %q, want %q", out.String(), "/tmp/x\n")
	}
}

// --path is for scripting, so it must work even when the directory is gone;
// the caller decides what that means.
func TestPathWorksOnADeadPath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/gone"})
	if err := Run(env, cli.Command{Action: cli.ActionPath, Ref: "7"}); err != nil {
		t.Fatalf("path on a dead slot returned an error: %v", err)
	}
	if out.String() != "/gone\n" {
		t.Fatalf("output = %q", out.String())
	}
}
