// Package tui is dl's interactive browser. It owns no behaviour of its own:
// every key that changes something builds a cli.Command and hands it to the
// same actions the command line uses. Model.Update is a pure function of
// state and message, which is what makes the whole interaction testable
// without a terminal.
package tui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	modeFilter
	modeRename
	modeConfirm
	modeHelp
)

// Layout constants. Below wideMin the note moves under the list; below
// narrowMin it is dropped entirely.
const (
	wideMin   = 80
	narrowMin = 60

	// header, its blank line, and the footer's two rows (status and hints).
	chromeRows = 4
)

// listPaneWidth is how wide the list pane gets. The list takes the larger
// share because a truncated path is harder to recognise than a wrapped note.
func listPaneWidth(total int) int {
	if total >= wideMin {
		return total * 55 / 100
	}
	return total - 2
}

// notePaneWidth is the whole width of the note block, border and padding
// included.
func notePaneWidth(total int) int {
	w := total - 2
	if total >= wideMin {
		w = total - listPaneWidth(total) - 3 // gap, border and padding
	}
	if w < 1 {
		return 1
	}
	return w
}

// noteChrome is the columns notePaneWidth spends on the left border and the
// padding beside it.
const noteChrome = 2

// noteContentWidth is how wide the viewport inside the note block may be.
// Setting the viewport to notePaneWidth instead would overflow the block by
// exactly these two columns, wrapping every full-width line and pushing the
// body one row taller than the layout budgeted for.
func noteContentWidth(total int) int {
	if total < wideMin {
		return notePaneWidth(total) // stacked: no border, no padding
	}
	w := notePaneWidth(total) - noteChrome
	if w < 1 {
		return 1
	}
	return w
}

// bodyRows is how many rows the list and the note have to share, after the
// header, its blank line and the footer are taken out.
func bodyRows(height int) int {
	h := height - chromeRows
	if h < 1 {
		return 1
	}
	return h
}

// listRows is how many slot rows fit on screen. Side by side the list gets
// the whole body; stacked it gets the larger share, because the list is what
// you steer with and the note can be scrolled.
func listRows(width, height int) int {
	body := bodyRows(height)
	if width >= wideMin || width < narrowMin {
		return body
	}
	rows := body * 60 / 100
	if rows < 1 {
		return 1
	}
	return rows
}

// notePaneHeight is how many rows the note pane gets.
func notePaneHeight(width, height int) int {
	if width < narrowMin {
		return 0 // no note pane at all
	}
	if width >= wideMin {
		return bodyRows(height)
	}
	h := bodyRows(height) - listRows(width, height)
	if h < 1 {
		return 1
	}
	return h
}

// Model is the browser's whole state.
type Model struct {
	session Session

	slots  []slots.Slot // everything the store holds
	view   []row        // the lines on screen: slots, and the holes between them
	cursor int          // index into view

	// top is the first row of view that is on screen. The list is windowed
	// rather than drawn whole: a full-screen program that renders more lines
	// than the terminal has scrolls its own footer away.
	top int

	mode   mode
	status string // the last action's output or error, shown in the footer

	// filter is the query typed after `/`. It stays applied when the user
	// returns to the list, so you can filter and then act on what is left.
	filter textinput.Model

	// note is the right-hand pane. It is a viewport so that a note longer
	// than the screen can be scrolled rather than truncated.
	note viewport.Model

	// rename is the inline input shown by `n`.
	rename textinput.Model

	// confirmPrompt is the question shown in modeConfirm, and confirmCmd is
	// what answering yes would run. A single keystroke destroying something
	// deserves a question; a typed command line does not.
	confirmPrompt string
	confirmCmd    cli.Command

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
	in := textinput.New()
	in.Prompt = "/"
	in.Placeholder = "name, path or note"

	ren := textinput.New()
	ren.Prompt = "name: "
	ren.CharLimit = 40

	return Model{session: s, filter: in, rename: ren}
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

// selectedRow is the line under the cursor. Its zero value is a row covering
// no slot number, which every caller treats as "nothing here".
func (m Model) selectedRow() row {
	if m.cursor < 0 || m.cursor >= len(m.view) {
		return row{}
	}
	return m.view[m.cursor]
}

// selected is the slot under the cursor, or nil when the cursor is on an
// empty row or the list is empty. Returning nil for an empty row is what
// makes every slot key a no-op there without each one having to check.
func (m Model) selected() *slots.Slot {
	return m.selectedRow().slot
}

// slotCount is how many real slots the view holds, which is what the header
// reports: an empty row is not a slot.
func (m Model) slotCount() int {
	n := 0
	for _, r := range m.view {
		if r.slot != nil {
			n++
		}
	}
	return n
}

// applyFilter recomputes view from slots and the current query. With no query
// the holes in the numbering are shown; with one they are not, because
// filtering is for finding something and an empty slot is not something.
func (m *Model) applyFilter() {
	if q := m.filter.Value(); q != "" {
		m.view = slotRows(filterSlots(m.slots, q))
	} else {
		m.view = buildRows(m.slots)
	}
	m.clampCursor()
	m.scrollToCursor()
	m.syncNote()
}

// syncNote points the note pane at the selected slot and rewinds it to the
// top, so moving to another slot never lands you halfway down its note.
func (m *Model) syncNote() {
	r := m.selectedRow()
	switch {
	case r.slot != nil:
		m.note.SetContent(r.slot.Note)
	case r.collapsed():
		m.note.SetContent(fmt.Sprintf("slots %d to %d are empty", r.from, r.to))
	case r.from > 0:
		m.note.SetContent(fmt.Sprintf("slot %d is empty\n\npress a to save the current directory here", r.from))
	default:
		m.note.SetContent("")
	}
	m.note.GotoTop()
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

// scrollToCursor moves the window the minimum distance needed to bring the
// cursor back on screen, so stepping through a long list never jumps.
func (m *Model) scrollToCursor() {
	rows := listRows(m.width, m.height)
	if rows < 1 {
		rows = 1
	}
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+rows {
		m.top = m.cursor - rows + 1
	}
	// A shorter list than the window means there is nothing to scroll past.
	if max := len(m.view) - rows; m.top > max {
		m.top = max
	}
	if m.top < 0 {
		m.top = 0
	}
}

// Update is the state machine. Every branch returns a new Model; nothing is
// mutated through a pointer receiver from here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.note.Width = noteContentWidth(msg.Width)
		m.note.Height = notePaneHeight(msg.Width, msg.Height)
		m.scrollToCursor()
		m.syncNote()
		return m, nil

	case reloadMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.slots = msg.slots
		m.applyFilter()
		return m, nil

	case editFinishedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = "note saved"
		return m, m.reload()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes a keypress to the current mode.
//
// It clears the status first: a message about something you did three
// keystrokes ago is noise. An action that sets a new one does so after this
// point, so a key never wipes its own message.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if m.mode == modeHelp {
		m.mode = modeList // any key closes the overlay
		return m, nil
	}
	switch m.mode {
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeRename:
		return m.handleRenameKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
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
		m.scrollToCursor()
		m.syncNote()
		return m, nil
	case "down", "j":
		if m.cursor < len(m.view)-1 {
			m.cursor++
		}
		m.scrollToCursor()
		m.syncNote()
		return m, nil
	case "g", "home":
		m.cursor = 0
		m.scrollToCursor()
		m.syncNote()
		return m, nil
	case "G", "end":
		m.cursor = len(m.view) - 1
		m.clampCursor()
		m.scrollToCursor()
		m.syncNote()
		return m, nil

	case "ctrl+d", "pgdown":
		m.note.HalfViewDown()
		return m, nil
	case "ctrl+u", "pgup":
		m.note.HalfViewUp()
		return m, nil

	case "a":
		// On a single empty row, save into that slot: showing a free number
		// is only half the value, filling it is the other half. Anywhere
		// else --set picks the lowest free slot.
		if r := m.selectedRow(); r.empty() && !r.collapsed() && r.from > 0 {
			return m.run(cli.Command{Action: cli.ActionSet, Ref: strconv.Itoa(r.from)})
		}
		return m.run(cli.Command{Action: cli.ActionSet})

	case "d":
		sl := m.selected()
		if sl == nil {
			m.status = m.emptyRowMessage("delete")
			return m, nil
		}
		m.mode = modeConfirm
		m.confirmPrompt = fmt.Sprintf("delete slot %d (%s)? [y/N]", sl.Number, sl.DisplayName())
		m.confirmCmd = cli.Command{Action: cli.ActionDelete, Ref: strconv.Itoa(sl.Number)}
		return m, nil

	case "r":
		sl := m.selected()
		if sl == nil {
			m.status = m.emptyRowMessage("clear")
			return m, nil
		}
		m.mode = modeConfirm
		m.confirmPrompt = fmt.Sprintf("clear slot %d's name and note? [y/N]", sl.Number)
		m.confirmCmd = cli.Command{Action: cli.ActionReset, Ref: strconv.Itoa(sl.Number)}
		return m, nil

	case "e":
		sl := m.selected()
		if sl == nil {
			m.status = m.emptyRowMessage("edit")
			return m, nil
		}
		return m, tea.Exec(
			&editCommand{session: m.session, ref: strconv.Itoa(sl.Number)},
			func(err error) tea.Msg { return editFinishedMsg{err: err} },
		)

	case "n":
		sl := m.selected()
		if sl == nil {
			m.status = m.emptyRowMessage("rename")
			return m, nil
		}
		m.mode = modeRename
		m.rename.SetValue(sl.Name)
		m.rename.CursorEnd()
		m.rename.Focus()
		return m, textinput.Blink

	case "/":
		m.mode = modeFilter
		m.filter.Focus()
		return m, textinput.Blink

	case "?":
		m.mode = modeHelp
		return m, nil

	case "enter":
		return m.jump()
	}
	return m, nil
}

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

// handleConfirmKey takes the y/N answer. Anything but y cancels, so a
// mistyped key is always the safe outcome.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeList
	pending := m.confirmCmd
	m.confirmPrompt = ""
	m.confirmCmd = cli.Command{}
	if msg.String() != "y" && msg.String() != "Y" {
		m.status = "cancelled"
		return m, nil
	}
	return m.run(pending)
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

// emptyRowMessage explains why a slot key did nothing.
func (m Model) emptyRowMessage(verb string) string {
	r := m.selectedRow()
	switch {
	case r.collapsed():
		return fmt.Sprintf("slots %d to %d are empty — nothing to %s", r.from, r.to, verb)
	case r.from > 0:
		return fmt.Sprintf("slot %d is empty — nothing to %s", r.from, verb)
	default:
		return ""
	}
}

// jump runs --cd on the selected slot and quits. On failure it stays open and
// reports why, because an error the user can act on is worth a screen.
func (m Model) jump() (tea.Model, tea.Cmd) {
	sl := m.selected()
	if sl == nil {
		m.status = m.emptyRowMessage("jump to")
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
