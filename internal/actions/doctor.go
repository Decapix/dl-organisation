package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Doctor reports slots whose directory has disappeared. It only reports:
// deciding whether a moved project should be repointed or dropped is the
// user's call, not the tool's.
func Doctor(env *Env, cmd cli.Command) error {
	var dead []slots.Slot
	for _, sl := range env.Store.All() {
		if !env.exists(sl) {
			dead = append(dead, sl)
		}
	}
	if len(dead) == 0 {
		fmt.Fprintln(env.Out, "every slot points at a directory that exists")
		return nil
	}

	fmt.Fprintf(env.Out, "%d slot%s point at a missing directory\n\n", len(dead), plural(len(dead)))
	for _, sl := range dead {
		fmt.Fprintf(env.Out, "  %d  %s\n", sl.Number, sl.DisplayName())
		fmt.Fprintf(env.Out, "     %s\n", sl.Path)
		fmt.Fprintf(env.Out, "     repoint: cd <somewhere> && dl -z %d      drop: dl -d %d\n\n",
			sl.Number, sl.Number)
	}
	return nil
}
