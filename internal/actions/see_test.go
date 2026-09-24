package actions

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestSeeEmptyStore(t *testing.T) {
	env, out, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	want := "no slots yet — save one with: dl -z\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

// allPathsExist pins the marker column so a layout test asserts on formatting
// rather than on what happens to be on disk.
func allPathsExist(env *Env) { env.Exists = func(string) bool { return true } }

func TestSeeLayout(t *testing.T) {
	env, out, _ := testEnv(t)
	allPathsExist(env)
	// Home is /home/u in testEnv, so these paths shorten to ~/...
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/home/u/work", Note: "a note"})
	env.Store.Put(slots.Slot{Number: 12, Name: "scraping", Path: "/home/u/scrape"})

	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	want := "2 slots\n\n" +
		"  *  7  exam42    ~/work\n" +
		"    12  scraping  ~/scrape\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestSeeSingularHeader(t *testing.T) {
	env, out, _ := testEnv(t)
	allPathsExist(env)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/home/u"})
	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	want := "1 slot\n\n" +
		"    1  a  ~\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestSeeMarksADeadPath(t *testing.T) {
	env, out, _ := testEnv(t)
	// A note AND a dead path: the dead marker wins.
	env.Exists = func(p string) bool { return p != "/gone" }
	env.Store.Put(slots.Slot{Number: 1, Name: "gone", Path: "/gone", Note: "n"})
	env.Store.Put(slots.Slot{Number: 2, Name: "here", Path: "/home/u/here"})

	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	lines := splitLines(out.String())
	if lines[2][2] != 'x' {
		t.Fatalf("row 1 = %q, want an x marker", lines[2])
	}
	if lines[3][2] != ' ' {
		t.Fatalf("row 2 = %q, want no marker", lines[3])
	}
}

func TestSeeFallsBackToTheBasenameForUnnamedSlots(t *testing.T) {
	env, out, _ := testEnv(t)
	allPathsExist(env)
	env.Store.Put(slots.Slot{Number: 1, Path: "/home/u/projects/scraper"})
	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	want := "1 slot\n\n" +
		"    1  scraper  ~/projects/scraper\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestSeeLong(t *testing.T) {
	env, out, _ := testEnv(t)
	allPathsExist(env)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/home/u/a", Note: "line one\nline two"})
	env.Store.Put(slots.Slot{Number: 2, Name: "b", Path: "/home/u/b"})

	if err := Run(env, cli.Command{Action: cli.ActionSee, Long: true}); err != nil {
		t.Fatal(err)
	}
	want := "2 slots\n\n" +
		"  * 1  a  ~/a\n" +
		"       line one\n" +
		"       line two\n" +
		"\n" +
		"    2  b  ~/b\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

// A blank line inside a note must not pick up the indent, or every paragraph
// break would carry trailing whitespace.
func TestSeeLongDoesNotIndentBlankNoteLines(t *testing.T) {
	env, out, _ := testEnv(t)
	allPathsExist(env)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/home/u/a", Note: "one\n\ntwo"})

	if err := Run(env, cli.Command{Action: cli.ActionSee, Long: true}); err != nil {
		t.Fatal(err)
	}
	want := "1 slot\n\n" +
		"  * 1  a  ~/a\n" +
		"       one\n" +
		"\n" +
		"       two\n"
	if out.String() != want {
		t.Fatalf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestSeeQuietPrintsFullPathsOnly(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 3, Name: "c", Path: "/home/u/c", Note: "n"})
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/home/u/a"})

	if err := Run(env, cli.Command{Action: cli.ActionSee, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	// Full paths, not shortened: the output is meant to be piped.
	want := "/home/u/a\n/home/u/c\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestSeeQuietOnAnEmptyStorePrintsNothing(t *testing.T) {
	env, out, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionSee, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("output = %q, want nothing so that pipes see an empty stream", out.String())
	}
}

// splitLines keeps trailing empties out of the way for index-based assertions.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return lines
}

// colourEnv gives the env a renderer that always emits colour, so a test can
// see the escape sequences a real terminal would get.
func colourEnv(t *testing.T) (*Env, *bytes.Buffer) {
	t.Helper()
	env, out, _ := testEnv(t)
	allPathsExist(env)
	r := lipgloss.NewRenderer(out)
	r.SetColorProfile(termenv.TrueColor)
	env.Renderer = r
	return env, out
}

// The rows of a listing alternate between two colours, so the eye can follow
// a row across the columns. The colour covers the whole line.
func TestSeeStripesRowsWithTwoColours(t *testing.T) {
	env, out := colourEnv(t)
	for _, n := range []int{1, 2, 3} {
		env.Store.Put(slots.Slot{Number: n, Name: "s", Path: "/home/u"})
	}
	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	rows := lines[2:] // after "3 slots" and the blank line
	if len(rows) != 3 {
		t.Fatalf("got %d rows: %q", len(rows), rows)
	}
	first, second, third := colourOf(t, rows[0]), colourOf(t, rows[1]), colourOf(t, rows[2])
	if first == second {
		t.Fatalf("rows 1 and 2 share a colour %q; want them to differ", first)
	}
	if first != third {
		t.Fatalf("row 3 = %q, want it back to row 1's colour %q", third, first)
	}
	for _, row := range rows {
		if !strings.HasPrefix(row, "\x1b[") {
			t.Fatalf("row %q does not start with the colour; want the whole line coloured", row)
		}
	}
}

// In long form the note lines belong to their row and take its colour, so a
// row and its note read as one block.
func TestSeeLongStripesRowAndNoteTogether(t *testing.T) {
	env, out := colourEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/home/u", Note: "line one\nline two"})
	env.Store.Put(slots.Slot{Number: 2, Name: "b", Path: "/home/u", Note: "other"})
	if err := Run(env, cli.Command{Action: cli.ActionSee, Long: true}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")[2:]
	// row 1, note, note, blank, row 2, note
	if len(lines) != 6 {
		t.Fatalf("got %d lines: %q", len(lines), lines)
	}
	rowA := colourOf(t, lines[0])
	if colourOf(t, lines[1]) != rowA || colourOf(t, lines[2]) != rowA {
		t.Fatalf("note lines %q %q do not share their row's colour %q", lines[1], lines[2], rowA)
	}
	if lines[3] != "" {
		t.Fatalf("separator = %q, want a bare blank line", lines[3])
	}
	if colourOf(t, lines[4]) == rowA || colourOf(t, lines[5]) != colourOf(t, lines[4]) {
		t.Fatalf("second block %q %q should share a colour distinct from %q", lines[4], lines[5], rowA)
	}
}

// Without a renderer the output is plain text; every other see test relies
// on that, this one just says so out loud.
func TestSeeIsPlainWithoutARenderer(t *testing.T) {
	env, out, _ := testEnv(t)
	allPathsExist(env)
	env.Store.Put(slots.Slot{Number: 1, Name: "a", Path: "/home/u"})
	if err := Run(env, cli.Command{Action: cli.ActionSee}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("output %q carries escape sequences without a renderer", out.String())
	}
}

// colourOf is the leading SGR sequence of a line.
func colourOf(t *testing.T, line string) string {
	t.Helper()
	if !strings.HasPrefix(line, "\x1b[") {
		t.Fatalf("line %q has no leading colour", line)
	}
	end := strings.IndexByte(line, 'm')
	if end < 0 {
		t.Fatalf("line %q: unterminated escape", line)
	}
	return line[:end+1]
}
