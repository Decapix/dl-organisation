# dl

Directory bookmarks with notes. `dl` saves a directory into a numbered slot,
lets you jump back with four keystrokes, and — the part that makes it more than
a bookmark manager — prints the note you left there when you arrive.

```
$ cd ~/work/scraper
$ dl -z 12 -n scraping -m "pagination stops at p.4, fix the retry loop"
slot 12 -> ~/work/scraper

$ dl -s
1 slot

  * 12  scraping  ~/work/scraper

$ cd /
$ dl scraping
pagination stops at p.4, fix the retry loop
$ pwd
/home/you/work/scraper
```

## Install

```sh
go install github.com/Decapix/dl-organisation/cmd/dl@latest
```

Then add the shell integration, which is what lets `dl` actually move your
shell:

| shell | line to add | file |
|-------|-------------|------|
| zsh | `eval "$(dl init zsh)"` | `~/.zshrc` |
| bash | `eval "$(dl init bash)"` | `~/.bashrc` |
| fish | `dl init fish \| source` | `~/.config/fish/config.fish` |

Without it `dl 7` prints the path instead of taking you there.

## Usage

```
dl — directory bookmarks with notes

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
```

Every action has its own help page with its options and real examples:
`dl -z --help`, `dl -s --help`, and so on.

## Slots, names and notes

A slot holds three things:

- **a path**, the directory it points at
- **a name**, optional and short, which is also a way to address the slot:
  once slot 7 is named `exam42`, `dl 7`, `dl exam42` and `dl ex` all reach it.
  An unnamed slot displays as its directory's base name, so no row is ever
  unlabelled.
- **a note**, free-form and multi-line. It is printed when you jump in, which
  is the point: you come back to a project three weeks later and the first
  thing you read is where you stopped.

`dl -s` marks a slot with `*` when it has a note and `x` when its directory is
gone. `dl -s -l` prints the notes in full; `dl doctor` lists only the broken
ones.

Nothing is ever destroyed implicitly. Saving over an occupied slot replaces the
path and **keeps** the name and the note — you have to type `-r` to clear them
or `-d` to remove the slot.

## Coming from stl / cdl / seedl

| old | new |
|-----|-----|
| `cdl7` | `dl 7` |
| `stl7` | `dl -z 7` |
| `stl7 -c "text"` | `dl -z 7 -m "text"` |
| `stl7 -d` | `dl -z 7 -e` |
| `cdl7 -d` | `dl -e 7` |
| `cdl7 -sd` | `dl -s -l` |
| `seedl` | `dl -s` |

The first run imports `~/.cdl` automatically: each slot keeps its number and
path, and the old comment and `descriptions/<N>.txt` file are merged into one
note. The old directory is renamed to `~/.cdl.bak` rather than deleted.

## How the cd works

A program cannot change its parent shell's directory. `dl init <shell>` emits a
small function that runs the binary with `$DL_CD_FILE` pointing at a temp file;
the binary writes the target path there, and the function performs the `cd`.

The temp file is used instead of capturing stdout — the approach zoxide takes —
because capturing stdout would break the interactive browser, which needs the
real terminal.

## Storage

One file, `~/.config/dl/slots.json`, notes included. Set `$DL_DIR` to move it.
Writes are atomic (temp file plus rename) and serialised with an `flock`, so
two shells saving at the same time cannot clobber each other.

## Licence

MIT.
