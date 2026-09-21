// Command dl bookmarks directories in numbered slots and keeps a free-form
// note on each one.
//
// This file is wiring only: parse argv, build the environment, dispatch, and
// turn errors into exit codes. All behaviour lives in internal/actions so that
// the interactive browser can reuse it unchanged.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Decapix/dl-organisation/internal/actions"
	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/shellinit"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

// Exit codes. The shell wrapper propagates these, so `dl 7 && make` behaves.
const (
	exitOK      = 0
	exitFailure = 1 // a runtime failure: bad ref, dead path, store I/O
	exitUsage   = 2 // the command was typed wrong
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is main's body, extracted so the end-to-end test can call it directly.
func run(argv []string) int {
	cmd, err := cli.Parse(argv)
	if err != nil {
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "dl: %s\n", ue.Msg)
			fmt.Fprintf(os.Stderr, "see: dl %s--help\n", helpPointer(ue.Action))
			return exitUsage
		}
		fmt.Fprintf(os.Stderr, "dl: %v\n", err)
		return exitUsage
	}

	// These three answer without touching the store, so they keep working even
	// when the config directory is unreadable.
	switch cmd.Action {
	case cli.ActionHelp:
		fmt.Fprint(os.Stdout, cli.Help(cmd.HelpFor))
		return exitOK
	case cli.ActionVersion:
		fmt.Fprintf(os.Stdout, "dl %s\n", version)
		return exitOK
	case cli.ActionInit:
		script, err := shellinit.Script(cmd.Shell)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dl: %v\n", err)
			return exitFailure
		}
		fmt.Fprint(os.Stdout, script)
		return exitOK
	}

	if err := runStoreCommand(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "dl: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// runStoreCommand builds a Session and dispatches through it.
func runStoreCommand(cmd cli.Command) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locate the home directory: %w", err)
	}
	dir := slots.DefaultDir(os.Getenv, home)

	// One-time import of the legacy ~/.cdl layout. It is a no-op once the new
	// store exists, so calling it on every run costs a single stat.
	if n, err := slots.Migrate(home, dir); err != nil {
		fmt.Fprintf(os.Stderr, "dl: could not import ~/.cdl: %v\n", err)
	} else if n > 0 {
		fmt.Fprintf(os.Stderr, "dl: imported %d slot%s from ~/.cdl (kept as ~/.cdl.bak)\n",
			n, pluralS(n))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate the current directory: %w", err)
	}

	session := &actions.Session{
		Dir:    dir,
		Home:   home,
		Cwd:    cwd,
		CDFile: os.Getenv("DL_CD_FILE"),
		Editor: editor.OS{},
		Now:    time.Now,
		Err:    os.Stderr,
	}

	// The interactive browser arrives in task 8. Until then a bare `dl`
	// still falls back to listing.
	if cmd.Action == cli.ActionTUI {
		cmd.Action = cli.ActionSee
	}
	return session.Run(cmd, os.Stdout)
}

// helpPointer renders the action name for the "see: dl ... --help" line.
// ActionTUI has no flag of its own, so a bare `dl --help` is the right pointer.
func helpPointer(a cli.Action) string {
	if a == cli.ActionTUI {
		return ""
	}
	return a.String() + " "
}

// pluralS returns the "s" for a count.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
