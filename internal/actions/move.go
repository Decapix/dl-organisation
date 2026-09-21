package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Move puts a slot at another number.
//
// Two cases, and the difference matters:
//
//   - the target number is free: the slot takes it, nothing else changes;
//   - the target number is in use: the slot takes that position in the
//     ordering and the slots in between shift by one to fill the hole it
//     left behind.
//
// In both cases the set of numbers in use is unchanged. A move rearranges
// what sits where; it never closes a gap on its own, because closing gaps is
// what --compact is for and doing it silently here would renumber slots the
// user never touched.
func Move(env *Env, cmd cli.Command) error {
	src, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	to, ok := slots.ParseSlotNumber(cmd.To)
	if !ok {
		return fmt.Errorf("%q is not a slot number between %d and %d",
			cmd.To, slots.MinSlot, slots.MaxSlot)
	}
	if to == src.Number {
		fmt.Fprintf(env.Out, "slot %d is already there\n", to)
		return nil
	}

	if _, taken := env.Store.Get(to); !taken {
		// The simple case: a free number just changes hands.
		origin := src.Number
		env.Store.Delete(origin)
		src.Number = to
		env.Store.Put(src)
		if err := saveChanges(env); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "%d -> %d\n", origin, to)
		return nil
	}

	// The target is occupied, so this is a reordering. Take the slot out of
	// the sequence and put it back at the target's position; the numbers
	// themselves stay where they are and the contents slide along them.
	all := env.Store.All() // ordered by number
	numbers := make([]int, len(all))
	from, at := -1, -1
	for i, sl := range all {
		numbers[i] = sl.Number
		if sl.Number == src.Number {
			from = i
		}
		if sl.Number == to {
			at = i
		}
	}

	ordered := make([]slots.Slot, 0, len(all))
	ordered = append(ordered, all[:from]...)
	ordered = append(ordered, all[from+1:]...)
	// `at` is the target's index before the removal. Inserting there lands
	// the slot on the target number whichever direction it came from.
	tail := append([]slots.Slot{}, ordered[at:]...)
	ordered = append(ordered[:at], src)
	ordered = append(ordered, tail...)

	var moves [][2]int
	for i, sl := range ordered {
		if sl.Number != numbers[i] {
			moves = append(moves, [2]int{sl.Number, numbers[i]})
		}
	}
	for _, sl := range all {
		env.Store.Delete(sl.Number)
	}
	for i := range ordered {
		ordered[i].Number = numbers[i]
		env.Store.Put(ordered[i])
	}
	if err := saveChanges(env); err != nil {
		return err
	}

	for _, mv := range moves {
		fmt.Fprintf(env.Out, "%d -> %d\n", mv[0], mv[1])
	}
	return nil
}
