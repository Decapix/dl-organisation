// Package actions implements one function per user-visible operation. Every
// action takes its dependencies through Env rather than reaching for globals,
// so each one is testable with a temp store and a byte buffer. No action calls
// os.Exit or prints to os.Stdout directly; that is main's job.
//
// Phase 2's TUI will call these same functions, which is why they do not
// assume they are running in a one-shot process.
package actions

import (
	"fmt"
	"io"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Env carries everything an action needs from the outside world.
type Env struct {
	Store *slots.Store
	Out   io.Writer // normal output
	Err   io.Writer // hints and warnings; never parsed by anything

	Cwd  string // the directory --set records
	Home string // used to shorten paths for display

	// CDFile is $DL_CD_FILE: the path the shell wrapper reads to perform the
	// cd. Empty means the integration is not installed.
	CDFile string

	Editor editor.Editor
	Now    func() time.Time // injected so tests get stable timestamps

	// Exists reports whether a slot's directory is still there. It is a seam
	// so that rendering tests can pin the marker column without creating real
	// directories. Nil means the real filesystem check.
	Exists func(path string) bool
}

// exists reports whether a slot still points at a real directory, through the
// injected seam when there is one.
func (e *Env) exists(sl slots.Slot) bool {
	if e.Exists != nil {
		return e.Exists(sl.Path)
	}
	return sl.Exists()
}

// now returns the current time through the injected clock.
func (e *Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

// Run dispatches a parsed command to its action. Keeping the switch here
// rather than in main means the TUI can reuse it unchanged.
func Run(env *Env, cmd cli.Command) error {
	switch cmd.Action {
	case cli.ActionCD:
		return CD(env, cmd)
	case cli.ActionSet:
		return Set(env, cmd)
	case cli.ActionSee:
		return See(env, cmd)
	case cli.ActionEdit:
		return Edit(env, cmd)
	case cli.ActionReset:
		return Reset(env, cmd)
	case cli.ActionDelete:
		return Delete(env, cmd)
	case cli.ActionPath:
		return Path(env, cmd)
	case cli.ActionRename:
		return Rename(env, cmd)
	case cli.ActionSetNote:
		return SetNote(env, cmd)
	case cli.ActionMove:
		return Move(env, cmd)
	case cli.ActionCompact:
		return Compact(env, cmd)
	case cli.ActionDoctor:
		return Doctor(env, cmd)
	}
	return fmt.Errorf("action %v is not implemented", cmd.Action)
}

// saveChanges persists the store. Actions call it once, at the end.
func saveChanges(env *Env) error {
	return env.Store.Save()
}
