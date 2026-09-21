package slots

import (
	"fmt"
	"strconv"
	"strings"
)

// AmbiguousRefError is returned when a name prefix matches more than one slot.
// It carries the candidates so the caller can print them, which turns a dead
// end into a menu.
type AmbiguousRefError struct {
	Ref        string
	Candidates []Slot
}

func (e *AmbiguousRefError) Error() string {
	names := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		names = append(names, fmt.Sprintf("%d %s", c.Number, c.DisplayName()))
	}
	return fmt.Sprintf("%q is ambiguous: %s", e.Ref, strings.Join(names, ", "))
}

// ParseSlotNumber reports whether ref is a slot number in range. It is the
// first rule of Resolve and is also used by --set, which accepts a number for
// a slot that does not exist yet.
func ParseSlotNumber(ref string) (int, bool) {
	if ref == "" {
		return 0, false
	}
	for _, r := range ref {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(ref)
	if err != nil || n < MinSlot || n > MaxSlot {
		return 0, false
	}
	return n, true
}

// Resolve turns a user-supplied reference into a slot.
//
// A number in range is looked up directly and never falls through to the name
// rules, so numbers and names can never shadow each other. Anything else is
// matched against names in four tiers, most specific first:
//
//  1. exact match on an explicit name
//  2. exact match on a display name (a slot's directory base name)
//  3. prefix match on an explicit name
//  4. prefix match on a display name
//
// The first tier with any match decides; several matches inside one tier is an
// *AmbiguousRefError listing them. Matching display names is what makes an
// unnamed slot reachable by what the listing shows for it, which matters right
// after an import from ~/.cdl where every slot arrives unnamed. Keeping
// explicit names in their own tiers means a name you chose always beats one
// derived from a path.
func (s *Store) Resolve(ref string) (Slot, error) {
	if n, ok := ParseSlotNumber(ref); ok {
		sl, exists := s.Get(n)
		if !exists {
			return Slot{}, fmt.Errorf("slot %d is empty; save one with: dl -z %d", n, n)
		}
		return sl, nil
	}
	if ref == "" {
		return Slot{}, fmt.Errorf("missing slot reference")
	}

	needle := strings.ToLower(ref)
	var exactName, exactDisplay, prefixName, prefixDisplay []Slot
	for _, sl := range s.All() {
		name := strings.ToLower(sl.Name)
		display := strings.ToLower(sl.DisplayName())

		switch {
		case name != "" && name == needle:
			exactName = append(exactName, sl)
		case display == needle:
			exactDisplay = append(exactDisplay, sl)
		case name != "" && strings.HasPrefix(name, needle):
			prefixName = append(prefixName, sl)
		case strings.HasPrefix(display, needle):
			prefixDisplay = append(prefixDisplay, sl)
		}
	}

	for _, tier := range [][]Slot{exactName, exactDisplay, prefixName, prefixDisplay} {
		switch len(tier) {
		case 0:
			continue
		case 1:
			return tier[0], nil
		default:
			return Slot{}, &AmbiguousRefError{Ref: ref, Candidates: tier}
		}
	}
	return Slot{}, fmt.Errorf("no slot matches %q; list them with: dl -s", ref)
}
