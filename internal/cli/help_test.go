package cli

import (
	"strings"
	"testing"
)

// Every action a user can type must have its own help page. This test is the
// guard that stops a new action shipping without documentation.
func TestEveryActionHasHelp(t *testing.T) {
	actions := []Action{
		ActionTUI, ActionCD, ActionSet, ActionSee, ActionEdit, ActionReset,
		ActionDelete, ActionPath, ActionRename, ActionSetNote, ActionMove,
		ActionCompact, ActionDoctor, ActionInit,
	}
	for _, a := range actions {
		got := Help(a)
		if strings.TrimSpace(got) == "" {
			t.Errorf("Help(%v) is empty", a)
		}
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("Help(%v) does not end with a newline", a)
		}
	}
}

func TestGlobalHelpListsEveryAction(t *testing.T) {
	got := Help(ActionTUI)
	for _, want := range []string{
		"--cd", "--set", "--see", "--edit", "--reset", "--delete", "--path",
		"--name", "--note", "--move", "--compact", "doctor", "init", "--help", "--version",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("global help does not mention %q", want)
		}
	}
}

func TestActionHelpHasOptionsAndExamples(t *testing.T) {
	got := Help(ActionSet)
	for _, want := range []string{"OPTIONS", "EXAMPLES", "-n, --name", "-r, --reset", "dl -z 7"} {
		if !strings.Contains(got, want) {
			t.Errorf("--set help does not contain %q", want)
		}
	}
}

// An unknown action must not panic; it falls back to the overview.
func TestHelpFallsBackToTheOverview(t *testing.T) {
	if Help(Action(999)) != Help(ActionTUI) {
		t.Error("Help for an unknown action should fall back to the global overview")
	}
}
