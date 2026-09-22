package actions

import (
	"fmt"
	"strings"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Edit opens a slot's note in the user's editor. Only the note changes: the
// path and the name are left exactly as they were.
//
// The editor runs before the write lock is taken, not under it: see update.
// The slot is read once, without the lock, to seed the editor, and read again
// under the lock to apply the result, because anything may have happened to
// the store in between.
func Edit(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	edited, err := env.Editor.Edit(sl.Note)
	if err != nil {
		// Unlike --set, editing is the entire point of this command, so a
		// failed editor is a failed command.
		return fmt.Errorf("the note is unchanged: %w", err)
	}

	err = update(env, func(st *slots.Store) error {
		cur, ok := st.Get(sl.Number)
		if !ok {
			// Saving would recreate a slot somebody just removed on purpose.
			rescue(env, edited)
			return fmt.Errorf("slot %d was deleted while the editor was open; the note was not saved",
				sl.Number)
		}
		if cur.Note != sl.Note {
			fmt.Fprintf(env.Err, "warning: slot %d's note was changed by another dl while the editor was open; this version replaces it\n",
				sl.Number)
		}
		cur.Note = edited
		st.Put(cur)
		sl = cur
		return nil
	})
	if err != nil {
		return err
	}
	if sl.HasNote() {
		fmt.Fprintf(env.Out, "slot %d note saved\n", sl.Number)
	} else {
		fmt.Fprintf(env.Out, "slot %d note is now empty\n", sl.Number)
	}
	return nil
}

// Reset clears a slot's name and note while keeping where it points. It is the
// "same shelf, new project" command.
func Reset(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	sl.Name, sl.Note = "", ""
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "slot %d name and note cleared, still pointing at %s\n",
		sl.Number, slots.ShortPath(sl.Path, env.Home))
	return nil
}

// Delete removes a slot outright. It is the only command that does.
func Delete(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	env.Store.Delete(sl.Number)
	if err := saveChanges(env); err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "slot %d deleted (%s)\n", sl.Number, slots.ShortPath(sl.Path, env.Home))
	return nil
}

// Rename sets or clears a slot's name.
func Rename(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(cmd.Name)
	if err := validateName(env, sl.Number, name); err != nil {
		return err
	}
	sl.Name = name
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	if name == "" {
		fmt.Fprintf(env.Out, "slot %d name removed\n", sl.Number)
	} else {
		fmt.Fprintf(env.Out, "slot %d is now %q\n", sl.Number, name)
	}
	return nil
}

// SetNote replaces a slot's note with the given text.
func SetNote(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	sl.Note = strings.TrimRight(cmd.Note, " \t\r\n")
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	if sl.HasNote() {
		fmt.Fprintf(env.Out, "slot %d note set\n", sl.Number)
	} else {
		fmt.Fprintf(env.Out, "slot %d note cleared\n", sl.Number)
	}
	return nil
}

// validateName rejects names that would make references ambiguous: a purely
// numeric name would collide with slot numbers, a name with a space could not
// be typed as a single argument, and a duplicate would make both slots
// unreachable by name.
func validateName(env *Env, number int, name string) error {
	if name == "" {
		return nil
	}
	if _, numeric := slots.ParseSlotNumber(name); numeric {
		return fmt.Errorf("%q is a slot number, not a usable name", name)
	}
	if strings.ContainsAny(name, " \t") {
		return fmt.Errorf("a name cannot contain spaces")
	}
	for _, other := range env.Store.All() {
		if other.Number != number && strings.EqualFold(other.Name, name) {
			return fmt.Errorf("slot %d is already named %q", other.Number, other.Name)
		}
	}
	return nil
}
