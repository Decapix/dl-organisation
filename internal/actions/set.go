package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Set saves the current directory into a slot.
//
// An occupied slot is overwritten without asking — that is the design choice
// that keeps the common case one command long. What makes it safe is that
// nothing is ever destroyed implicitly: the name and the note survive the
// overwrite, and only an explicit -r or --delete removes them.
func Set(env *Env, cmd cli.Command) error {
	number, err := resolveSetTarget(env, cmd.Ref)
	if err != nil {
		return err
	}

	sl, existed := env.Store.Get(number)
	previousPath := sl.Path

	sl.Number = number
	sl.Path = env.Cwd
	sl.Updated = env.now()

	// -r runs first so that a -n or -m in the same command survives it.
	if cmd.Reset {
		sl.Name, sl.Note = "", ""
	}
	if cmd.Name != "" {
		sl.Name = cmd.Name
	}
	if cmd.Note != "" {
		sl.Note = cmd.Note
	}

	// The editor runs before the save so a failed edit cannot leave a
	// half-written note on disk.
	if cmd.Edit {
		edited, editErr := env.Editor.Edit(sl.Note)
		if editErr != nil {
			// A broken editor must not cost the user the save they asked for.
			fmt.Fprintf(env.Err, "warning: the editor failed, the note is unchanged: %v\n", editErr)
		} else {
			sl.Note = edited
		}
	}

	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}

	fmt.Fprintf(env.Out, "slot %d -> %s\n", number, slots.ShortPath(sl.Path, env.Home))
	// Only report a replacement when something actually moved.
	if existed && previousPath != "" && previousPath != sl.Path {
		fmt.Fprintf(env.Out, "  was %s\n", slots.ShortPath(previousPath, env.Home))
		if !cmd.Reset && (sl.Name != "" || sl.HasNote()) {
			fmt.Fprintf(env.Out, "  name and note kept — dl -e %d to update, dl -z %d -r to start clean\n",
				number, number)
		}
	}
	return nil
}

// resolveSetTarget turns --set's optional ref into a slot number.
//
// Unlike the other actions it accepts a number for a slot that does not exist
// yet, because creating slot 7 out of nothing is the normal case. A name, on
// the other hand, must already exist — there is nothing to name yet otherwise.
func resolveSetTarget(env *Env, ref string) (int, error) {
	if ref == "" {
		return env.Store.FirstFree()
	}
	if n, ok := slots.ParseSlotNumber(ref); ok {
		return n, nil
	}
	// Not a number: it has to be an existing name.
	sl, err := env.Store.Resolve(ref)
	if err != nil {
		return 0, fmt.Errorf("%w (a new slot must be given a number between %d and %d)",
			err, slots.MinSlot, slots.MaxSlot)
	}
	return sl.Number, nil
}
