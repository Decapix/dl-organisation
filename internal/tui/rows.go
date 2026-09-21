package tui

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// collapseRun is the length at which a stretch of empty slots stops being
// listed one by one. Slot numbers run to 1000, so someone who jumps from slot
// 1 to slot 500 would otherwise get 498 blank rows.
const collapseRun = 3

// row is one line of the list. It is either a slot, a single empty slot
// number, or a collapsed stretch of empty ones.
//
// Empty rows exist because the numbers are the interface: after deleting slot
// 5, seeing that 5 is now free is what tells you the number is available
// again.
type row struct {
	slot *slots.Slot // nil for an empty row
	from int         // first slot number the row covers
	to   int         // last one; equal to from unless the row is collapsed
}

// empty reports whether the row stands for free slot numbers.
func (r row) empty() bool { return r.slot == nil }

// collapsed reports whether the row stands for more than one free number.
func (r row) collapsed() bool { return r.slot == nil && r.to > r.from }

// label is what a collapsed run reads as.
func (r row) label() string {
	if r.collapsed() {
		return fmt.Sprintf("%d–%d empty", r.from, r.to)
	}
	return "empty"
}

// buildRows lays the slots out with their holes shown.
//
// The list runs from slot 1 to the last occupied slot and stops there:
// showing the free numbers past the end would mean showing hundreds of them,
// and `a` picks the lowest free slot anyway.
func buildRows(all []slots.Slot) []row {
	if len(all) == 0 {
		return nil
	}
	occupied := make(map[int]*slots.Slot, len(all))
	last := 0
	for i := range all {
		occupied[all[i].Number] = &all[i]
		if all[i].Number > last {
			last = all[i].Number
		}
	}

	var rows []row
	for n := 1; n <= last; {
		if sl, ok := occupied[n]; ok {
			rows = append(rows, row{slot: sl, from: n, to: n})
			n++
			continue
		}
		start := n
		for n <= last && occupied[n] == nil {
			n++
		}
		end := n - 1
		if end-start+1 >= collapseRun {
			rows = append(rows, row{from: start, to: end})
			continue
		}
		for i := start; i <= end; i++ {
			rows = append(rows, row{from: i, to: i})
		}
	}
	return rows
}

// slotRows wraps slots as rows with no holes, which is what a filtered list
// is: an empty slot is not something you were looking for.
func slotRows(all []slots.Slot) []row {
	rows := make([]row, 0, len(all))
	for i := range all {
		rows = append(rows, row{slot: &all[i], from: all[i].Number, to: all[i].Number})
	}
	return rows
}
