package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// captureRun runs the program with argv and returns stdout, stderr and the
// exit code. It swaps the real os.Stdout/os.Stderr because run writes to them
// directly, which is exactly what the test needs to exercise.
func captureRun(t *testing.T, argv ...string) (string, string, int) {
	t.Helper()

	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	code := run(argv)

	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = origOut, origErr

	var outB, errB strings.Builder
	buf := make([]byte, 4096)
	for _, pair := range []struct {
		f *os.File
		b *strings.Builder
	}{{outR, &outB}, {errR, &errB}} {
		for {
			n, err := pair.f.Read(buf)
			pair.b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		pair.f.Close()
	}
	return outB.String(), errB.String(), code
}

// setupEnv points dl at a throwaway store and home for the duration of a test.
func setupEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DL_DIR", filepath.Join(home, ".config", "dl"))
	t.Setenv("DL_CD_FILE", "")
	return home
}

// TestFullLifecycle walks the path a real user takes: save, list, note, jump,
// rename, jump by name, reset, delete.
func TestFullLifecycle(t *testing.T) {
	setupEnv(t)
	// t.Chdir restores the previous directory at cleanup; a bare os.Chdir
	// would leave later tests running inside a deleted temp directory.
	work := t.TempDir()
	t.Chdir(work)

	if out, errS, code := captureRun(t, "-z", "7"); code != 0 {
		t.Fatalf("save failed: code=%d out=%q err=%q", code, out, errS)
	}

	out, _, code := captureRun(t, "-s")
	if code != 0 || !strings.Contains(out, "1 slot") {
		t.Fatalf("list after save: code=%d out=%q", code, out)
	}

	if _, _, code := captureRun(t, "-m", "7", "pick up at step 3"); code != 0 {
		t.Fatalf("setting the note failed with code %d", code)
	}

	// Jumping writes the path to the cd file and prints the note.
	cdfile := filepath.Join(t.TempDir(), "cd")
	t.Setenv("DL_CD_FILE", cdfile)
	out, _, code = captureRun(t, "7")
	if code != 0 {
		t.Fatalf("cd failed with code %d", code)
	}
	if !strings.Contains(out, "pick up at step 3") {
		t.Fatalf("cd did not print the note: %q", out)
	}
	raw, err := os.ReadFile(cdfile)
	if err != nil || string(raw) != work {
		t.Fatalf("cd file = %q (err %v), want %q", string(raw), err, work)
	}

	if _, _, code := captureRun(t, "-n", "7", "project"); code != 0 {
		t.Fatalf("rename failed with code %d", code)
	}
	// The name and a unique prefix of it both resolve.
	for _, ref := range []string{"project", "proj"} {
		if _, _, code := captureRun(t, ref); code != 0 {
			t.Fatalf("cd by %q failed with code %d", ref, code)
		}
	}

	if _, _, code := captureRun(t, "-r", "7"); code != 0 {
		t.Fatalf("reset failed with code %d", code)
	}
	out, _, _ = captureRun(t, "-s", "-l")
	if strings.Contains(out, "pick up at step 3") {
		t.Fatalf("the note survived --reset: %q", out)
	}

	if _, _, code := captureRun(t, "-d", "7"); code != 0 {
		t.Fatalf("delete failed with code %d", code)
	}
	out, _, _ = captureRun(t, "-s")
	if !strings.Contains(out, "no slots yet") {
		t.Fatalf("store is not empty after delete: %q", out)
	}
}

// The regression test for the zsh version's worst bug, at the binary level.
func TestOverwriteKeepsTheNameAndNote(t *testing.T) {
	setupEnv(t)
	first, second := t.TempDir(), t.TempDir()

	t.Chdir(first)
	captureRun(t, "-z", "3", "-n", "thing", "-m", "important context")

	t.Chdir(second)
	out, _, code := captureRun(t, "-z", "3")
	if code != 0 {
		t.Fatalf("overwrite failed with code %d", code)
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("overwrite output = %q, want a note-kept line", out)
	}

	out, _, _ = captureRun(t, "-s", "-l")
	if !strings.Contains(out, "important context") {
		t.Fatalf("the note was lost on overwrite: %q", out)
	}
	if !strings.Contains(out, "thing") {
		t.Fatalf("the name was lost on overwrite: %q", out)
	}
}

func TestExitCodes(t *testing.T) {
	setupEnv(t)
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{"success", []string{"-s"}, 0},
		{"unknown flag", []string{"-x"}, 2},
		{"two actions", []string{"-s", "-d", "1"}, 2},
		{"missing ref", []string{"-d"}, 2},
		{"empty slot", []string{"-c", "500"}, 1},
		{"unknown name", []string{"nope"}, 1},
		{"help", []string{"-h"}, 0},
		{"version", []string{"--version"}, 0},
		{"init", []string{"init", "zsh"}, 0},
		{"unknown shell", []string{"init", "csh"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, code := captureRun(t, c.argv...); code != c.want {
				t.Fatalf("exit code = %d, want %d", code, c.want)
			}
		})
	}
}

// A usage error must point at the relevant help page, not the global one.
func TestUsageErrorPointsAtTheActionHelp(t *testing.T) {
	setupEnv(t)
	_, errS, code := captureRun(t, "-z", "-l")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errS, "dl --set --help") {
		t.Fatalf("stderr = %q, want a pointer to the --set help", errS)
	}
}

// The legacy store must be imported on the first run of the real binary.
func TestMigrationHappensOnFirstRun(t *testing.T) {
	home := setupEnv(t)
	cdl := filepath.Join(home, ".cdl")
	if err := os.MkdirAll(filepath.Join(cdl, "descriptions"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cdl, "slots.json"),
		[]byte(`{"7": {"path": "/tmp", "comment": "old comment"}}`), 0o644)
	os.WriteFile(filepath.Join(cdl, "descriptions", "7.txt"),
		[]byte("old description"), 0o644)

	out, errS, code := captureRun(t, "-s", "-l")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errS)
	}
	if !strings.Contains(errS, "imported 1 slot") {
		t.Errorf("stderr = %q, want an import notice", errS)
	}
	for _, want := range []string{"old comment", "old description"} {
		if !strings.Contains(out, want) {
			t.Errorf("listing = %q, want it to contain %q", out, want)
		}
	}
	if _, err := os.Stat(cdl + ".bak"); err != nil {
		t.Errorf("~/.cdl.bak missing: %v", err)
	}
}

// A bare `dl` opens the browser, which wants a terminal. Under `go test`
// there is none. Whether bubbletea errors out or reads EOF and exits cleanly
// is its business; what this test pins is that the binary terminates, does
// not panic, and reports a code we recognise. Asserting a specific code here
// would be asserting a dependency's internals.
func TestBareDlWithoutATerminalTerminates(t *testing.T) {
	setupEnv(t)
	done := make(chan int, 1)
	go func() {
		_, _, code := captureRun(t)
		done <- code
	}()
	select {
	case code := <-done:
		if code != exitOK && code != exitFailure {
			t.Fatalf("exit code = %d, want 0 or 1", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a bare dl hung with no terminal")
	}
}
