package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEnv builds a getenv function from a map.
func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolvePrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"DL_EDITOR wins", map[string]string{"DL_EDITOR": "a", "VISUAL": "b", "EDITOR": "c"}, "a"},
		{"VISUAL next", map[string]string{"VISUAL": "b", "EDITOR": "c"}, "b"},
		{"EDITOR next", map[string]string{"EDITOR": "c"}, "c"},
		{"vi is the floor", map[string]string{}, "vi"},
		{"blank values are skipped", map[string]string{"DL_EDITOR": "", "EDITOR": "c"}, "c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Resolve(fakeEnv(c.env)); got != c.want {
				t.Fatalf("Resolve() = %q, want %q", got, c.want)
			}
		})
	}
}

// writeScript creates an executable shell script and returns its path.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-editor")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEditRoundTrip(t *testing.T) {
	// The editor appends a line to whatever it is given.
	script := writeScript(t, `printf 'appended\n' >> "$1"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	got, err := e.Edit("original")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got != "original\nappended" {
		t.Fatalf("Edit() = %q, want %q", got, "original\nappended")
	}
}

func TestEditSeesTheInitialContent(t *testing.T) {
	// The editor copies its input somewhere we can inspect.
	out := filepath.Join(t.TempDir(), "seen")
	script := writeScript(t, `cat "$1" > `+out)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	if _, err := e.Edit("line1\nline2"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(raw), "\n") != "line1\nline2" {
		t.Fatalf("editor saw %q", string(raw))
	}
}

func TestEditTrimsTrailingWhitespace(t *testing.T) {
	script := writeScript(t, `printf '  \n\n\n' >> "$1"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	got, err := e.Edit("note")
	if err != nil {
		t.Fatal(err)
	}
	if got != "note" {
		t.Fatalf("Edit() = %q, want %q (trailing blank lines must go)", got, "note")
	}
}

func TestEditSupportsAnEditorWithArguments(t *testing.T) {
	script := writeScript(t, `printf 'flag=%s\n' "$1" >> "$2"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script + " -w"})}

	got, err := e.Edit("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "flag=-w" {
		t.Fatalf("Edit() = %q, want %q", got, "flag=-w")
	}
}

func TestEditReturnsAnErrorWhenTheEditorFails(t *testing.T) {
	script := writeScript(t, `exit 3`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	if _, err := e.Edit("note"); err == nil {
		t.Fatal("Edit() = nil error after a non-zero editor exit, want an error")
	}
}

func TestEditLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	script := writeScript(t, `printf 'x' >> "$1"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	if _, err := e.Edit("note"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), "dl-note") {
			t.Fatalf("temp file left behind: %s", en.Name())
		}
	}
}
