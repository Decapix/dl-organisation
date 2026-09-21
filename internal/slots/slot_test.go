package slots

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisplayNameUsesNameWhenSet(t *testing.T) {
	s := Slot{Number: 7, Name: "exam42", Path: "/home/u/work/level1"}
	if got := s.DisplayName(); got != "exam42" {
		t.Fatalf("DisplayName() = %q, want %q", got, "exam42")
	}
}

func TestDisplayNameFallsBackToBasename(t *testing.T) {
	s := Slot{Number: 7, Path: "/home/u/work/level1"}
	if got := s.DisplayName(); got != "level1" {
		t.Fatalf("DisplayName() = %q, want %q", got, "level1")
	}
}

func TestHasNote(t *testing.T) {
	cases := []struct {
		note string
		want bool
	}{
		{"", false},
		{"   \n\t ", false}, // whitespace only does not count as a note
		{"fix the retry loop", true},
	}
	for _, c := range cases {
		s := Slot{Note: c.note}
		if got := s.HasNote(); got != c.want {
			t.Errorf("HasNote(%q) = %v, want %v", c.note, got, c.want)
		}
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	if got := (Slot{Path: dir}).Exists(); !got {
		t.Error("Exists() on a real directory = false, want true")
	}
	if got := (Slot{Path: filepath.Join(dir, "gone")}).Exists(); got {
		t.Error("Exists() on a missing directory = true, want false")
	}
	// A regular file is not a valid bookmark target.
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (Slot{Path: f}).Exists(); got {
		t.Error("Exists() on a regular file = true, want false")
	}
}

func TestShortPathCollapsesHome(t *testing.T) {
	cases := []struct{ in, home, want string }{
		{"/home/u/work", "/home/u", "~/work"},
		{"/home/u", "/home/u", "~"},
		{"/etc/hosts", "/home/u", "/etc/hosts"},
		{"/home/user2/x", "/home/u", "/home/user2/x"}, // prefix must end at a separator
	}
	for _, c := range cases {
		if got := ShortPath(c.in, c.home); got != c.want {
			t.Errorf("ShortPath(%q, %q) = %q, want %q", c.in, c.home, got, c.want)
		}
	}
}

func TestNoteFirstLine(t *testing.T) {
	s := Slot{Note: "\n\nscrape the listing pages\npagination stops at p.4\n"}
	if got := s.NoteFirstLine(); got != "scrape the listing pages" {
		t.Fatalf("NoteFirstLine() = %q", got)
	}
	if got := (Slot{}).NoteFirstLine(); got != "" {
		t.Fatalf("NoteFirstLine() on empty note = %q, want empty", got)
	}
}
