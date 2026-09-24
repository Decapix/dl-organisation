package actions

import (
	"fmt"
	"os"

	"github.com/Decapix/dl-organisation/internal/cli"
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
	if !env.exists(sl) {
		return fmt.Errorf("slot %d points at %s, which no longer exists; repoint it with: dl -z %d",
			sl.Number, sl.Path, sl.Number)
	}

	if env.CDFile == "" {
		// No shell integration. Print the path so the user can still use it,
		// and say how to make the cd automatic.
		fmt.Fprintln(env.Out, sl.Path)
		fmt.Fprintln(env.Err, `hint: add eval "$(dl init zsh)" to your shell rc to cd automatically`)
		return nil
	}
	if err := os.WriteFile(env.CDFile, []byte(sl.Path), 0o600); err != nil {
		return fmt.Errorf("write the cd file: %w", err)
	}

	if sl.HasNote() {
		fmt.Fprintln(env.Out, sl.Note)
	}
	return nil
}
