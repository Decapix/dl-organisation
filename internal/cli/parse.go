package cli

import "strings"

// validShells are the shells `dl init` can emit an integration for.
var validShells = map[string]bool{"zsh": true, "bash": true, "fish": true}

// Parse turns argv (without the program name) into a Command.
//
// The grammar has two rules no flag library expresses well:
//
//  1. -n, -m, -e and -r are modifiers of -z; without -z each is an action of
//     its own. Whether -n consumes the next token therefore depends on a flag
//     that may appear after it, which is why argv is pre-scanned for -z.
//  2. A bare argument means --cd, so `dl 7` stays four keystrokes.
//
// Action flags never consume a value; references are always positionals. That
// makes `dl 7` and `dl -c 7` take the identical path through the parser.
func Parse(argv []string) (Command, error) {
	if len(argv) == 0 {
		return Command{Action: ActionTUI}, nil
	}

	// Subcommand words are recognised only in first position, so a slot named
	// "doctor" is still reachable as `dl -c doctor`.
	switch argv[0] {
	case "init":
		return parseInit(argv[1:])
	case "doctor":
		return parseDoctor(argv[1:])
	}

	// Rule 1's pre-scan.
	setMode := false
	for _, tok := range argv {
		if tok == "-z" || tok == "--set" {
			setMode = true
			break
		}
	}

	var (
		cmd         Command
		positionals []string
		help        bool
		// which flags were seen
		cd, set, see, edit, reset, del, path, about, rename, note bool
		compact, move, long, quiet, version                       bool
	)

	// An explicit index rather than a range loop: value-taking flags advance
	// it themselves to swallow their operand.
	i := 0
	for i < len(argv) {
		tok := argv[i]

		// A lone "-" or anything not starting with "-" is a positional.
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			positionals = append(positionals, tok)
			i++
			continue
		}

		// Split --flag=value once, so --name=x and --name x are equivalent.
		flag, inline := tok, ""
		hasInline := false
		if eq := strings.Index(tok, "="); eq > 0 && strings.HasPrefix(tok, "--") {
			flag, inline, hasInline = tok[:eq], tok[eq+1:], true
		}

		// value returns the operand of a value-taking flag, from either
		// --flag=value or the following token, advancing i in the latter case.
		value := func() (string, bool) {
			if hasInline {
				return inline, true
			}
			if i+1 >= len(argv) {
				return "", false
			}
			i++
			return argv[i], true
		}

		switch flag {
		case "-h", "--help":
			help = true
		case "--version":
			version = true
		case "-c", "--cd":
			cd = true
		case "-z", "--set":
			set = true
		case "-s", "--see":
			see = true
		case "--compact":
			compact = true
		case "--move":
			move = true
		case "-d", "--delete":
			del = true
		case "-p", "--path":
			path = true
		case "-a", "--about":
			about = true
		case "-l", "--long":
			long = true
		case "-q", "--quiet":
			quiet = true
		case "-e", "--edit":
			edit = true
		case "-r", "--reset":
			reset = true

		case "-n", "--name":
			rename = true
			// As a modifier of -z the name is this flag's operand; as a
			// standalone action it is a positional, so that `dl -n 7 exam42`
			// reads left to right.
			if setMode {
				v, ok := value()
				if !ok {
					return Command{}, usagef(ActionSet, "%s needs a value", flag)
				}
				cmd.Name = v
			} else if hasInline {
				return Command{}, usagef(ActionRename, "use: dl -n <ref> <name>")
			}
		case "-m", "--note":
			note = true
			if setMode {
				v, ok := value()
				if !ok {
					return Command{}, usagef(ActionSet, "%s needs a value", flag)
				}
				cmd.Note = v
			} else if hasInline {
				return Command{}, usagef(ActionSetNote, "use: dl -m <ref> <text>")
			}

		default:
			return Command{}, usagef(ActionTUI, "unknown option %q", tok)
		}

		// Only -n and -m take a value; anything else with "=" is a mistake.
		if hasInline && flag != "-n" && flag != "--name" && flag != "-m" && flag != "--note" {
			return Command{}, usagef(ActionTUI, "%s does not take a value", flag)
		}
		i++
	}

	// Decide the action. -e, -r, -n and -m count as actions only when -z is
	// absent.
	actions := 0
	countIf := func(b bool, a Action) {
		if b {
			actions++
			cmd.Action = a
		}
	}
	countIf(cd, ActionCD)
	countIf(set, ActionSet)
	countIf(see, ActionSee)
	countIf(del, ActionDelete)
	countIf(path, ActionPath)
	countIf(about, ActionAbout)
	countIf(compact, ActionCompact)
	countIf(move, ActionMove)
	if !set {
		countIf(edit, ActionEdit)
		countIf(reset, ActionReset)
		countIf(rename, ActionRename)
		countIf(note, ActionSetNote)
	}

	if actions > 1 {
		return Command{}, usagef(ActionTUI, "only one action per command")
	}
	if actions == 0 {
		switch {
		case version:
			return Command{Action: ActionVersion}, nil
		case help:
			return Command{Action: ActionHelp, HelpFor: ActionTUI}, nil
		case len(positionals) > 0:
			cmd.Action = ActionCD // rule 2: a bare argument means cd
		default:
			cmd.Action = ActionTUI
		}
	}

	// -h anywhere short-circuits into that action's help page.
	if help {
		return Command{Action: ActionHelp, HelpFor: cmd.Action}, nil
	}

	if set {
		cmd.Edit, cmd.Reset = edit, reset
	}
	cmd.Long, cmd.Quiet = long, quiet

	if err := validate(&cmd, positionals, long, quiet); err != nil {
		return Command{}, err
	}
	return cmd, nil
}

// validate checks the positional count and the flag combinations for the
// chosen action, and moves the positionals into the Command.
func validate(cmd *Command, pos []string, long, quiet bool) error {
	// -l and -q belong to --see only, and are mutually exclusive there.
	if (long || quiet) && cmd.Action != ActionSee {
		return usagef(cmd.Action, "-l and -q only apply to --see")
	}
	if long && quiet {
		return usagef(ActionSee, "-l and -q cannot be combined")
	}

	want := func(n int, what string) error {
		if len(pos) != n {
			return usagef(cmd.Action, "%s takes %s", cmd.Action, what)
		}
		return nil
	}

	switch cmd.Action {
	case ActionCD, ActionAbout, ActionEdit, ActionReset, ActionDelete, ActionPath:
		if err := want(1, "exactly one slot reference"); err != nil {
			return err
		}
		cmd.Ref = pos[0]
	case ActionSet:
		if len(pos) > 1 {
			return usagef(ActionSet, "--set takes at most one slot reference")
		}
		if len(pos) == 1 {
			cmd.Ref = pos[0]
		}
	case ActionRename:
		if err := want(2, "a slot reference and a name"); err != nil {
			return err
		}
		cmd.Ref, cmd.Name = pos[0], pos[1]
	case ActionSetNote:
		if err := want(2, "a slot reference and a note"); err != nil {
			return err
		}
		cmd.Ref, cmd.Note = pos[0], pos[1]
	case ActionMove:
		if err := want(2, "a slot reference and a target number"); err != nil {
			return err
		}
		cmd.Ref, cmd.To = pos[0], pos[1]
	case ActionSee, ActionTUI, ActionCompact:
		if err := want(0, "no arguments"); err != nil {
			return err
		}
	}
	return nil
}

// parseInit handles `dl init <shell>`.
func parseInit(rest []string) (Command, error) {
	for _, tok := range rest {
		if tok == "-h" || tok == "--help" {
			return Command{Action: ActionHelp, HelpFor: ActionInit}, nil
		}
	}
	if len(rest) != 1 {
		return Command{}, usagef(ActionInit, "init takes exactly one shell name (zsh, bash, fish)")
	}
	if !validShells[rest[0]] {
		return Command{}, usagef(ActionInit, "unknown shell %q; supported: zsh, bash, fish", rest[0])
	}
	return Command{Action: ActionInit, Shell: rest[0]}, nil
}

// parseDoctor handles `dl doctor`.
func parseDoctor(rest []string) (Command, error) {
	for _, tok := range rest {
		if tok == "-h" || tok == "--help" {
			return Command{Action: ActionHelp, HelpFor: ActionDoctor}, nil
		}
	}
	if len(rest) != 0 {
		return Command{}, usagef(ActionDoctor, "doctor takes no arguments")
	}
	return Command{Action: ActionDoctor}, nil
}
