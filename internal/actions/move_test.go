package actions

import (
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// names returns the slot names in number order, which is the ordering a move
// rearranges.
func names(env *Env) []string {
	var out []string
	for _, sl := range env.Store.All() {
		out = append(out, sl.Name)
	}
	return out
}

func numbers(env *Env) []int {
	var out []int
	for _, sl := range env.Store.All() {
		out = append(out, sl.Number)
	}
	return out
}

func fourSlots(env *Env) {
	for i, name := range []string{"a", "b", "c", "d"} {
		env.Store.Put(slots.Slot{Number: i + 1, Name: name, Path: "/" + name})
	}
}

func TestMoveUpShiftsTheOthersDown(t *testing.T) {
	env, out, _ := testEnv(t)
	fourSlots(env)

	// d is at 4; put it at 2.
	err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "4", To: "2"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "d", "b", "c"}
	if got := names(env); !equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if !strings.Contains(out.String(), "4 -> 2") {
		t.Errorf("output = %q, want the move reported", out.String())
	}
}

func TestMoveDownShiftsTheOthersUp(t *testing.T) {
	env, _, _ := testEnv(t)
	fourSlots(env)

	// a is at 1; put it at 3.
	if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "1", To: "3"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"b", "c", "a", "d"}
	if got := names(env); !equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// A move is a permutation over the numbers already in use: it must not close
// gaps behind your back. --compact is the command for that.
func TestMovePreservesTheOccupiedNumbers(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 4, Name: "b", Path: "/b"})
	env.Store.Put(slots.Slot{Number: 9, Name: "c", Path: "/c"})

	if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "9", To: "4"}); err != nil {
		t.Fatal(err)
	}
	if got := numbers(env); !equalInts(got, []int{1, 4, 9}) {
		t.Fatalf("numbers = %v, want them unchanged", got)
	}
	if got := names(env); !equal(got, []string{"a", "c", "b"}) {
		t.Fatalf("order = %v, want a, c, b", got)
	}
}

// Dropping onto a free number is the simple case: the slot takes it and
// nothing else moves.
func TestMoveOntoAFreeNumberJustRenumbers(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 4, Name: "b", Path: "/b"})
	env.Store.Put(slots.Slot{Number: 9, Name: "c", Path: "/c"})

	if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "9", To: "5"}); err != nil {
		t.Fatal(err)
	}
	if got := numbers(env); !equalInts(got, []int{1, 4, 5}) {
		t.Fatalf("numbers = %v, want 1, 4, 5", got)
	}
	if got := names(env); !equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("order = %v, want a, b, c", got)
	}
}

func TestMoveToItsOwnNumberChangesNothing(t *testing.T) {
	env, out, _ := testEnv(t)
	fourSlots(env)
	if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "2", To: "2"}); err != nil {
		t.Fatal(err)
	}
	if got := names(env); !equal(got, []string{"a", "b", "c", "d"}) {
		t.Fatalf("order = %v, want it unchanged", got)
	}
	if !strings.Contains(out.String(), "already") {
		t.Errorf("output = %q, want it to say nothing moved", out.String())
	}
}

// Everything a slot holds rides along with it.
func TestMoveCarriesTheNameAndNote(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 2, Name: "b", Path: "/b", Note: "important"})

	if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "2", To: "1"}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(1)
	if got.Name != "b" || got.Path != "/b" || got.Note != "important" {
		t.Fatalf("slot 1 = %+v, want the old slot 2 intact", got)
	}
}

func TestMoveRejectsABadTarget(t *testing.T) {
	env, _, _ := testEnv(t)
	fourSlots(env)
	for _, to := range []string{"0", "1001", "abc", ""} {
		if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "1", To: to}); err == nil {
			t.Errorf("target %q was accepted", to)
		}
	}
}

func TestMoveRejectsAnUnknownSource(t *testing.T) {
	env, _, _ := testEnv(t)
	fourSlots(env)
	if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: "99", To: "1"}); err == nil {
		t.Fatal("moving an empty slot was accepted")
	}
}

// No slot may be lost or duplicated, whatever the move.
func TestMoveNeverLosesASlot(t *testing.T) {
	for _, pair := range [][2]string{{"1", "4"}, {"4", "1"}, {"2", "3"}, {"3", "2"}, {"1", "2"}} {
		env, _, _ := testEnv(t)
		fourSlots(env)
		if err := Run(env, cli.Command{Action: cli.ActionMove, Ref: pair[0], To: pair[1]}); err != nil {
			t.Fatalf("move %v: %v", pair, err)
		}
		if got := env.Store.Len(); got != 4 {
			t.Fatalf("move %v left %d slots, want 4", pair, got)
		}
		seen := map[string]bool{}
		for _, sl := range env.Store.All() {
			if seen[sl.Name] {
				t.Fatalf("move %v duplicated %q", pair, sl.Name)
			}
			seen[sl.Name] = true
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
