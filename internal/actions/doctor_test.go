package actions

import (
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestDoctorReportsOnlyDeadPaths(t *testing.T) {
	env, out, _ := testEnv(t)
	const alive = "/home/u/alive"
	env.Exists = func(p string) bool { return p == alive }
	env.Store.Put(slots.Slot{Number: 1, Name: "alive", Path: alive})
	env.Store.Put(slots.Slot{Number: 2, Name: "dead", Path: "/definitely/not/here"})

	if err := Run(env, cli.Command{Action: cli.ActionDoctor}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "/definitely/not/here") {
		t.Errorf("output = %q, want the dead path", got)
	}
	if strings.Contains(got, alive) {
		t.Errorf("output = %q, must not list the live path", got)
	}
	// The report must say how to fix each one.
	if !strings.Contains(got, "dl -z 2") || !strings.Contains(got, "dl -d 2") {
		t.Errorf("output = %q, want repair hints", got)
	}
}

func TestDoctorOnAHealthyStore(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Exists = func(string) bool { return true }
	env.Store.Put(slots.Slot{Number: 1, Path: "/home/u/a"})
	if err := Run(env, cli.Command{Action: cli.ActionDoctor}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "every slot") {
		t.Fatalf("output = %q, want an all-clear message", out.String())
	}
}

// doctor only reports; it must never modify the store.
func TestDoctorChangesNothing(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Exists = func(string) bool { return false }
	env.Store.Put(slots.Slot{Number: 2, Path: "/gone"})
	if err := Run(env, cli.Command{Action: cli.ActionDoctor}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Store.Get(2); !ok {
		t.Fatal("doctor removed a slot; it must only report")
	}
}
