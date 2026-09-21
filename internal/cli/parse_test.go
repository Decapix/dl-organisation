package cli

import (
	"errors"
	"testing"
)

func TestParseTable(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want Command
	}{
		{"bare opens the TUI", nil, Command{Action: ActionTUI}},
		{"bare ref is a cd", []string{"7"}, Command{Action: ActionCD, Ref: "7"}},
		{"named ref is a cd", []string{"exam42"}, Command{Action: ActionCD, Ref: "exam42"}},
		{"explicit short cd", []string{"-c", "7"}, Command{Action: ActionCD, Ref: "7"}},
		{"explicit long cd", []string{"--cd", "7"}, Command{Action: ActionCD, Ref: "7"}},

		{"set with no ref", []string{"-z"}, Command{Action: ActionSet}},
		{"set with ref", []string{"-z", "7"}, Command{Action: ActionSet, Ref: "7"}},
		{"set long", []string{"--set", "7"}, Command{Action: ActionSet, Ref: "7"}},
		{"set with name", []string{"-z", "7", "-n", "exam42"},
			Command{Action: ActionSet, Ref: "7", Name: "exam42"}},
		{"set with note", []string{"-z", "7", "-m", "fix the loop"},
			Command{Action: ActionSet, Ref: "7", Note: "fix the loop"}},
		{"set with edit", []string{"-z", "7", "-e"},
			Command{Action: ActionSet, Ref: "7", Edit: true}},
		{"set with reset", []string{"-z", "7", "-r"},
			Command{Action: ActionSet, Ref: "7", Reset: true}},
		{"set with everything", []string{"-z", "7", "-r", "-n", "x", "-e"},
			Command{Action: ActionSet, Ref: "7", Reset: true, Name: "x", Edit: true}},
		{"flags may precede the ref", []string{"-z", "-n", "x", "7"},
			Command{Action: ActionSet, Ref: "7", Name: "x"}},
		{"long flag with equals", []string{"-z", "7", "--name=exam42"},
			Command{Action: ActionSet, Ref: "7", Name: "exam42"}},
		{"a note that looks like the ref is still the note",
			[]string{"-z", "7", "-m", "7"},
			Command{Action: ActionSet, Ref: "7", Note: "7"}},

		{"see", []string{"-s"}, Command{Action: ActionSee}},
		{"see long", []string{"--see"}, Command{Action: ActionSee}},
		{"see long form", []string{"-s", "-l"}, Command{Action: ActionSee, Long: true}},
		{"see quiet", []string{"-s", "-q"}, Command{Action: ActionSee, Quiet: true}},

		{"edit as an action", []string{"-e", "7"}, Command{Action: ActionEdit, Ref: "7"}},
		{"reset as an action", []string{"-r", "7"}, Command{Action: ActionReset, Ref: "7"}},
		{"delete", []string{"-d", "7"}, Command{Action: ActionDelete, Ref: "7"}},
		{"path", []string{"-p", "7"}, Command{Action: ActionPath, Ref: "7"}},
		{"rename as an action", []string{"-n", "7", "exam42"},
			Command{Action: ActionRename, Ref: "7", Name: "exam42"}},
		{"set note as an action", []string{"-m", "7", "fix the loop"},
			Command{Action: ActionSetNote, Ref: "7", Note: "fix the loop"}},

		{"doctor", []string{"doctor"}, Command{Action: ActionDoctor}},
		{"init", []string{"init", "zsh"}, Command{Action: ActionInit, Shell: "zsh"}},
		{"version", []string{"--version"}, Command{Action: ActionVersion}},

		{"global help", []string{"-h"}, Command{Action: ActionHelp, HelpFor: ActionTUI}},
		{"global help long", []string{"--help"}, Command{Action: ActionHelp, HelpFor: ActionTUI}},
		{"action help", []string{"-z", "-h"}, Command{Action: ActionHelp, HelpFor: ActionSet}},
		{"action help long", []string{"--cd", "--help"}, Command{Action: ActionHelp, HelpFor: ActionCD}},
		{"help before the action still works", []string{"-h", "-z"},
			Command{Action: ActionHelp, HelpFor: ActionSet}},
		{"init help", []string{"init", "-h"}, Command{Action: ActionHelp, HelpFor: ActionInit}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.argv)
			if err != nil {
				t.Fatalf("Parse(%q) returned an error: %v", c.argv, err)
			}
			if got != c.want {
				t.Fatalf("Parse(%q) =\n  %+v\nwant\n  %+v", c.argv, got, c.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"unknown short flag", []string{"-x"}},
		{"unknown long flag", []string{"--nope"}},
		{"two actions", []string{"-c", "-s", "7"}},
		{"two actions long", []string{"--set", "--delete", "7"}},
		{"cd without a ref", []string{"-c"}},
		{"delete without a ref", []string{"-d"}},
		{"path without a ref", []string{"-p"}},
		{"edit without a ref", []string{"-e"}},
		{"reset without a ref", []string{"-r"}},
		{"rename without a name", []string{"-n", "7"}},
		{"set note without text", []string{"-m", "7"}},
		{"too many positionals", []string{"7", "8"}},
		{"see takes no ref", []string{"-s", "7"}},
		{"long and quiet together", []string{"-s", "-l", "-q"}},
		{"name modifier without a value", []string{"-z", "7", "-n"}},
		{"note modifier without a value", []string{"-z", "7", "-m"}},
		{"init without a shell", []string{"init"}},
		{"init with an unknown shell", []string{"init", "csh"}},
		{"long form outside see", []string{"-z", "-l"}},
		{"quiet outside see", []string{"-c", "7", "-q"}},
		{"flag bundling is not supported", []string{"-sl"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.argv); err == nil {
				t.Fatalf("Parse(%q) = nil error, want a usage error", c.argv)
			}
		})
	}
}

// Every parse failure must be a *UsageError so main can exit 2 and point at
// the right help page.
func TestParseErrorsAreUsageErrors(t *testing.T) {
	_, err := Parse([]string{"-z", "-l"})
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("error type = %T, want *UsageError", err)
	}
	if ue.Action != ActionSet {
		t.Fatalf("UsageError.Action = %v, want %v", ue.Action, ActionSet)
	}
}
