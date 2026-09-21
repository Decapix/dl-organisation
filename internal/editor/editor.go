// Package editor runs the user's text editor over a slot's note. The round
// trip goes through a temp file because that is the only interface every
// editor agrees on.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Editor is the seam the actions depend on. Production code uses OS; tests
// use Fake.
type Editor interface {
	// Edit shows initial to the user and returns what they saved. A non-nil
	// error means the note must be left untouched.
	Edit(initial string) (string, error)
}

// Resolve picks the editor command: $DL_EDITOR, then $VISUAL, then $EDITOR,
// then vi. Honouring $EDITOR is what the old zsh implementation failed to do —
// it hardcoded vim.
func Resolve(getenv func(string) string) string {
	for _, key := range []string{"DL_EDITOR", "VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
	}
	return "vi"
}

// OS is the real editor: it spawns a process attached to the terminal.
type OS struct {
	// Getenv is injected so tests do not have to mutate the process
	// environment. Nil means os.Getenv.
	Getenv func(string) string

	// Stdin, Stdout and Stderr are what the editor process inherits. Nil
	// means the corresponding os.Std* file.
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

// Edit writes initial to a temp file, runs the editor on it, and reads the
// result back with trailing whitespace removed.
func (e OS) Edit(initial string) (string, error) {
	getenv := e.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	// The .md suffix gives editors a reason to turn on soft wrap and
	// spell-checking, which is what a note wants.
	f, err := os.CreateTemp("", "dl-note-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp note: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)

	body := initial
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n" // leave the cursor on a fresh line
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return "", fmt.Errorf("write temp note: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp note: %w", err)
	}

	// The editor setting may carry arguments, e.g. "code -w" or "emacsclient
	// -nw", so split it rather than treating it as a bare program name.
	parts := strings.Fields(Resolve(getenv))
	cmd := exec.Command(parts[0], append(parts[1:], name)...)
	cmd.Stdin = orStd(e.Stdin, os.Stdin)
	cmd.Stdout = orStd(e.Stdout, os.Stdout)
	cmd.Stderr = orStd(e.Stderr, os.Stderr)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", parts[0], err)
	}

	raw, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read temp note: %w", err)
	}
	// Editors add a trailing newline; some users leave blank lines behind.
	// Neither is content.
	return strings.TrimRight(string(raw), " \t\r\n"), nil
}

// orStd returns f, or fallback when f is nil.
func orStd(f, fallback *os.File) *os.File {
	if f == nil {
		return fallback
	}
	return f
}
