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

// Resolve turns a user-supplied reference into a slot, in this order:
//
//  1. an in-range number   -> that slot, or an "empty" error
//  2. an exact name        -> that slot (case-insensitive)
//  3. a unique name prefix -> that slot
//  4. several prefixes     -> *AmbiguousRefError listing them
//  5. anything else        -> not found
//
// Rule 1 never falls through to the name rules, so numbers and names can never
// shadow each other.
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
	var prefixes []Slot
	for _, sl := range s.All() {
		name := strings.ToLower(sl.Name)
		if name == "" {
			continue
		}
		if name == needle {
			return sl, nil // exact match wins outright
		}
		if strings.HasPrefix(name, needle) {
			prefixes = append(prefixes, sl)
		}
	}
	switch len(prefixes) {
	case 1:
		return prefixes[0], nil
	case 0:
		return Slot{}, fmt.Errorf("no slot matches %q; list them with: dl -s", ref)
	default:
		return Slot{}, &AmbiguousRefError{Ref: ref, Candidates: prefixes}
	}
}
