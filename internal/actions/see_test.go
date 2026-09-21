package actions

import (
	"testing"

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
