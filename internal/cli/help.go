package cli

// Help returns the help page for an action. ActionTUI means the global
// overview, which is deliberately short: it is a page you re-read, not a
// manual you read once.
func Help(a Action) string {
	if page, ok := helpPages[a]; ok {
		return page
	}
	return helpPages[ActionTUI]
}

var helpPages = map[Action]string{
	ActionTUI: `dl — directory bookmarks with notes

USAGE
  dl                      open the interactive browser
  dl <ref>                cd into a slot          ref: 7 | exam42 | ex
  dl -z [ref]             save the current directory
  dl -s                   list every slot

ACTIONS
  -c, --cd     <ref>      cd into a slot
  -z, --set    [ref]      save $PWD into a slot (default: lowest free slot)
  -s, --see               list every slot
  -e, --edit   <ref>      edit a slot's note in $EDITOR
  -r, --reset  <ref>      clear a slot's name and note, keep the path
  -d, --delete <ref>      remove a slot entirely
  -p, --path   <ref>      print a slot's path only
  -n, --name   <ref> <name>   rename a slot
  -m, --note   <ref> <text>   replace a slot's note inline

OTHER
  doctor                  report slots whose path is gone
  init <shell>            print the shell integration (zsh, bash, fish)
  -h, --help              this page; -h after an action for its details
      --version

Run "dl -z --help" (or -c, -s, -e ...) for an action's own options.
`,

	ActionCD: `dl -c, --cd <ref> — cd into a slot

  <ref> is a slot number, a slot name, or a unique prefix of a name.
  The bare form "dl <ref>" does the same thing and is shorter.
  Arriving at a slot prints its note, so you land on where you left off.

  This needs the shell integration to actually move your shell:
  add   eval "$(dl init zsh)"   to your shell rc file.

EXAMPLES
  dl 7                    by number
  dl exam42               by name
  dl ex                   by unique prefix
  dl -c 7                 explicit, for scripts
`,

	ActionSet: `dl -z, --set [ref] — save the current directory into a slot

  ref is optional: with no ref, dl picks the lowest free slot.
  An occupied slot is overwritten directly. The name and note are
  kept unless you pass -r.

OPTIONS
  -n, --name <name>   set the slot's short name
  -m, --note <text>   set the note inline (replaces the current one)
  -e, --edit          open $EDITOR on the note after saving
  -r, --reset         clear the name and the note

EXAMPLES
  dl -z                          save here, lowest free slot
  dl -z 7                        save here into slot 7
  dl -z 7 -n exam42              save and name it
  dl -z 7 -n exam42 -e           save, name it, then write the note
  dl -z 7 -m "waiting on the API fix"
  dl -z 7 -r -n scraping         reuse slot 7 for something else
`,

	ActionSee: `dl -s, --see — list every slot

  Slots are listed in number order. A leading marker shows the state:
    *   the slot has a note
    x   the path no longer exists

OPTIONS
  -l, --long    print each note in full under its slot
  -q, --quiet   print paths only, one per line, for piping

  -l and -q cannot be combined.

EXAMPLES
  dl -s
  dl -s -l
  dl -s -q | fzf
`,

	ActionEdit: `dl -e, --edit <ref> — edit a slot's note

  Opens the note in $DL_EDITOR, $VISUAL, $EDITOR, or vi, in that order.
  If the editor exits non-zero the stored note is left untouched.

EXAMPLES
  dl -e 7
  dl -e exam42
`,

	ActionReset: `dl -r, --reset <ref> — clear a slot's name and note

  The path is kept. To clear the whole slot use --delete.
  As a modifier of --set, -r saves the current directory and starts clean.

EXAMPLES
  dl -r 7                 wipe slot 7's name and note
  dl -z 7 -r              save here into slot 7, starting clean
`,

	ActionDelete: `dl -d, --delete <ref> — remove a slot entirely

  Path, name and note all go. This is the only command that removes a
  slot; --set never does.

EXAMPLES
  dl -d 7
  dl -d exam42
`,

	ActionPath: `dl -p, --path <ref> — print a slot's path

  Prints the path and nothing else, so it composes with other commands.

EXAMPLES
  dl -p 7
  cp report.pdf "$(dl -p 7)"
  ls "$(dl -p exam42)"
`,

	ActionRename: `dl -n, --name <ref> <name> — rename a slot

  The name is a second way to address a slot: once slot 7 is named
  exam42, "dl exam42" and "dl ex" both reach it.
  Pass an empty name to remove it: dl -n 7 ""

EXAMPLES
  dl -n 7 exam42
  dl -n 12 scraping
  dl -n 7 ""              drop the name
`,

	ActionSetNote: `dl -m, --note <ref> <text> — replace a slot's note

  The text replaces the whole note. For multi-line notes use --edit.

EXAMPLES
  dl -m 7 "waiting on the API fix"
  dl -m exam42 ""         clear the note
`,

	ActionDoctor: `dl doctor — report slots whose path is gone

  Lists every slot pointing at a directory that no longer exists, so you
  can repoint it with --set or drop it with --delete. It changes nothing
  on its own.

EXAMPLES
  dl doctor
`,

	ActionInit: `dl init <shell> — print the shell integration

  A program cannot change its parent shell's directory, so dl ships a
  small function that does the cd for you. Add this to your rc file:

    zsh    eval "$(dl init zsh)"      in ~/.zshrc
    bash   eval "$(dl init bash)"     in ~/.bashrc
    fish   dl init fish | source      in ~/.config/fish/config.fish

  Without it, "dl 7" prints the path instead of moving you.
`,
}
