package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// --about is a cd that also shows the slot itself: its number, name and path
// on one line, then the whole note. Plain `dl 7` prints only the note, which
// is right when you know where you are going; -a is for when you do not.
func TestAboutJumpsAndPrintsNamePathAndNote(t *testing.T) {
	env, out, _ := testEnv(t)
	dir := filepath.Join(env.Home, "work", "scraper")
	env.Exists = func(string) bool { return true }
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Name: "scraping", Path: dir,
		Note: "pagination stops at p.4\nfix the retry loop"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatalf("about: %v", err)
	}
	raw, err := os.ReadFile(env.CDFile)
	if err != nil || string(raw) != dir {
		t.Fatalf("cd file = %q, %v; want %q", raw, err, dir)
	}
	want := "slot 7  scraping  ~/work/scraper\n\npagination stops at p.4\nfix the retry loop\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

// An unnamed slot shows its display name, like every listing does, so the
// header never has a hole in it.
func TestAboutUsesTheDisplayNameOfAnUnnamedSlot(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Exists = func(string) bool { return true }
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
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
	env.Exists = func(string) bool { return true }
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Name: "bare", Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no note") || !strings.Contains(out.String(), "dl -e 7") {
		t.Fatalf("output = %q, want it to say there is no note and how to write one", out.String())
	}
}

// It is a cd, so it fails the same way a cd does when the directory is gone.
func TestAboutFailsOnADeadPath(t *testing.T) {
	env, _, _ := testEnv(t)
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Path: "/definitely/gone"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err == nil {
		t.Fatal("about on a dead path = nil error, want an error")
	}
	if _, err := os.Stat(env.CDFile); err == nil {
		t.Fatal("the cd file was written despite the failure")
	}
}

// Without the shell integration the header still carries the path, so the
// user can cd by hand; the hint about installing it goes to stderr as for cd.
func TestAboutWithoutIntegrationPrintsTheHint(t *testing.T) {
	env, out, errb := testEnv(t)
	env.Exists = func(string) bool { return true }
	env.Store.Put(slots.Slot{Number: 7, Name: "n", Path: "/tmp/x", Note: "note"})

	if err := Run(env, cli.Command{Action: cli.ActionAbout, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "/tmp/x") || !strings.Contains(out.String(), "note") {
		t.Fatalf("output = %q, want the path and the note", out.String())
	}
	if !strings.Contains(errb.String(), "dl init") {
		t.Fatalf("stderr = %q, want the integration hint", errb.String())
	}
}
