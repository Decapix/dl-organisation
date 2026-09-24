package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// About shows a slot without going there: one line with its number, name and
// path, then the whole note. It is the read-only counterpart of `dl 7`,
// which prints the note only once you have arrived. Like --path it works on
// a dead directory, because looking is never the thing that should fail.
func About(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "slot %d  %s  %s\n\n", sl.Number, sl.DisplayName(),
		slots.ShortPath(sl.Path, env.Home))
	if sl.HasNote() {
		fmt.Fprintln(env.Out, sl.Note)
	} else {
		fmt.Fprintf(env.Out, "no note — write one with: dl -e %d\n", sl.Number)
	}
	return nil
}
