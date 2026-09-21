package tui

// keyHints is the footer line in list mode: every key the list keymap acts
// on, kept under 80 columns so that nothing — least of all `? help` — falls
// off the end of a standard terminal. The arrows carry no label because they
// need none, which is what buys the room. keys_test.go holds it to this.
const keyHints = "↑↓  ⏎ cd  / find  e edit  n name  o organize  a add  d del  c compact  ? help"

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
                   (on an empty row, into that slot)
    c              renumber every slot to close the gaps
    o              organize: space picks a slot up, move, space
                   drops it there and the rest shift to suit
    ?              this page
    q / ctrl-c     quit without moving

  press any key to go back`
