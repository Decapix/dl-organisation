package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

// TestRenderHelp prints the ? overlay so its layout can be looked at.
func TestRenderHelp(t *testing.T) {
	if os.Getenv("DL_RENDER_HELP") == "" {
		t.Skip("DL_RENDER_HELP not set")
	}
	t.Logf("\n%s\n", renderHelpOverlay())
}

// TestRenderOrganize prints the browser mid-move, so the held marker and the
// footer can be looked at.
func TestRenderOrganize(t *testing.T) {
	if os.Getenv("DL_RENDER_ORGANIZE") == "" {
		t.Skip("DL_RENDER_ORGANIZE not set")
	}
	f := &fakeSession{home: "/home/u", slots: []slots.Slot{
		{Number: 1, Name: "coloriage", Path: "/home/u/work/coloriage-app/svg"},
		{Number: 2, Name: "cpp08", Path: "/home/u/42/code/cpp/cpp08", Note: "cpp08"},
		{Number: 3, Name: "ir-landing", Path: "/home/u/style-site/new-landing-ir"},
		{Number: 4, Name: "exam42", Path: "/home/u/42/exams/exam5/s4/level1", Note: "after ex04"},
	}}
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	u, _ := m.Update(tea.WindowSizeMsg{Width: 88, Height: 12})
	mm := u.(Model)

	step := func(label string, keys ...tea.KeyMsg) {
		var tm tea.Model = mm
		for _, k := range keys {
			tm, _ = tm.Update(k)
		}
		mm = tm.(Model)
		t.Logf("\n--- %s ---\n%s\n", label, mm.View())
	}
	sp := tea.KeyMsg{Type: tea.KeySpace}
	r := func(c rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}} }

	step("G then o", r('G'), r('o'))
	step("space picks slot 4 up", sp)
	step("k k moves to slot 2", r('k'), r('k'))
}

// TestRenderHoles prints a store with gaps in its numbering, so the empty
// rows and the collapsed run can be looked at.
func TestRenderHoles(t *testing.T) {
	if os.Getenv("DL_RENDER_HOLES") == "" {
		t.Skip("DL_RENDER_HOLES not set")
	}
	f := &fakeSession{home: "/home/u", slots: []slots.Slot{
		{Number: 1, Name: "coloriage", Path: "/home/u/work/coloriage-app/svg"},
		{Number: 2, Name: "cpp08", Path: "/home/u/42/code/cpp/cpp08", Note: "cpp08"},
		{Number: 4, Name: "ir-landing", Path: "/home/u/style-site/new-landing-ir", Note: "IR landing page"},
		{Number: 7, Name: "exam42", Path: "/home/u/42/exams/exam5/s4/level1", Note: "continue after ex04"},
		{Number: 40, Name: "far", Path: "/home/u/somewhere/far"},
	}}
	m := New(f)
	m.exists = func(string) bool { return true }
	m.slots = f.slots
	m.applyFilter()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 92, Height: 16})
	mm := updated.(Model)
	mm.cursor = 2 // the empty slot 3
	mm.syncNote()
	t.Logf("\n%s\n", mm.View())
}

// TestRenderRealStore prints the browser as it would appear over a real store.
// It is a look-at-it check, not an assertion; it skips unless DL_RENDER_DIR
// points at a store directory.
func TestRenderRealStore(t *testing.T) {
	dir := os.Getenv("DL_RENDER_DIR")
	if dir == "" {
		t.Skip("DL_RENDER_DIR not set")
	}
	st, err := slots.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	f := &fakeSession{slots: st.All(), home: home}

	for _, size := range []struct{ w, h int }{{100, 20}, {70, 16}, {50, 10}} {
		m := New(f)
		m.slots = f.slots
		m.applyFilter()
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		mm := updated.(Model)
		// Put the cursor on a slot that has a note.
		for i, r := range mm.view {
			if r.slot != nil && r.slot.HasNote() {
				mm.cursor = i
				mm.syncNote()
				break
			}
		}
		t.Logf("\n===== %dx%d =====\n%s\n", size.w, size.h, mm.View())
	}
}
