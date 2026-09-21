package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// Path prints a slot's path and nothing else, so it composes:
//
//	cp report.pdf "$(dl -p 7)"
//
// It deliberately succeeds on a dead path: the caller decides what a missing
// directory means for their command.
func Path(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Out, sl.Path)
	return nil
}
