package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decapix/dl-organisation/internal/slots"
)

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
		for i, sl := range mm.view {
			if sl.HasNote() {
				mm.cursor = i
				mm.syncNote()
				break
			}
		}
		t.Logf("\n===== %dx%d =====\n%s\n", size.w, size.h, mm.View())
	}
}
