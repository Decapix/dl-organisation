package actions

import (
	"fmt"
	"os"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// CD resolves a reference and hands the target directory to the shell wrapper
// through $DL_CD_FILE, then prints the slot's note.
//
// Printing the note is the feature the old tool only half had: you jump back
// into a project and immediately read what you were in the middle of.
func CD(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	moved, err := jump(env, sl)
	if err != nil {
		return err
	}
	if !moved {
		// No shell integration. Print the path so the user can still use it.
		fmt.Fprintln(env.Out, sl.Path)
		return nil
	}
	if sl.HasNote() {
		fmt.Fprintln(env.Out, sl.Note)
	}
	return nil
}

// About is a cd that also shows the slot itself: one line with its number,
// name and path, then the whole note. Plain `dl 7` prints only the note,
// which is right when you know where you are going; -a is for when you do
// not, or when you want to re-read a note in full.
func About(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	if _, err := jump(env, sl); err != nil {
		return err
	}
	// The header carries the path, so even without the shell integration
	// the user has what they need to cd by hand.
	fmt.Fprintf(env.Out, "slot %d  %s  %s\n\n", sl.Number, sl.DisplayName(),
		slots.ShortPath(sl.Path, env.Home))
	if sl.HasNote() {
		fmt.Fprintln(env.Out, sl.Note)
	} else {
		fmt.Fprintf(env.Out, "no note — write one with: dl -e %d\n", sl.Number)
	}
	return nil
}

// jump checks that the slot's directory still exists and writes its path to
// the cd file for the shell wrapper. It reports whether the shell will move:
// false means the integration is not installed, in which case it has
// already printed the hint that says how to install it.
func jump(env *Env, sl slots.Slot) (bool, error) {
	if !env.exists(sl) {
		return false, fmt.Errorf("slot %d points at %s, which no longer exists; repoint it with: dl -z %d",
			sl.Number, sl.Path, sl.Number)
	}
	if env.CDFile == "" {
		fmt.Fprintln(env.Err, `hint: add eval "$(dl init zsh)" to your shell rc to cd automatically`)
		return false, nil
	}
	if err := os.WriteFile(env.CDFile, []byte(sl.Path), 0o600); err != nil {
		return false, fmt.Errorf("write the cd file: %w", err)
	}
	return true, nil
}
