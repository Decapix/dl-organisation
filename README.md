# dl

Directory bookmarks with notes. `dl` saves a directory into a numbered slot,
lets you jump back with four keystrokes, and — the part that makes it more than
a bookmark manager — prints the note you left there when you arrive.

```
$ dl                                 # the browser
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

**Put it after your `PATH` setup**, near the end of the file. The line runs the
`dl` binary, so if it comes before the export that puts `go install`'s output
directory on your `PATH`, your shell reports `command not found: dl` at startup
and the integration is silently never installed.

Without the integration, `dl 7` prints the path and a hint instead of taking
you there.

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
      --move <ref> <to>   move a slot to another number
      --compact           renumber the slots to close the gaps
  doctor                  report slots whose path is gone
  init <shell>            print the shell integration (zsh, bash, fish)
  -h, --help              this page; -h after an action for its details
      --version
```

Every action has its own help page with its options and real examples:
`dl -z --help`, `dl -s --help`, and so on.

## The browser

`dl` with no arguments opens an interactive browser: the list on the left, the
selected slot's note in full on the right.

```
 dl                                                          5 slots

    1  coloriage    ~/…/coloriage/svg          │ slot 3 is empty
  * 2  cpp08        ~/…/code/cpp/cpp08         │
 ▸  3  empty                                   │ press a to save the
  * 4  ir-landing   ~/…/new-landing-ir         │ current directory here
    ⋯  5–6 empty                               │
  * 7  exam42       ~/…/exam5/s4/level1        │
   40  far          ~/…/somewhere/far          │

 ↑↓  ⏎ cd  / find  e edit  n name  o organize  a add  d del  c compact  ? help
```

Deleted slots stay on screen as empty rows, because the numbers are the
interface: seeing that 3 is free is what tells you the number is available.
Press `a` on one to save the current directory into exactly that slot. A run
of three or more free numbers collapses to one row, so a store using slots 1
and 500 does not become 498 blank lines. The list stops at the last occupied
slot.

| Key | Action |
|-----|--------|
| `↑` `↓` `j` `k` `g` `G` | move |
| `⏎` | cd into the slot and quit |
| `/` | filter by name, path **or note text** |
| `e` | edit the note in `$EDITOR` |
| `n` | rename in place |
| `a` | save the current directory to the lowest free slot |
| `d` `r` | delete / clear name and note — both ask first |
| `c` | renumber every slot to close the gaps — asks first |
| `o` | organize: space picks a slot up, move, space drops it there |
| `ctrl-d` `ctrl-u` | scroll a long note |
| `?` | every key |
| `q` `ctrl-c` | quit without moving |

Paths are elided in the middle, keeping the end, because the end is what tells
you which project a row is. Below 80 columns the note moves under the list;
below 60 it is dropped. A browser left open holds no lock, so `dl -z` in
another shell never waits on it.

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

## Reordering

Press `o` in the browser, then space to pick the slot under the cursor up. Move
wherever you like — the held slot is marked `↕` and the footer says what you
are carrying — and press space again to put it down there.

```
    1  coloriage   ~/work/coloriage-app/svg
  * 2  cpp08       ~/42/code/cpp/cpp08
    3  ir-landing  ~/style-site/new-landing-ir
 ▸↕ 4  exam42      ~/42/exams/exam5/s4/level1

 moving slot 4 — space drops it here, esc cancels
```

Dropping it on slot 2 makes it slot 2 and shifts 2 and 3 down by one. Dropping
it on a free number just gives it that number. Either way the set of numbers in
use is unchanged: a move rearranges, it never closes gaps behind your back.

From the command line the same thing is `dl --move <ref> <to>`:

```
$ dl --move 4 2
4 -> 2
2 -> 3
3 -> 4
```

## Closing the gaps

Deleting slots leaves holes: `1, 4, 9`. `dl --compact` slides everything down
to `1, 2, 3`, in the same order, carrying every name and note across. It
prints what moved, because the numbers are what you type:

```
$ dl --compact
compacted 3 slots
  4 -> 2
  9 -> 3
```

`c` does the same from the browser.

## Coming from stl / cdl / seedl

| old | new |
|-----|-----|
| `cdl7` | `dl 7` |
| `stl7` | `dl -z 7` |
| `stl7 -c "text"` | `dl -z 7 -m "text"` |
| `stl7 -d` | `dl -z 7 -e` |
| `cdl7 -d` | `dl -e 7` |
| `cdl7 -sd` | `dl -s -l` |
| `seedl` | `dl -s` or `dl` |

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
