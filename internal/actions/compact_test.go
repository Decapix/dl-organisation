package actions

import (
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestCompactClosesTheGaps(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Name: "cpp09", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 4, Name: "ir", Path: "/b", Note: "landing"})
	env.Store.Put(slots.Slot{Number: 9, Path: "/c"})

	if err := Run(env, cli.Command{Action: cli.ActionCompact}); err != nil {
		t.Fatal(err)
	}

	all := env.Store.All()
	if len(all) != 3 {
		t.Fatalf("Len = %d, want 3", len(all))
	}
	for i, want := range []int{1, 2, 3} {
		if all[i].Number != want {
			t.Fatalf("slot %d has number %d, want %d", i, all[i].Number, want)
		}
	}
	// Order is preserved, and nothing else about a slot changes.
	if all[1].Name != "ir" || all[1].Path != "/b" || all[1].Note != "landing" {
		t.Fatalf("slot 2 = %+v, want the old slot 4 intact", all[1])
	}
	// The moves are reported, because the numbers are muscle memory.
	for _, want := range []string{"4", "2", "9", "3"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output = %q, want it to mention %q", out.String(), want)
		}
	}
}

// A slot that does not move must not be listed as if it had.
func TestCompactOnlyReportsWhatMoved(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 3, Name: "c", Path: "/c"})

	if err := Run(env, cli.Command{Action: cli.ActionCompact}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	moves := 0
	for _, l := range lines {
		if strings.Contains(l, "->") {
			moves++
		}
	}
	if moves != 1 {
		t.Fatalf("reported %d moves, want 1 (only slot 3 moved):\n%s", moves, out.String())
	}
}

func TestCompactOnAnAlreadyCompactStore(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Path: "/a"})
	env.Store.Put(slots.Slot{Number: 2, Path: "/b"})

	if err := Run(env, cli.Command{Action: cli.ActionCompact}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "already") {
		t.Fatalf("output = %q, want it to say the store is already compact", out.String())
	}
	if env.Store.Len() != 2 {
		t.Fatalf("Len = %d, want 2", env.Store.Len())
	}
}

func TestCompactOnAnEmptyStore(t *testing.T) {
	env, out, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionCompact}); err != nil {
		t.Fatalf("compact on an empty store: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("compact on an empty store said nothing")
	}
}

// Renumbering must never merge two slots into one, which a naive in-place
// loop does as soon as a target number is still occupied.
func TestCompactNeverLosesASlot(t *testing.T) {
	env, _, _ := testEnv(t)
	for _, n := range []int{2, 3, 5, 8, 13, 21, 34} {
		env.Store.Put(slots.Slot{Number: n, Path: "/p"})
	}
	if err := Run(env, cli.Command{Action: cli.ActionCompact}); err != nil {
		t.Fatal(err)
	}
	if got := env.Store.Len(); got != 7 {
		t.Fatalf("Len = %d after compacting, want 7", got)
	}
	for i, sl := range env.Store.All() {
		if sl.Number != i+1 {
			t.Fatalf("slot at index %d has number %d, want %d", i, sl.Number, i+1)
		}
	}
}
