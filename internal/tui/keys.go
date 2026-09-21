package tui

// keyHints is the footer line in list mode. It lists only what a new user
// needs; `?` opens the full overlay.
const keyHints = "↑↓ move  ⏎ cd  / find  e edit  n name  a add  d del  ? help"

// helpOverlay is the full keymap, shown on `?`.
const helpOverlay = `  MOVE
    ↑ ↓ / j k      move the cursor
    g / G          first / last slot
    ctrl-u ctrl-d  scroll the note

  ACT ON THE SELECTED SLOT
    ⏎              cd into it and quit
    e              edit its note in $EDITOR
    n              rename it
    r              clear its name and note
    d              delete it

  OTHER
    /              filter by name, path or note
    a              save the current directory to a free slot
    ?              this page
    q / ctrl-c     quit without moving

  press any key to go back`
