package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// --about shows a slot in full without moving the shell: number, name and
// path on one line, then the whole note. Plain `dl 7` prints the note only
// once you have arrived; -a is for reading it from wherever you are.
func TestAboutPrintsNamePathAndNote(t *testing.T) {
	env, out, _ := testEnv(t)
	dir := filepath.Join(env.Home, "work", "scraper")
	env.Store.Put(slots.Slot{Number: 7, Name: "scraping", Path: dir,
		Note: "pagination stops at p.4\nfix the retry loop"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatalf("about: %v", err)
	}
	want := "slot 7  scraping  ~/work/scraper\n\npagination stops at p.4\nfix the retry loop\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

// It must not cd: the cd file stays unwritten even with the integration on.
func TestAboutDoesNotMoveTheShell(t *testing.T) {
	env, _, errb := testEnv(t)
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Name: "n", Path: "/tmp/x", Note: "note"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(env.CDFile); err == nil {
		t.Fatal("the cd file was written; --about must not move the shell")
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing (no cd, so no integration hint)", errb.String())
	}
}

// An unnamed slot shows its display name, like every listing does, so the
// header never has a hole in it.
func TestAboutUsesTheDisplayNameOfAnUnnamedSlot(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/srv/coloriage", Note: "x"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "slot 7  coloriage  /srv/coloriage\n") {
		t.Fatalf("output = %q, want the directory's base name as the name", out.String())
	}
}

func TestAboutOnASlotWithoutANoteSaysSo(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "bare", Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no note") || !strings.Contains(out.String(), "dl -e 7") {
		t.Fatalf("output = %q, want it to say there is no note and how to write one", out.String())
	}
}

// Looking at a slot whose directory is gone still works, as --path does; the
// note may be the only record of what was there.
func TestAboutWorksOnADeadPath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/definitely/gone", Note: "was the old scraper"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatalf("about on a dead path: %v", err)
	}
	if !strings.Contains(out.String(), "was the old scraper") {
		t.Fatalf("output = %q, want the note", out.String())
	}
}
