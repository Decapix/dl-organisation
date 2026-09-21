package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain pins the colour profile so that every rendering test compares
// plain text. Colour is exercised on purpose in TestTheKeysAreStyled below.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// footerKeys is what the always-visible line has to carry. It is not every
// bound key — the line is already 77 columns — but it is every key you would
// not otherwise discover. `o organize` shipped missing from it once, which is
// why this table exists.
var footerKeys = []struct {
	key  string
	word string
}{
	{"⏎", "cd"},
	{"/", "find"},
	{"e", "edit"},
	{"n", "name"},
	{"o", "organize"},
	{"a", "add"},
	{"d", "del"},
	{"c", "compact"},
	{"?", "help"},
}

func TestEveryFooterKeyIsInTheHints(t *testing.T) {
	got := renderHints()
	for _, b := range footerKeys {
		if !strings.Contains(got, b.key+" "+b.word) {
			t.Errorf("the hints line is missing %q %q:\n%s", b.key, b.word, got)
		}
	}
}

// The overlay is the complete list, so it must cover the keys the footer has
// no room for as well as the ones it shows.
func TestEveryBoundKeyIsInTheHelpOverlay(t *testing.T) {
	bound := []string{"⏎", "e", "n", "r", "d", "o", "c", "/", "a", "?", "j k", "g G", "ctrl-u", "q"}
	got := renderHelpOverlay()
	for _, k := range bound {
		if !strings.Contains(got, k) {
			t.Errorf("the help overlay is missing %q:\n%s", k, got)
		}
	}
}

// The hints must fit a standard terminal, or the last keys — which include
// the one that opens the full list — get truncated away.
func TestTheHintsFitEightyColumns(t *testing.T) {
	if w := visibleWidth(renderHints()); w > 80 {
		t.Fatalf("the hints line is %d columns:\n%s", w, renderHints())
	}
}

// The overlay must fit too, or its right-hand column wraps.
func TestTheHelpOverlayFitsEightyColumns(t *testing.T) {
	for _, line := range strings.Split(renderHelpOverlay(), "\n") {
		if w := visibleWidth(line); w > 80 {
			t.Fatalf("an overlay line is %d columns: %q", w, line)
		}
	}
}

// The point of the exercise: a key must render differently from its label,
// or the line is one undifferentiated grey run.
func TestTheKeysAreStyled(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	got := renderHints()
	if !strings.Contains(got, "\x1b[") {
		t.Fatal("the hints line carries no styling at all")
	}
	// The key and its label must not share one escape sequence, which is what
	// would happen if the whole line were styled in one go.
	key := styleKey.Render("e")
	if !strings.Contains(got, key) {
		t.Errorf("the key %q is not styled as a key:\n%q", "e", got)
	}
	if strings.Contains(got, styleKey.Render("e edit")) {
		t.Error("the key and its label share a style; the eye cannot separate them")
	}
}

// Styling must not change the width, or the 80-column budget is a fiction.
func TestStylingDoesNotChangeTheWidth(t *testing.T) {
	plain := visibleWidth(renderHints())
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	if coloured := visibleWidth(renderHints()); coloured != plain {
		t.Fatalf("width is %d plain and %d coloured", plain, coloured)
	}
}
