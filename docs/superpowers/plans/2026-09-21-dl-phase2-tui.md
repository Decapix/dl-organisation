# dl Phase 2 (TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `dl` → `--see` fallback with the two-pane interactive browser from the spec: list on the left, full note on the right, with jump, filter, edit, rename, add, reset and delete all reachable from one keystroke.

**Architecture:** The TUI owns no behaviour. Every key that changes something builds a `cli.Command` and hands it to the same `actions.Run` the command line uses. A new `actions.Session` owns the store's lifecycle — read without a lock for display, lock taken and released per mutation — so a browser left open for ten minutes never blocks another shell. The bubbletea `Model.Update` is a pure function of state and message, which is what makes the whole interaction testable without a terminal.

**Tech Stack:** bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0.

**Spec:** `docs/superpowers/specs/2026-09-21-dl-design.md` section 9.
**Builds on:** `docs/superpowers/plans/2026-09-21-dl-phase1-cli.md`.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/actions/session.go` | store lifecycle: read for display, lock per mutation |
| `cmd/dl/main.go` (modify) | build a Session, drop the inline store handling |
| `internal/tui/tui.go` | `Run(Session) error`: program lifecycle, final output |
| `internal/tui/model.go` | `Model`, `Init`, `Update` — all state transitions |
| `internal/tui/view.go` | `View` and the layout modes |
| `internal/tui/keys.go` | the keymap and the help overlay text |
| `internal/tui/style.go` | lipgloss styles, adaptive for light and dark |
| `internal/tui/filter.go` | the matcher, so it can be swapped for fuzzy later |
| `internal/tui/exec.go` | the `tea.Exec` adapter that runs `$EDITOR` |
| `internal/tui/fake_test.go` | the fake Session every model test runs against |

---

### Task 1: actions.Session

The TUI must not hold the write lock between keystrokes, and `main` should not
be opening stores by hand. One type answers both.

**Files:**
- Create: `internal/actions/session.go`
- Test: `internal/actions/session_test.go`
- Modify: `cmd/dl/main.go`

- [ ] **Step 1: Write the failing test**

```go
package actions

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func testSession(t *testing.T) *Session {
	t.Helper()
	home := t.TempDir()
	return &Session{
		Dir:    filepath.Join(home, "store"),
		Home:   home,
		Cwd:    filepath.Join(home, "work"),
		Editor: &editor.Fake{},
		Now:    func() time.Time { return time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) },
		Err:    &bytes.Buffer{},
	}
}

func TestSessionRunPersistsBetweenCalls(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "7"}, &out); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.Slots()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Number != 7 {
		t.Fatalf("Slots() = %+v, want slot 7", got)
	}
}

// The point of the type: no lock is held between calls, so a second session
// (a shell running dl while the browser is open) can write.
func TestSessionHoldsNoLockBetweenCalls(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "1"}, &out); err != nil {
		t.Fatal(err)
	}
	// If Run leaked the lock this would block forever; the test would time out.
	other, err := slots.OpenForUpdate(s.Dir)
	if err != nil {
		t.Fatalf("a second writer could not take the lock: %v", err)
	}
	other.Close()
}

// Slots() must see what another process wrote, so the browser picks up a save
// made in another shell on its next reload.
func TestSlotsRereadsFromDisk(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	s.Run(cli.Command{Action: cli.ActionSet, Ref: "1"}, &out)

	other, err := slots.OpenForUpdate(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	other.Put(slots.Slot{Number: 2, Path: "/elsewhere"})
	other.Save()
	other.Close()

	got, _ := s.Slots()
	if len(got) != 2 {
		t.Fatalf("Slots() = %d entries, want 2 (an external write was missed)", len(got))
	}
}

func TestSessionRunReportsActionOutput(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionSet, Ref: "7"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("Run wrote nothing; the caller needs the action's output")
	}
}

func TestSessionRunPropagatesErrors(t *testing.T) {
	s := testSession(t)
	var out bytes.Buffer
	if err := s.Run(cli.Command{Action: cli.ActionCD, Ref: "999"}, &out); err == nil {
		t.Fatal("cd into an empty slot = nil error, want an error")
	}
}

// Slots() on a store that was never written is empty, not an error.
func TestSlotsOnAFreshStore(t *testing.T) {
	s := testSession(t)
	got, err := s.Slots()
	if err != nil {
		t.Fatalf("Slots() on a fresh store: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Slots() = %v, want empty", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run TestSession -v`
Expected: build failure, `undefined: Session`

- [ ] **Step 3: Write `internal/actions/session.go`**

```go
package actions

import (
	"io"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Session owns the store's lifecycle so that no caller has to.
//
// It opens the store for the duration of one command and closes it again. A
// one-shot CLI invocation could just as well hold it open, but the browser
// runs for minutes at a time, and an exclusive flock held across keystrokes
// would block every other shell. Both callers therefore go through the same
// type, and there is one place that decides when the lock is taken.
type Session struct {
	Dir  string // the store directory
	Home string // for shortening paths in output
	Cwd  string // what --set records

	// CDFile is $DL_CD_FILE: the file the shell wrapper reads to perform the
	// cd. Empty means the integration is not installed.
	CDFile string

	Editor editor.Editor
	Now    func() time.Time
	Err    io.Writer // warnings and hints
}

// Slots reads the store without taking the write lock and returns every slot
// in number order. It re-reads each time, so a save made in another shell
// shows up on the next call.
func (s *Session) Slots() ([]slots.Slot, error) {
	store, err := slots.Open(s.Dir)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.All(), nil
}

// Run executes one command, writing the action's output to out.
//
// The store is opened for update only when the action writes, so listing and
// jumping never wait behind an edit in progress elsewhere.
func (s *Session) Run(cmd cli.Command, out io.Writer) error {
	var (
		store *slots.Store
		err   error
	)
	if writesStore(cmd.Action) {
		store, err = slots.OpenForUpdate(s.Dir)
	} else {
		store, err = slots.Open(s.Dir)
	}
	if err != nil {
		return err
	}
	defer store.Close()

	return Run(&Env{
		Store:  store,
		Out:    out,
		Err:    s.Err,
		Cwd:    s.Cwd,
		Home:   s.Home,
		CDFile: s.CDFile,
		Editor: s.Editor,
		Now:    s.Now,
	}, cmd)
}

// writesStore reports whether an action needs the write lock.
func writesStore(a cli.Action) bool {
	switch a {
	case cli.ActionSet, cli.ActionEdit, cli.ActionReset, cli.ActionDelete,
		cli.ActionRename, cli.ActionSetNote:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/actions/ -run 'TestSession|TestSlots' -v`
Expected: all PASS

- [ ] **Step 5: Move main onto Session**

In `cmd/dl/main.go`, replace the body of `runStoreCommand` with:

```go
// runStoreCommand builds a Session and dispatches through it.
func runStoreCommand(cmd cli.Command) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locate the home directory: %w", err)
	}
	dir := slots.DefaultDir(os.Getenv, home)

	// One-time import of the legacy ~/.cdl layout. It is a no-op once the new
	// store exists, so calling it on every run costs a single stat.
	if n, err := slots.Migrate(home, dir); err != nil {
		fmt.Fprintf(os.Stderr, "dl: could not import ~/.cdl: %v\n", err)
	} else if n > 0 {
		fmt.Fprintf(os.Stderr, "dl: imported %d slot%s from ~/.cdl (kept as ~/.cdl.bak)\n",
			n, pluralS(n))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate the current directory: %w", err)
	}

	session := &actions.Session{
		Dir:    dir,
		Home:   home,
		Cwd:    cwd,
		CDFile: os.Getenv("DL_CD_FILE"),
		Editor: editor.OS{},
		Now:    time.Now,
		Err:    os.Stderr,
	}

	// The interactive browser arrives in task 8. Until then a bare `dl`
	// still falls back to listing.
	if cmd.Action == cli.ActionTUI {
		cmd.Action = cli.ActionSee
	}
	return session.Run(cmd, os.Stdout)
}
```

Delete the now-unused `writes` function from `main.go` — `Session` owns that
decision. Keep `pluralS`.

- [ ] **Step 6: Run the whole suite**

Run: `go test ./... -race -count=1 && go vet ./...`
Expected: all PASS, no vet findings. The phase 1 end-to-end tests must still
pass unchanged — that is the proof the refactor changed no behaviour.

- [ ] **Step 7: Commit**

```bash
git add internal/actions/session.go internal/actions/session_test.go cmd/dl/main.go
git commit -m "refactor(actions): put the store lifecycle in a Session"
```

---

### Task 2: The TUI skeleton — list, cursor, quit

**Files:**
- Modify: `go.mod` (add bubbletea, bubbles, lipgloss)
- Create: `internal/tui/tui.go`, `internal/tui/model.go`, `internal/tui/keys.go`, `internal/tui/fake_test.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Add the dependencies**

```bash
go get github.com/charmbracelet/bubbletea@v1.3.10
go get github.com/charmbracelet/bubbles@v1.0.0
go get github.com/charmbracelet/lipgloss@v1.1.0
go mod tidy
```

- [ ] **Step 2: Write the fake Session the model tests run against**

`internal/tui/fake_test.go`:

```go
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
func (f *fakeSession) HomeDir() string    { return "/home/u" }

// lastCommand is the most recent command the model issued.
func (f *fakeSession) lastCommand() cli.Command {
	if len(f.ran) == 0 {
		return cli.Command{}
	}
	return f.ran[len(f.ran)-1]
}

// threeSlots is the fixture most tests use: one with a note, one plain, one
// named.
func threeSlots() []slots.Slot {
	return []slots.Slot{
		{Number: 1, Name: "alpha", Path: "/home/u/alpha", Note: "first note\nsecond line"},
		{Number: 7, Path: "/home/u/work/beta"},
		{Number: 12, Name: "gamma", Path: "/home/u/gamma", Note: "gamma note"},
	}
}
```

- [ ] **Step 3: Write the failing model test**

`internal/tui/model_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// key builds a KeyMsg for a single rune.
func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// special builds a KeyMsg for a named key.
func special(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// newTestModel builds a model at a size wide enough for two panes.
func newTestModel(f *fakeSession) Model {
	m := New(f)
	// Fixture paths are not on disk; pin the marker column so tests assert on
	// behaviour rather than on what happens to exist.
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return updated.(Model)
}

func TestCursorMoves(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	if m.cursor != 0 {
		t.Fatalf("cursor starts at %d, want 0", m.cursor)
	}

	next, _ := m.Update(key('j'))
	m = next.(Model)
	if m.cursor != 1 {
		t.Fatalf("after j, cursor = %d, want 1", m.cursor)
	}

	next, _ = m.Update(special(tea.KeyDown))
	m = next.(Model)
	if m.cursor != 2 {
		t.Fatalf("after down, cursor = %d, want 2", m.cursor)
	}

	// The cursor stops at the end rather than wrapping: wrapping makes it
	// easy to overshoot onto the wrong slot and press enter.
	next, _ = m.Update(key('j'))
	m = next.(Model)
	if m.cursor != 2 {
		t.Fatalf("after j at the end, cursor = %d, want 2", m.cursor)
	}

	next, _ = m.Update(key('k'))
	m = next.(Model)
	if m.cursor != 1 {
		t.Fatalf("after k, cursor = %d, want 1", m.cursor)
	}
}

func TestGAndShiftGJumpToTheEnds(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('G'))
	m = next.(Model)
	if m.cursor != 2 {
		t.Fatalf("after G, cursor = %d, want 2", m.cursor)
	}
	next, _ = m.Update(key('g'))
	m = next.(Model)
	if m.cursor != 0 {
		t.Fatalf("after g, cursor = %d, want 0", m.cursor)
	}
}

func TestCursorOnAnEmptyStore(t *testing.T) {
	m := newTestModel(&fakeSession{})
	// Moving around an empty list must not panic or go negative.
	for _, k := range []rune{'j', 'k', 'g', 'G'} {
		next, _ := m.Update(key(k))
		m = next.(Model)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor = %d on an empty store, want 0", m.cursor)
	}
	if m.selected() != nil {
		t.Fatal("selected() on an empty store should be nil")
	}
}

func TestQuitDoesNotJump(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	_, cmd := m.Update(key('q'))
	if cmd == nil {
		t.Fatal("q produced no command, want tea.Quit")
	}
	if len(f.ran) != 0 {
		t.Fatalf("q ran %v, want nothing", f.ran)
	}
}

func TestEnterJumpsAndQuits(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('j')) // select slot 7
	m = next.(Model)

	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter produced no command, want tea.Quit")
	}
	got := f.lastCommand()
	if got.Action != cli.ActionCD {
		t.Fatalf("action = %v, want ActionCD", got.Action)
	}
	if got.Ref != "7" {
		t.Fatalf("ref = %q, want %q", got.Ref, "7")
	}
}

// A failed jump keeps the browser open and shows why, rather than quitting
// into an error the user cannot act on.
func TestEnterOnAFailureStaysOpen(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	m := newTestModel(f)
	next, cmd := m.Update(special(tea.KeyEnter))
	m = next.(Model)
	if cmd != nil {
		t.Fatal("a failed jump should not quit")
	}
	if m.status == "" {
		t.Fatal("a failed jump should show a status message")
	}
}

// errDead is a stand-in for "that directory is gone".
type errDead struct{}

func (errDead) Error() string { return "slot 1 points at /gone, which no longer exists" }

func TestEnterOnAnEmptyStoreDoesNothing(t *testing.T) {
	f := &fakeSession{}
	m := newTestModel(f)
	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter on an empty store should not quit")
	}
	if len(f.ran) != 0 {
		t.Fatalf("enter on an empty store ran %v, want nothing", f.ran)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `go test ./internal/tui/ -v`
Expected: build failure, `undefined: New`

- [ ] **Step 5: Write `internal/tui/keys.go`**

```go
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
```

- [ ] **Step 6: Write `internal/tui/model.go`**

```go
// Package tui is dl's interactive browser. It owns no behaviour of its own:
// every key that changes something builds a cli.Command and hands it to the
// same actions the command line uses. Model.Update is a pure function of
// state and message, which is what makes the whole interaction testable
// without a terminal.
package tui

import (
	"io"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Session is what the browser needs from the store. actions.Session
// implements it; the tests use a fake.
type Session interface {
	Slots() ([]slots.Slot, error)
	Run(cmd cli.Command, out io.Writer) error
	CurrentDir() string // what `a` would save
	HomeDir() string    // for shortening paths for display
}

// mode is which input the browser is currently taking.
type mode int

const (
	modeList mode = iota
	modeHelp
)

// Model is the browser's whole state.
type Model struct {
	session Session

	slots  []slots.Slot // everything the store holds
	view   []slots.Slot // what the current filter admits
	cursor int          // index into view

	mode   mode
	status string // the last action's output or error, shown in the footer

	width, height int

	// finalOutput is printed to stdout after the program exits, so the note
	// a jump prints survives the alternate screen being torn down.
	finalOutput string

	// exists reports whether a slot's directory is still there. It is a seam
	// so that layout tests can pin the marker column without creating real
	// directories. Nil means the real filesystem check.
	exists func(path string) bool
}

// slotExists reports whether a slot is still valid, through the seam when
// there is one.
func (m Model) slotExists(sl slots.Slot) bool {
	if m.exists != nil {
		return m.exists(sl.Path)
	}
	return sl.Exists()
}

// New builds a browser over a session. Call Run rather than using this
// directly, except in tests.
func New(s Session) Model {
	return Model{session: s}
}

// Init loads the slots.
func (m Model) Init() tea.Cmd { return m.reload() }

// reloadMsg carries a fresh slot list, or the error that stopped it.
type reloadMsg struct {
	slots []slots.Slot
	err   error
}

// reload re-reads the store. It is a command rather than a direct call so
// that a save made in another shell is picked up the same way as one made
// here.
func (m Model) reload() tea.Cmd {
	return func() tea.Msg {
		got, err := m.session.Slots()
		return reloadMsg{slots: got, err: err}
	}
}

// selected is the slot under the cursor, or nil when the list is empty.
func (m Model) selected() *slots.Slot {
	if m.cursor < 0 || m.cursor >= len(m.view) {
		return nil
	}
	return &m.view[m.cursor]
}

// applyFilter recomputes view from slots. Task 3 gives it a filter to apply;
// for now every slot is admitted.
func (m *Model) applyFilter() {
	m.view = m.slots
	m.clampCursor()
}

// clampCursor keeps the cursor inside the view after it changes size.
func (m *Model) clampCursor() {
	if m.cursor >= len(m.view) {
		m.cursor = len(m.view) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Update is the state machine. Every branch returns a new Model; nothing is
// mutated through a pointer receiver from here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case reloadMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.slots = msg.slots
		m.applyFilter()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes a keypress to the current mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeHelp {
		m.mode = modeList // any key closes the overlay
		return m, nil
	}
	return m.handleListKey(msg)
}

// handleListKey is the main keymap.
func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.cursor--
		m.clampCursor()
		return m, nil
	case "down", "j":
		if m.cursor < len(m.view)-1 {
			m.cursor++
		}
		return m, nil
	case "g", "home":
		m.cursor = 0
		return m, nil
	case "G", "end":
		m.cursor = len(m.view) - 1
		m.clampCursor()
		return m, nil

	case "?":
		m.mode = modeHelp
		return m, nil

	case "enter":
		return m.jump()
	}
	return m, nil
}

// jump runs --cd on the selected slot and quits. On failure it stays open and
// reports why, because an error the user can act on is worth a screen.
func (m Model) jump() (tea.Model, tea.Cmd) {
	sl := m.selected()
	if sl == nil {
		return m, nil
	}
	var out strings.Builder
	if err := m.session.Run(cli.Command{
		Action: cli.ActionCD,
		Ref:    strconv.Itoa(sl.Number),
	}, &out); err != nil {
		m.status = err.Error()
		return m, nil
	}
	// The note the cd action printed is shown after the alternate screen is
	// torn down, so it is still on screen in the new directory.
	m.finalOutput = out.String()
	return m, tea.Quit
}
```

- [ ] **Step 7: Write a stub `internal/tui/view.go` so the package builds**

```go
package tui

// View renders the browser. Task 7 gives it the real layout; this is enough
// for the model tests to compile and run.
func (m Model) View() string {
	if m.mode == modeHelp {
		return helpOverlay
	}
	var b []byte
	for i, sl := range m.view {
		if i == m.cursor {
			b = append(b, '>')
		} else {
			b = append(b, ' ')
		}
		b = append(b, ' ')
		b = append(b, sl.DisplayName()...)
		b = append(b, '\n')
	}
	return string(b)
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -v`
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/tui
git commit -m "feat(tui): add the model skeleton with cursor movement and jump"
```

---

### Task 3: Filtering

**Files:**
- Create: `internal/tui/filter.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/filter_test.go`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// typeString feeds a string to the model one rune at a time.
func typeString(m Model, s string) Model {
	for _, r := range s {
		next, _ := m.Update(key(r))
		m = next.(Model)
	}
	return m
}

func TestMatchesNamePathAndNote(t *testing.T) {
	sl := slots.Slot{Number: 1, Name: "alpha", Path: "/home/u/projects/api", Note: "retry loop"}
	for _, q := range []string{"alpha", "ALPHA", "api", "projects", "retry", "loop"} {
		if !matches(sl, q) {
			t.Errorf("matches(%q) = false, want true", q)
		}
	}
	for _, q := range []string{"zzz", "beta"} {
		if matches(sl, q) {
			t.Errorf("matches(%q) = true, want false", q)
		}
	}
}

// An unnamed slot is matched on its display name, exactly as the ref grammar
// does on the command line.
func TestMatchesTheDisplayNameOfAnUnnamedSlot(t *testing.T) {
	sl := slots.Slot{Number: 1, Path: "/home/u/scraper"}
	if !matches(sl, "scrap") {
		t.Error("an unnamed slot is not matched on its base name")
	}
}

func TestSlashFiltersTheList(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = next.(Model)
	if m.mode != modeFilter {
		t.Fatalf("mode = %v after /, want modeFilter", m.mode)
	}

	m = typeString(m, "gam")
	if len(m.view) != 1 || m.view[0].Number != 12 {
		t.Fatalf("view = %+v, want only slot 12", m.view)
	}
}

// The filter reads notes, which is how you find a project you remember by
// what you were doing rather than by where it lives.
func TestFilterSearchesNotes(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "second")
	if len(m.view) != 1 || m.view[0].Number != 1 {
		t.Fatalf("view = %+v, want only slot 1 (matched in its note)", m.view)
	}
}

func TestEnterKeepsTheFilterAndReturnsToTheList(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gam")

	next, _ = m.Update(special(tea.KeyEnter))
	m = next.(Model)
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
	if len(m.view) != 1 {
		t.Fatalf("view = %d entries, want the filter kept", len(m.view))
	}
}

func TestEscapeClearsTheFilter(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gam")

	next, _ = m.Update(special(tea.KeyEsc))
	m = next.(Model)
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
	if len(m.view) != 3 {
		t.Fatalf("view = %d entries, want all 3 back", len(m.view))
	}
}

// Filtering down to fewer entries than the cursor's index must not leave the
// cursor pointing past the end.
func TestFilteringClampsTheCursor(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('G')) // cursor on the last entry
	m = next.(Model)

	next, _ = m.Update(key('/'))
	m = typeString(next.(Model), "alpha")
	if m.cursor != 0 {
		t.Fatalf("cursor = %d after filtering to one entry, want 0", m.cursor)
	}
	if m.selected() == nil {
		t.Fatal("selected() is nil after filtering")
	}
}

func TestFilterWithNoMatches(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "zzzz")
	if len(m.view) != 0 {
		t.Fatalf("view = %+v, want empty", m.view)
	}
	if m.selected() != nil {
		t.Fatal("selected() should be nil when nothing matches")
	}
	// Enter on nothing must not quit.
	_, cmd := m.Update(special(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("enter with no matches should not quit")
	}
}

func TestBackspaceWidensTheFilter(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gamm")
	next, _ = m.Update(special(tea.KeyBackspace))
	next, _ = next.(Model).Update(special(tea.KeyBackspace))
	next, _ = next.(Model).Update(special(tea.KeyBackspace))
	next, _ = next.(Model).Update(special(tea.KeyBackspace))
	m = next.(Model)
	if len(m.view) != 3 {
		t.Fatalf("view = %d entries after clearing the query, want 3", len(m.view))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/tui/ -run 'TestMatches|TestSlash|TestFilter|TestEnterKeeps|TestEscape|TestBackspace' -v`
Expected: build failure, `undefined: matches`, `undefined: modeFilter`

- [ ] **Step 3: Write `internal/tui/filter.go`**

```go
package tui

import (
	"strings"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// matches reports whether a slot satisfies a query.
//
// Case-insensitive substring across the display name, the path and the note.
// Substring rather than fuzzy: on a list of a dozen entries fuzzy mostly
// produces surprises, and a predictable filter is worth more than a clever
// one. Swapping in fuzzy later means replacing this function and nothing else.
//
// Searching the note is what lets you find a project by what you were doing
// rather than by where it lives.
func matches(sl slots.Slot, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	for _, field := range []string{sl.DisplayName(), sl.Path, sl.Note} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// filterSlots returns the slots a query admits, in the order given.
func filterSlots(all []slots.Slot, query string) []slots.Slot {
	if query == "" {
		return all
	}
	out := make([]slots.Slot, 0, len(all))
	for _, sl := range all {
		if matches(sl, query) {
			out = append(out, sl)
		}
	}
	return out
}
```

- [ ] **Step 4: Wire the filter into `model.go`**

Add `modeFilter` to the mode constants:

```go
const (
	modeList mode = iota
	modeFilter
	modeHelp
)
```

Add the input to `Model`, after `mode`:

```go
	// filter is the query typed after `/`. It stays applied when the user
	// returns to the list, so you can filter and then act on what is left.
	filter textinput.Model
```

Set it up in `New`:

```go
func New(s Session) Model {
	in := textinput.New()
	in.Prompt = "/"
	in.Placeholder = "name, path or note"
	return Model{session: s, filter: in}
}
```

Replace `applyFilter`:

```go
// applyFilter recomputes view from slots and the current query.
func (m *Model) applyFilter() {
	m.view = filterSlots(m.slots, m.filter.Value())
	m.clampCursor()
}
```

Route filter-mode keys in `handleKey`, before the list branch:

```go
	if m.mode == modeFilter {
		return m.handleFilterKey(msg)
	}
```

And add:

```go
// handleFilterKey feeds the query input and refilters on every keystroke, so
// the list narrows as you type.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Keep the query; the user filtered in order to act on what is left.
		m.mode = modeList
		m.filter.Blur()
		return m, nil
	case "esc", "ctrl+c":
		m.mode = modeList
		m.filter.Blur()
		m.filter.SetValue("")
		m.applyFilter()
		return m, nil
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()
	return m, cmd
}
```

Open filter mode from the list keymap, next to `?`:

```go
	case "/":
		m.mode = modeFilter
		m.filter.Focus()
		return m, textinput.Blink
```

Import `"github.com/charmbracelet/bubbles/textinput"`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): filter by name, path and note as you type"
```

---

### Task 4: The note pane

**Files:**
- Modify: `internal/tui/model.go`
- Test: `internal/tui/note_test.go`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestTheNotePaneFollowsTheCursor(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	if !strings.Contains(m.note.View(), "first note") {
		t.Fatalf("note pane = %q, want slot 1's note", m.note.View())
	}

	next, _ := m.Update(key('G')) // slot 12
	m = next.(Model)
	if !strings.Contains(m.note.View(), "gamma note") {
		t.Fatalf("note pane = %q, want slot 12's note", m.note.View())
	}
}

func TestTheNotePaneIsEmptyForASlotWithoutOne(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('j')) // slot 7, no note
	m = next.(Model)
	if strings.TrimSpace(m.note.View()) != "" {
		t.Fatalf("note pane = %q, want empty", m.note.View())
	}
}

func TestALongNoteScrolls(t *testing.T) {
	long := strings.Repeat("a line of the note\n", 80)
	m := newTestModel(&fakeSession{slots: []slots.Slot{
		{Number: 1, Name: "long", Path: "/home/u/x", Note: long},
	}})
	if m.note.AtBottom() {
		t.Fatal("an 80-line note fits on a 24-row screen? the viewport is not sized")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = next.(Model)
	if m.note.YOffset == 0 {
		t.Fatal("ctrl-d did not scroll the note")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(Model)
	if m.note.YOffset != 0 {
		t.Fatalf("ctrl-u left the offset at %d, want back at 0", m.note.YOffset)
	}
}

// Moving to another slot resets the scroll, so you never read slot B's note
// starting halfway down because slot A's was long.
func TestMovingResetsTheScroll(t *testing.T) {
	long := strings.Repeat("line\n", 80)
	m := newTestModel(&fakeSession{slots: []slots.Slot{
		{Number: 1, Path: "/home/u/a", Note: long},
		{Number: 2, Path: "/home/u/b", Note: "short"},
	}})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	next, _ = next.(Model).Update(key('j'))
	m = next.(Model)
	if m.note.YOffset != 0 {
		t.Fatalf("YOffset = %d after moving, want 0", m.note.YOffset)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/tui/ -run 'TestTheNote|TestALongNote|TestMovingResets' -v`
Expected: build failure, `m.note undefined`

- [ ] **Step 3: Wire the viewport into `model.go`**

Add to `Model`, after `filter`:

```go
	// note is the right-hand pane. It is a viewport so that a note longer
	// than the screen can be scrolled rather than truncated.
	note viewport.Model
```

Add a helper and call it wherever the cursor or the view changes:

```go
// syncNote points the note pane at the selected slot and rewinds it to the
// top, so moving to another slot never lands you halfway down its note.
func (m *Model) syncNote() {
	sl := m.selected()
	if sl == nil {
		m.note.SetContent("")
	} else {
		m.note.SetContent(sl.Note)
	}
	m.note.GotoTop()
}
```

Call it from exactly two places, which between them cover every way the
selection can change:

1. the last line of `applyFilter`, after `m.clampCursor()` — this covers a
   reload and every filter keystroke;
2. each of the five movement cases in `handleListKey` (`up`/`k`, `down`/`j`,
   `g`/`home`, `G`/`end`), just before `return m, nil`.

Do not call it from `clampCursor`: that runs on a pointer receiver from inside
`applyFilter` and would double the work for no gain.

Size it on `tea.WindowSizeMsg`:

```go
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.note.Width = notePaneWidth(msg.Width)
		m.note.Height = paneHeight(msg.Height)
		m.syncNote()
		return m, nil
```

Add the two sizing helpers (task 7 reuses them for the layout):

```go
// Layout constants. Below wideMin the note moves under the list; below
// narrowMin it is dropped entirely.
const (
	wideMin   = 80
	narrowMin = 60

	chromeRows = 4 // header, blank, footer, and the status line
)

// notePaneWidth is how wide the note pane gets at a given terminal width.
func notePaneWidth(total int) int {
	if total >= wideMin {
		return total - listPaneWidth(total) - 3 // 3 for the gap and borders
	}
	return total - 2
}

// listPaneWidth is how wide the list pane gets. The list gets the larger
// share because a truncated path is harder to recognise than a wrapped note.
func listPaneWidth(total int) int {
	if total >= wideMin {
		return total * 55 / 100
	}
	return total - 2
}

// paneHeight is how many rows a pane gets.
func paneHeight(total int) int {
	h := total - chromeRows
	if h < 1 {
		return 1
	}
	return h
}
```

Add the scroll keys to `handleListKey`:

```go
	case "ctrl+d", "pgdown":
		m.note.HalfViewDown()
		return m, nil
	case "ctrl+u", "pgup":
		m.note.HalfViewUp()
		return m, nil
```

Import `"github.com/charmbracelet/bubbles/viewport"`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): show the selected slot's note in a scrollable pane"
```

---

### Task 5: Add, delete, reset and rename

Single-keystroke destruction gets a confirmation, unlike the command line
where the whole command was typed deliberately.

**Files:**
- Modify: `internal/tui/model.go`, `internal/tui/keys.go`
- Test: `internal/tui/mutate_test.go`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

func TestAddSavesTheCurrentDirectory(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('a'))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionSet {
		t.Fatalf("action = %v, want ActionSet", got.Action)
	}
	if got.Ref != "" {
		t.Fatalf("ref = %q, want empty so the lowest free slot is used", got.Ref)
	}
}

func TestDeleteAsksFirst(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('d'))
	m = next.(Model)

	if m.mode != modeConfirm {
		t.Fatalf("mode = %v after d, want modeConfirm", m.mode)
	}
	if len(f.ran) != 0 {
		t.Fatalf("d ran %v before confirmation", f.ran)
	}
	if m.confirmPrompt == "" {
		t.Fatal("no confirmation prompt was set")
	}
}

func TestDeleteProceedsOnY(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('d'))
	next, _ = next.(Model).Update(key('y'))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionDelete || got.Ref != "1" {
		t.Fatalf("command = %+v, want delete of slot 1", got)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v after confirming, want modeList", m.mode)
	}
}

func TestDeleteCancelsOnAnythingElse(t *testing.T) {
	for _, k := range []rune{'n', 'x', 'q'} {
		f := &fakeSession{slots: threeSlots()}
		m := newTestModel(f)
		next, _ := m.Update(key('d'))
		next, _ = next.(Model).Update(key(k))
		m = next.(Model)

		if len(f.ran) != 0 {
			t.Fatalf("%q ran %v, want the delete cancelled", string(k), f.ran)
		}
		if m.mode != modeList {
			t.Fatalf("mode = %v after cancelling with %q, want modeList", m.mode, string(k))
		}
	}
}

func TestResetAsksFirstToo(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('r'))
	next, _ = next.(Model).Update(key('y'))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionReset || got.Ref != "1" {
		t.Fatalf("command = %+v, want reset of slot 1", got)
	}
}

func TestRenameTakesTheNewNameInline(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('n'))
	m = next.(Model)
	if m.mode != modeRename {
		t.Fatalf("mode = %v after n, want modeRename", m.mode)
	}
	// The input starts from the current name so a small correction is easy.
	if m.rename.Value() != "alpha" {
		t.Fatalf("rename input = %q, want the current name", m.rename.Value())
	}

	m = typeString(m, "2")
	next, _ = m.Update(special(tea.KeyEnter))
	m = next.(Model)

	got := f.lastCommand()
	if got.Action != cli.ActionRename || got.Ref != "1" || got.Name != "alpha2" {
		t.Fatalf("command = %+v, want rename of slot 1 to alpha2", got)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
}

func TestRenameCancelsOnEscape(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, _ := m.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEsc))
	m = next.(Model)

	if len(f.ran) != 0 {
		t.Fatalf("escape ran %v, want nothing", f.ran)
	}
	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList", m.mode)
	}
}

// A rejected name (duplicate, numeric, spaced) must surface the reason rather
// than silently doing nothing.
func TestARejectedRenameShowsTheError(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	m := newTestModel(f)
	next, _ := m.Update(key('n'))
	next, _ = next.(Model).Update(special(tea.KeyEnter))
	m = next.(Model)

	if m.status == "" {
		t.Fatal("a rejected rename left no status message")
	}
}

// Every mutation reloads, so the list reflects what just happened.
func TestAMutationTriggersAReload(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	_, cmd := m.Update(key('a'))
	if cmd == nil {
		t.Fatal("add produced no command, want a reload")
	}
	msg := cmd()
	if _, ok := msg.(reloadMsg); !ok {
		t.Fatalf("command produced %T, want reloadMsg", msg)
	}
}

// Keys that act on a slot do nothing when there is no slot under the cursor.
func TestSlotKeysAreNoOpsOnAnEmptyStore(t *testing.T) {
	for _, k := range []rune{'d', 'r', 'n', 'e'} {
		f := &fakeSession{}
		m := newTestModel(f)
		next, _ := m.Update(key(k))
		m = next.(Model)
		if m.mode != modeList {
			t.Fatalf("%q changed mode to %v on an empty store", string(k), m.mode)
		}
		if len(f.ran) != 0 {
			t.Fatalf("%q ran %v on an empty store", string(k), f.ran)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/tui/ -run 'TestAdd|TestDelete|TestReset|TestRename|TestARejected|TestAMutation|TestSlotKeys' -v`
Expected: build failure, `undefined: modeConfirm`

- [ ] **Step 3: Extend `model.go`**

Add the modes:

```go
const (
	modeList mode = iota
	modeFilter
	modeRename
	modeConfirm
	modeHelp
)
```

Add to `Model`:

```go
	// rename is the inline input shown by `n`.
	rename textinput.Model

	// confirmPrompt is the question shown in modeConfirm, and confirmCmd is
	// what running it would do. A single keystroke destroying something
	// deserves a question; a typed command line does not.
	confirmPrompt string
	confirmCmd    cli.Command
```

Build the rename input in `New`:

```go
	ren := textinput.New()
	ren.Prompt = "name: "
	ren.CharLimit = 40
```

Add the dispatch helper:

```go
// run executes a command, records what it said, and schedules a reload so the
// list reflects the change.
func (m Model) run(cmd cli.Command) (tea.Model, tea.Cmd) {
	var out strings.Builder
	if err := m.session.Run(cmd, &out); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.status = strings.TrimSpace(firstLine(out.String()))
	return m, m.reload()
}

// firstLine is the part of an action's output that fits in a status bar.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
```

Add the list keys:

```go
	case "a":
		// No ref: --set picks the lowest free slot.
		return m.run(cli.Command{Action: cli.ActionSet})

	case "d":
		sl := m.selected()
		if sl == nil {
			return m, nil
		}
		m.mode = modeConfirm
		m.confirmPrompt = fmt.Sprintf("delete slot %d (%s)? [y/N]", sl.Number, sl.DisplayName())
		m.confirmCmd = cli.Command{Action: cli.ActionDelete, Ref: strconv.Itoa(sl.Number)}
		return m, nil

	case "r":
		sl := m.selected()
		if sl == nil {
			return m, nil
		}
		m.mode = modeConfirm
		m.confirmPrompt = fmt.Sprintf("clear slot %d's name and note? [y/N]", sl.Number)
		m.confirmCmd = cli.Command{Action: cli.ActionReset, Ref: strconv.Itoa(sl.Number)}
		return m, nil

	case "n":
		sl := m.selected()
		if sl == nil {
			return m, nil
		}
		m.mode = modeRename
		m.rename.SetValue(sl.Name)
		m.rename.CursorEnd()
		m.rename.Focus()
		return m, textinput.Blink
```

Add the two mode handlers, routed from `handleKey`:

```go
// handleConfirmKey takes the y/N answer. Anything but y cancels, so a
// mistyped key is always the safe outcome.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeList
	prompt := m.confirmCmd
	m.confirmPrompt = ""
	m.confirmCmd = cli.Command{}
	if msg.String() != "y" && msg.String() != "Y" {
		m.status = "cancelled"
		return m, nil
	}
	return m.run(prompt)
}

// handleRenameKey takes the new name.
func (m Model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeList
		m.rename.Blur()
		return m, nil
	case "enter":
		sl := m.selected()
		m.mode = modeList
		m.rename.Blur()
		if sl == nil {
			return m, nil
		}
		return m.run(cli.Command{
			Action: cli.ActionRename,
			Ref:    strconv.Itoa(sl.Number),
			Name:   m.rename.Value(),
		})
	}
	var cmd tea.Cmd
	m.rename, cmd = m.rename.Update(msg)
	return m, cmd
}
```

Import `"fmt"`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): add, rename, reset and delete from the browser"
```

---

### Task 6: Editing a note from the browser

`e` has to hand the terminal to `$EDITOR` and take it back. bubbletea's
`tea.Exec` is the primitive for that.

**Files:**
- Create: `internal/tui/exec.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/exec_test.go`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

func TestEEditsTheSelectedSlotsNote(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	_, cmd := m.Update(key('e'))
	if cmd == nil {
		t.Fatal("e produced no command, want a tea.Exec")
	}
	// The exec wrapper runs the edit when bubbletea invokes it; calling Run
	// directly is what the terminal handover amounts to.
	ec := &editCommand{session: f, ref: "1"}
	if err := ec.Run(); err != nil {
		t.Fatalf("editCommand.Run: %v", err)
	}
	got := f.lastCommand()
	if got.Action != cli.ActionEdit || got.Ref != "1" {
		t.Fatalf("command = %+v, want edit of slot 1", got)
	}
}

func TestEditCommandReportsFailure(t *testing.T) {
	f := &fakeSession{slots: threeSlots(), err: errDead{}}
	ec := &editCommand{session: f, ref: "1"}
	if err := ec.Run(); err == nil {
		t.Fatal("editCommand.Run = nil, want the session's error")
	}
}

// Coming back from the editor must reload, or the pane would still show the
// note as it was before the edit.
func TestReturningFromTheEditorReloads(t *testing.T) {
	f := &fakeSession{slots: threeSlots()}
	m := newTestModel(f)
	next, cmd := m.Update(editFinishedMsg{})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("returning from the editor produced no command, want a reload")
	}
	if _, ok := cmd().(reloadMsg); !ok {
		t.Fatal("returning from the editor did not reload")
	}
}

func TestAFailedEditIsReported(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(editFinishedMsg{err: errDead{}})
	m = next.(Model)
	if m.status == "" {
		t.Fatal("a failed edit left no status message")
	}
}

var _ tea.ExecCommand = (*editCommand)(nil)
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/tui/ -run 'TestE|TestEdit|TestReturning|TestAFailed' -v`
Expected: build failure, `undefined: editCommand`

- [ ] **Step 3: Write `internal/tui/exec.go`**

```go
package tui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// editCommand runs the --edit action with the terminal handed back to the
// editor.
//
// bubbletea's tea.Exec suspends the program, restores the terminal to its
// normal state, runs this, and then takes the screen back. That is why the
// editor can be a full-screen program like vim.
type editCommand struct {
	session Session
	ref     string
}

// Run performs the edit. The action's own editor attaches to os.Stdin and
// os.Stdout, which at this point are the real terminal bubbletea has just
// released, so the Set* methods below have nothing to do.
func (c *editCommand) Run() error {
	return c.session.Run(cli.Command{Action: cli.ActionEdit, Ref: c.ref}, io.Discard)
}

// SetStdin, SetStdout and SetStderr satisfy tea.ExecCommand. They are no-ops
// because the editor process is spawned by internal/editor against the real
// standard streams rather than through this interface.
func (c *editCommand) SetStdin(io.Reader)  {}
func (c *editCommand) SetStdout(io.Writer) {}
func (c *editCommand) SetStderr(io.Writer) {}

// editFinishedMsg is delivered once the editor exits.
type editFinishedMsg struct{ err error }
```

- [ ] **Step 4: Wire `e` into `model.go`**

In `handleListKey`:

```go
	case "e":
		sl := m.selected()
		if sl == nil {
			return m, nil
		}
		return m, tea.Exec(
			&editCommand{session: m.session, ref: strconv.Itoa(sl.Number)},
			func(err error) tea.Msg { return editFinishedMsg{err: err} },
		)
```

In `Update`, next to `reloadMsg`:

```go
	case editFinishedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = "note saved"
		return m, m.reload()
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): edit a note in \$EDITOR without leaving the browser"
```

---

### Task 7: The real view

**Files:**
- Create: `internal/tui/style.go`
- Rewrite: `internal/tui/view.go`
- Test: `internal/tui/view_test.go`

- [ ] **Step 1: Write the failing test**

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// sized renders the model at a given terminal size.
func sized(f *fakeSession, w, h int) string {
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model).View()
}

func TestViewShowsEverySlot(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	for _, want := range []string{"alpha", "beta", "gamma", "1", "7", "12"} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

func TestViewMarksNotesAndDeadPaths(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	if !strings.Contains(got, "*") {
		t.Errorf("view has no note marker:\n%s", got)
	}

	// With the seam saying nothing exists, every row carries the dead marker
	// and the dead marker wins over the note marker.
	m := New(&fakeSession{slots: threeSlots()})
	m.exists = func(string) bool { return false }
	m.slots = threeSlots()
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	dead := updated.(Model).View()
	if !strings.Contains(dead, "x") {
		t.Errorf("view has no dead-path marker:\n%s", dead)
	}
}

func TestViewShortensThePath(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	if strings.Contains(got, "/home/u/alpha") {
		t.Errorf("view shows an unshortened path:\n%s", got)
	}
	if !strings.Contains(got, "~/alpha") {
		t.Errorf("view does not shorten the home directory:\n%s", got)
	}
}

func TestViewShowsTheKeyHints(t *testing.T) {
	got := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	if !strings.Contains(got, "cd") || !strings.Contains(got, "help") {
		t.Errorf("view has no key hints:\n%s", got)
	}
}

func TestViewOnAnEmptyStoreSaysWhatToDo(t *testing.T) {
	got := sized(&fakeSession{}, 100, 24)
	if !strings.Contains(got, "no slots yet") {
		t.Errorf("empty view does not say what to do:\n%s", got)
	}
	if !strings.Contains(got, "a ") && !strings.Contains(got, "press a") {
		t.Errorf("empty view does not mention the add key:\n%s", got)
	}
}

func TestNarrowLayoutDropsTheSidePane(t *testing.T) {
	wide := sized(&fakeSession{slots: threeSlots()}, 100, 24)
	narrow := sized(&fakeSession{slots: threeSlots()}, 50, 24)

	if widestLine(narrow) > 50 {
		t.Errorf("narrow view is %d columns wide, want at most 50:\n%s", widestLine(narrow), narrow)
	}
	if widestLine(wide) > 100 {
		t.Errorf("wide view is %d columns wide, want at most 100", widestLine(wide))
	}
}

// Nothing may exceed the terminal width at any size, or the display wraps and
// the layout falls apart.
func TestNoLineExceedsTheTerminalWidth(t *testing.T) {
	long := slots.Slot{
		Number: 999,
		Name:   "a-very-long-slot-name-indeed",
		Path:   "/home/u/one/two/three/four/five/six/seven/eight/nine/ten/eleven",
		Note:   strings.Repeat("a long note line that will need wrapping ", 5),
	}
	for _, w := range []int{40, 50, 60, 79, 80, 100, 200} {
		got := sized(&fakeSession{slots: []slots.Slot{long}}, w, 24)
		if widest := widestLine(got); widest > w {
			t.Errorf("at width %d the view is %d columns wide:\n%s", w, widest, got)
		}
	}
}

func TestHelpOverlayReplacesTheView(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('?'))
	got := next.(Model).View()
	if !strings.Contains(got, "ctrl-u") {
		t.Errorf("help overlay is missing keys:\n%s", got)
	}
}

func TestTheFilterQueryIsVisible(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('/'))
	m = typeString(next.(Model), "gam")
	if !strings.Contains(m.View(), "gam") {
		t.Errorf("the filter query is not shown:\n%s", m.View())
	}
}

func TestTheConfirmPromptIsVisible(t *testing.T) {
	m := newTestModel(&fakeSession{slots: threeSlots()})
	next, _ := m.Update(key('d'))
	if !strings.Contains(next.(Model).View(), "[y/N]") {
		t.Errorf("the confirmation prompt is not shown:\n%s", next.(Model).View())
	}
}

// widestLine is the visible width of the longest line, ANSI escapes excluded.
func widestLine(s string) int {
	widest := 0
	for _, line := range strings.Split(s, "\n") {
		if w := visibleWidth(line); w > widest {
			widest = w
		}
	}
	return widest
}
```

- [ ] **Step 2: Write `internal/tui/style.go`**

```go
package tui

import "github.com/charmbracelet/lipgloss"

// Styles. Every colour is an AdaptiveColor so the browser reads on a light
// terminal as well as a dark one, and lipgloss drops colour entirely when the
// terminal cannot do it or when NO_COLOR is set.
var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#5a4fcf", Dark: "#b4a7ff"})

	styleCursor = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#ffffff"})

	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6c6c6c", Dark: "#8a8a8a"})

	styleNumber = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#2f6f4f", Dark: "#7fd1a8"})

	styleDead = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#a02020", Dark: "#ff8a8a"})

	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#8a5a00", Dark: "#e8c07d"})

	styleNotePane = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#4a4a4a"}).
			PaddingLeft(1)
)

// visibleWidth is the rendered width of a string, ignoring ANSI escapes. It
// is what the layout tests measure with.
func visibleWidth(s string) int { return lipgloss.Width(s) }
```

- [ ] **Step 3: Rewrite `internal/tui/view.go`**

```go
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// View renders the browser: a header, the list, the note pane beside or below
// it depending on the width, and a footer that is either the key hints, the
// filter input, the rename input, a confirmation or the last status message.
func (m Model) View() string {
	if m.mode == modeHelp {
		return helpOverlay
	}
	if m.width == 0 {
		return "" // no size yet; bubbletea sends one immediately
	}

	body := m.renderBody()
	return strings.Join([]string{
		m.renderHeader(),
		"",
		body,
		m.renderFooter(),
	}, "\n")
}

// renderHeader is the title line with the slot count.
func (m Model) renderHeader() string {
	count := fmt.Sprintf("%d slot%s", len(m.view), plural(len(m.view)))
	if len(m.view) != len(m.slots) {
		count = fmt.Sprintf("%d of %d slots", len(m.view), len(m.slots))
	}
	left := styleHeader.Render("dl")
	right := styleDim.Render(count)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left, m.width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderBody is the list, with the note pane beside it when there is room.
func (m Model) renderBody() string {
	if len(m.view) == 0 {
		return m.renderEmpty()
	}
	list := m.renderList()
	if m.width < wideMin {
		// Too narrow for two panes: put the note underneath, or drop it.
		if m.width < narrowMin {
			return list
		}
		return list + "\n" + styleDim.Render(truncate(m.note.View(), m.width))
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(listPaneWidth(m.width)).Render(list),
		styleNotePane.Width(notePaneWidth(m.width)).Render(m.note.View()),
	)
}

// renderEmpty is what a fresh install sees.
func (m Model) renderEmpty() string {
	return styleDim.Render("  no slots yet — press a to save the current directory")
}

// renderList draws one row per visible slot, fitted to the pane width.
func (m Model) renderList() string {
	width := listPaneWidth(m.width)
	numWidth, nameWidth := m.columnWidths()

	var b strings.Builder
	for i, sl := range m.view {
		cursor := "  "
		if i == m.cursor {
			cursor = styleCursor.Render("▸ ")
		}
		mark := " "
		if sl.Note != "" {
			mark = "*"
		}
		if !m.slotExists(sl) {
			mark = styleDead.Render("x")
		}

		number := styleNumber.Render(fmt.Sprintf("%*d", numWidth, sl.Number))
		name := fmt.Sprintf("%-*s", nameWidth, sl.DisplayName())
		path := styleDim.Render(slots.ShortPath(sl.Path, m.session.HomeDir()))

		row := cursor + mark + " " + number + "  " + name + "  " + path
		b.WriteString(truncate(row, width))
		if i < len(m.view)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// columnWidths measures the number and name columns, capping names so that a
// single long name cannot squeeze every path off the screen.
func (m Model) columnWidths() (numWidth, nameWidth int) {
	const maxName = 20
	for _, sl := range m.view {
		if w := len(strconv.Itoa(sl.Number)); w > numWidth {
			numWidth = w
		}
		if w := lipgloss.Width(sl.DisplayName()); w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > maxName {
		nameWidth = maxName
	}
	return numWidth, nameWidth
}

// renderFooter shows whichever prompt the current mode needs, falling back to
// the last status message and then to the key hints.
func (m Model) renderFooter() string {
	switch m.mode {
	case modeFilter:
		return truncate(m.filter.View(), m.width)
	case modeRename:
		return truncate(m.rename.View(), m.width)
	case modeConfirm:
		return truncate(styleStatus.Render(m.confirmPrompt), m.width)
	}
	if m.status != "" {
		return truncate(styleStatus.Render(m.status), m.width)
	}
	return truncate(styleDim.Render(keyHints), m.width)
}

// truncate cuts a rendered string to a visible width, adding an ellipsis when
// it had to cut. It measures with lipgloss so ANSI escapes are not counted.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// plural returns the "s" for a count.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -v`
Expected: all PASS. `TestNoLineExceedsTheTerminalWidth` is the one that will
catch a layout mistake — if it fails, print the view and find which element is
not going through `truncate`.

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): render the two-pane layout with adaptive colours"
```

---

### Task 8: Run the browser from `dl`

**Files:**
- Create: `internal/tui/tui.go`
- Modify: `cmd/dl/main.go`
- Test: `cmd/dl/e2e_test.go` (add one case)

- [ ] **Step 1: Write `internal/tui/tui.go`**

```go
package tui

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// Run opens the browser and blocks until the user leaves it.
//
// Anything the final action printed — the note a jump shows — is written to
// out after the alternate screen is torn down, so it is still on screen in the
// directory you land in.
func Run(s Session, out io.Writer) error {
	p := tea.NewProgram(New(s), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	if m, ok := final.(Model); ok && m.finalOutput != "" {
		fmt.Fprint(out, m.finalOutput)
	}
	return nil
}
```

- [ ] **Step 2: Replace the fallback in `cmd/dl/main.go`**

Replace:

```go
	// The interactive browser arrives in task 8. Until then a bare `dl`
	// still falls back to listing.
	if cmd.Action == cli.ActionTUI {
		cmd.Action = cli.ActionSee
	}
	return session.Run(cmd, os.Stdout)
```

with:

```go
	// A bare `dl` opens the browser. It is not a store command, so it does
	// not go through session.Run: the browser drives the session itself, one
	// command at a time.
	if cmd.Action == cli.ActionTUI {
		return tui.Run(session, os.Stdout)
	}
	return session.Run(cmd, os.Stdout)
```

Add `"github.com/Decapix/dl-organisation/internal/tui"` to the imports.

- [ ] **Step 3: Give Session the two accessors the browser needs**

`actions.Session` already has `Cwd` and `Home` *fields*, so the accessors need
different names. Add to `internal/actions/session.go`:

```go
// CurrentDir reports the directory --set would record. It exists so that
// Session satisfies the browser's interface; the browser cannot read the
// struct field because that would make it depend on the concrete type.
func (s *Session) CurrentDir() string { return s.Cwd }

// HomeDir reports the home directory, used to shorten paths for display.
func (s *Session) HomeDir() string { return s.Home }
```

Then assert the two stay in step, in `cmd/dl/main.go` — the assertion lives
here because neither package may import the other:

```go
// actions.Session must satisfy the browser's interface.
var _ tui.Session = (*actions.Session)(nil)
```

- [ ] **Step 4: Add the end-to-end case**

In `cmd/dl/e2e_test.go` (it needs `"time"` in its imports):

```go
// A bare `dl` opens the browser, which wants a terminal. Under `go test`
// there is none. Whether bubbletea errors out or reads EOF and exits cleanly
// is its business; what this test pins is that the binary terminates, does
// not panic, and reports a code we recognise. Asserting a specific code here
// would be asserting a dependency's internals.
func TestBareDlWithoutATerminalTerminates(t *testing.T) {
	setupEnv(t)
	done := make(chan int, 1)
	go func() {
		_, _, code := captureRun(t)
		done <- code
	}()
	select {
	case code := <-done:
		if code != exitOK && code != exitFailure {
			t.Fatalf("exit code = %d, want 0 or 1", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a bare dl hung with no terminal")
	}
}
```

- [ ] **Step 5: Run everything**

Run: `go test ./... -race -count=1 && go vet ./...`
Expected: all PASS, no vet findings

- [ ] **Step 6: Commit**

```bash
git add internal/tui cmd/dl
git commit -m "feat(cmd): open the browser on a bare dl"
```

---

### Task 9: Manual verification and docs

- [ ] **Step 1: Build and install**

```bash
go install ./cmd/dl
```

- [ ] **Step 2: Drive it by hand against a throwaway store**

```bash
DL_DIR=$(mktemp -d) dl
```

Walk the whole keymap and check each one:

| Key | What to confirm |
|-----|-----------------|
| (open) | two panes at a wide terminal, note on the right |
| `a` | the current directory appears as a new slot |
| `j` `k` `g` `G` | the cursor moves, the note pane follows |
| `/` | typing narrows the list; the note text matches too |
| `esc` | the full list is back |
| `n` | the input starts from the current name; enter renames |
| `e` | `$EDITOR` opens on the note, and the pane shows the change on exit |
| `r` then `y` | the name and note clear, the path stays |
| `d` then `n` | nothing is deleted |
| `d` then `y` | the slot goes |
| `?` | the overlay lists every key |
| `⏎` | the shell lands in the directory and the note is printed |
| `q` | the shell has not moved |

Then resize the terminal below 80 and below 60 columns and confirm the layout
degrades rather than wrapping.

- [ ] **Step 3: Confirm the lock is not held while the browser is open**

Open `dl` in one terminal, leave it on the list, and in another terminal run:

```bash
cd /tmp && dl -z 99 -n locktest
```

Expected: it returns immediately. Then press any movement key in the browser
and confirm slot 99 appears after the next reload. Finally `dl -d 99`.

- [ ] **Step 4: Update the README**

Add a **Browser** section after **Usage**: the ASCII layout from the spec, the
key table, and one line saying `dl` with no arguments opens it. Remove the
sentence in **Install** that implies `dl` lists.

- [ ] **Step 5: Run the full suite one more time and push**

```bash
go test ./... -race -count=1 && go vet ./... && gofmt -l .
git add -A && git commit -m "docs: document the browser"
git push
```

---

## Phase 2 done when

- `go test ./... -race` and `go vet ./...` are clean.
- `dl` opens the browser; every key in the table above does what it says.
- A browser left open does not block another shell's `dl -z`.
- Jumping with `⏎` moves the shell and prints the note.
- The layout holds at 40, 60, 80 and 200 columns.

## Deferred to phase 3

Shell completions, goreleaser, the README GIF, and fuzzy filtering if the
substring matcher turns out to be too blunt in daily use.
