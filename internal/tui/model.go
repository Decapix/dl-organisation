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

// New builds a browser over a session. Call Run rather than using this
// directly, except in tests.
func New(s Session) Model {
	return Model{session: s}
}

// slotExists reports whether a slot is still valid, through the seam when
// there is one.
func (m Model) slotExists(sl slots.Slot) bool {
	if m.exists != nil {
		return m.exists(sl.Path)
	}
	return sl.Exists()
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

// applyFilter recomputes view from slots. Filtering arrives in the next task;
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
