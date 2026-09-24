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

	"github.com/charmbracelet/lipgloss"

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

	// Renderer colours the output. Nil means plain text. main hands in one
	// built on stdout; lipgloss itself drops the colour when stdout is not a
	// terminal or NO_COLOR is set, so pipes always see plain text.
	Renderer *lipgloss.Renderer

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
	case cli.ActionAbout:
		return About(env, cmd)
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

// update runs fn against a store that holds the write lock, then saves.
//
// It exists for the two actions that run the editor. The editor is
// interactive and may stay open for an hour, and the exclusive lock waits
// silently, so holding it across the editor froze every other dl in every
// other shell. Those actions therefore run against a read-only store, do
// the editor round trip, and call update for just the write. fn is handed a
// freshly loaded store rather than env.Store, whose contents are as old as
// the editor session.
//
// When env.Store already holds the lock — the one-shot tests build their Env
// that way — fn runs against it directly; opening a second locked store on
// the same directory from the same process would wait on itself forever.
func update(env *Env, fn func(st *slots.Store) error) error {
	if env.Store.Writable() {
		if err := fn(env.Store); err != nil {
			return err
		}
		return env.Store.Save()
	}
	st, err := openForUpdate(env.Store.Dir(), env.Err)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := fn(st); err != nil {
		return err
	}
	return st.Save()
}

// openForUpdate takes the write lock, telling the user on errw when it has
// to wait for it. The wait is otherwise silent and, behind an editor left
// open in another shell, can last a long time; a frozen prompt with nothing
// on it reads as a hang.
func openForUpdate(dir string, errw io.Writer) (*slots.Store, error) {
	return slots.OpenForUpdateNotify(dir, func() {
		if errw == nil {
			return
		}
		fmt.Fprintln(errw, "waiting for another dl to release the store...")
		fmt.Fprintln(errw, "  (if this never returns, another dl is still busy — pgrep -a dl finds it)")
	})
}

// rescue prints text that could not be saved, so that a note typed into the
// editor is never simply gone.
func rescue(env *Env, text string) {
	if text == "" {
		return
	}
	fmt.Fprintf(env.Err, "the text you typed was:\n%s\n", text)
}
