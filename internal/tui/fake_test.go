package tui

import (
	"fmt"
	"io"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// fakeSession records the commands the model issues and serves a fixed slot
// list, so every interaction test runs without a store or a filesystem.
type fakeSession struct {
	slots []slots.Slot
	// ran is every command Run was called with, in order.
	ran []cli.Command
	// err, when non-nil, is what Run returns.
	err error
	// output is what Run writes.
	output string
	// slotsErr, when non-nil, is what Slots returns.
	slotsErr error
	// home overrides HomeDir; empty means the default fixture home.
	home string
}

func (f *fakeSession) Slots() ([]slots.Slot, error) {
	if f.slotsErr != nil {
		return nil, f.slotsErr
	}
	return f.slots, nil
}

func (f *fakeSession) Run(cmd cli.Command, out io.Writer) error {
	f.ran = append(f.ran, cmd)
	if f.err != nil {
		return f.err
	}
	if f.output != "" {
		fmt.Fprint(out, f.output)
	}
	return nil
}

func (f *fakeSession) CurrentDir() string { return "/home/u/work" }
func (f *fakeSession) HomeDir() string {
	if f.home != "" {
		return f.home
	}
	return "/home/u"
}

// lastCommand is the most recent command the model issued.
func (f *fakeSession) lastCommand() cli.Command {
	if len(f.ran) == 0 {
		return cli.Command{}
	}
	return f.ran[len(f.ran)-1]
}

// threeSlots is the fixture most tests use: one with a note, one plain and
// unnamed, one named. The numbers are consecutive so that the row count
// equals the slot count; gappySlots in rows_test.go covers the holes.
func threeSlots() []slots.Slot {
	return []slots.Slot{
		{Number: 1, Name: "alpha", Path: "/home/u/alpha", Note: "first note\nsecond line"},
		{Number: 2, Path: "/home/u/work/beta"},
		{Number: 3, Name: "gamma", Path: "/home/u/gamma", Note: "gamma note"},
	}
}
