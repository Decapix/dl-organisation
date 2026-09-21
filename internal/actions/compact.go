package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Compact renumbers the slots so they run 1, 2, 3 with no gaps, keeping their
// order and everything they hold.
//
// Deleting a slot leaves a hole, and holes accumulate. This closes them.
// Because the numbers are what you type, every move is printed: after
// compacting, the 4 in your fingers may be a 2.
func Compact(env *Env, cmd cli.Command) error {
	all := env.Store.All() // already ordered by number
	if len(all) == 0 {
		fmt.Fprintln(env.Out, "no slots to compact")
		return nil
	}

	type move struct{ from, to int }
	var moves []move
	renumbered := make([]slots.Slot, 0, len(all))
	for i, sl := range all {
		want := i + 1
		if sl.Number != want {
			moves = append(moves, move{from: sl.Number, to: want})
		}
		sl.Number = want
		renumbered = append(renumbered, sl)
	}

	if len(moves) == 0 {
		fmt.Fprintf(env.Out, "%d slot%s, already compact\n", len(all), plural(len(all)))
		return nil
	}

	// Clear first, then write: renumbering in place would overwrite a slot
	// whose own new number has not been assigned yet.
	for _, sl := range all {
		env.Store.Delete(sl.Number)
	}
	for _, sl := range renumbered {
		env.Store.Put(sl)
	}
	if err := saveChanges(env); err != nil {
		return err
	}

	fmt.Fprintf(env.Out, "compacted %d slot%s\n", len(all), plural(len(all)))
	for _, mv := range moves {
		fmt.Fprintf(env.Out, "  %d -> %d\n", mv.from, mv.to)
	}
	return nil
}
