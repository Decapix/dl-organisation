// Package cli turns argv into a Command value. It knows nothing about the
// store, the filesystem, or rendering, which makes the whole grammar testable
// as a pure function.
package cli

import "fmt"

// Action is the single operation an invocation performs.
type Action int

const (
	ActionTUI     Action = iota // bare `dl`
	ActionCD                    // dl <ref>, dl -c <ref>
	ActionSet                   // dl -z [ref]
	ActionSee                   // dl -s
	ActionEdit                  // dl -e <ref>
	ActionReset                 // dl -r <ref>
	ActionDelete                // dl -d <ref>
	ActionPath                  // dl -p <ref>
	ActionRename                // dl -n <ref> <name>
	ActionSetNote               // dl -m <ref> <text>
	ActionDoctor                // dl doctor
	ActionInit                  // dl init <shell>
	ActionHelp                  // dl -h, dl <action> -h
	ActionVersion               // dl --version
)

// String is used by help lookup and by error messages.
func (a Action) String() string {
	switch a {
	case ActionTUI:
		return "tui"
	case ActionCD:
		return "--cd"
	case ActionSet:
		return "--set"
	case ActionSee:
		return "--see"
	case ActionEdit:
		return "--edit"
	case ActionReset:
		return "--reset"
	case ActionDelete:
		return "--delete"
	case ActionPath:
		return "--path"
	case ActionRename:
		return "--name"
	case ActionSetNote:
		return "--note"
	case ActionDoctor:
		return "doctor"
	case ActionInit:
		return "init"
	case ActionHelp:
		return "--help"
	case ActionVersion:
		return "--version"
	}
	return "unknown"
}

// Command is a fully parsed invocation.
type Command struct {
	Action Action
	Ref    string // slot reference, when the action takes one
	Name   string // -n value
	Note   string // -m value
	Edit   bool   // -e alongside -z
	Reset  bool   // -r alongside -z
	Long   bool   // -l on --see
	Quiet  bool   // -q on --see
	Shell  string // `init` argument

	// HelpFor is the action whose help to print when Action is ActionHelp.
	// ActionTUI means the global overview.
	HelpFor Action
}

// UsageError is a mistake in how the command was typed, as opposed to a
// runtime failure. main maps it to exit code 2 and prints the action's help
// pointer rather than the global help page.
type UsageError struct {
	Msg    string
	Action Action // the action the user was reaching for, for the help pointer
}

func (e *UsageError) Error() string { return e.Msg }

// usagef builds a UsageError tied to an action.
func usagef(a Action, format string, args ...any) *UsageError {
	return &UsageError{Msg: fmt.Sprintf(format, args...), Action: a}
}
