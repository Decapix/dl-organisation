package slots

import (
	"errors"
	"strings"
	"testing"
)

func refFixture(t *testing.T) *Store {
	t.Helper()
	st := openTestStore(t)
	st.Put(Slot{Number: 7, Name: "exam42", Path: "/tmp/exam"})
	st.Put(Slot{Number: 12, Name: "scraping", Path: "/tmp/scrape"})
	st.Put(Slot{Number: 9, Name: "scan", Path: "/tmp/scan"})
	st.Put(Slot{Number: 3, Path: "/tmp/unnamed"})
	return st
}

func TestResolveByNumber(t *testing.T) {
	st := refFixture(t)
	got, err := st.Resolve("7")
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 7 {
		t.Fatalf("Resolve(\"7\").Number = %d, want 7", got.Number)
	}
}

// A numeric ref must never fall through to name matching, so a slot named "42"
// cannot shadow slot 42.
func TestNumericRefNeverMatchesAName(t *testing.T) {
	st := refFixture(t)
	st.Put(Slot{Number: 5, Name: "42", Path: "/tmp/trap"})
	if _, err := st.Resolve("42"); err == nil {
		t.Fatal("Resolve(\"42\") found something; slot 42 is empty so it must fail")
	}
}

func TestResolveByExactName(t *testing.T) {
	st := refFixture(t)
	got, err := st.Resolve("EXAM42") // matching is case-insensitive
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 7 {
		t.Fatalf("Number = %d, want 7", got.Number)
	}
}

func TestResolveByUniquePrefix(t *testing.T) {
	st := refFixture(t)
	got, err := st.Resolve("e")
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 7 {
		t.Fatalf("Number = %d, want 7", got.Number)
	}
}

func TestResolveAmbiguousPrefixListsCandidates(t *testing.T) {
	st := refFixture(t)
	_, err := st.Resolve("sc") // scraping and scan both match
	if err == nil {
		t.Fatal("Resolve(\"sc\") = nil error, want an ambiguity error")
	}
	var amb *AmbiguousRefError
	if !errors.As(err, &amb) {
		t.Fatalf("error type = %T, want *AmbiguousRefError", err)
	}
	if len(amb.Candidates) != 2 {
		t.Fatalf("Candidates = %v, want 2 entries", amb.Candidates)
	}
	msg := err.Error()
	for _, want := range []string{"scraping", "scan"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q does not mention %q", msg, want)
		}
	}
}

// An exact name wins over a prefix, otherwise naming a slot "s" would make it
// unreachable as soon as a second s-name appeared.
func TestExactNameBeatsPrefix(t *testing.T) {
	st := refFixture(t)
	st.Put(Slot{Number: 20, Name: "sc", Path: "/tmp/sc"})
	got, err := st.Resolve("sc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 20 {
		t.Fatalf("Number = %d, want 20", got.Number)
	}
}

func TestResolveUnknown(t *testing.T) {
	st := refFixture(t)
	if _, err := st.Resolve("nope"); err == nil {
		t.Fatal("Resolve(\"nope\") = nil error, want not-found")
	}
}

func TestResolveEmptySlotNumber(t *testing.T) {
	st := refFixture(t)
	_, err := st.Resolve("500")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("Resolve(\"500\") error = %v, want an \"empty\" message", err)
	}
}

func TestParseSlotNumber(t *testing.T) {
	cases := []struct {
		in string
		n  int
		ok bool
	}{
		{"7", 7, true},
		{"1000", 1000, true},
		{"0", 0, false},
		{"1001", 0, false},
		{"-3", 0, false},
		{"7a", 0, false},
		{"", 0, false},
		{"007", 7, true},
	}
	for _, c := range cases {
		n, ok := ParseSlotNumber(c.in)
		if ok != c.ok || (ok && n != c.n) {
			t.Errorf("ParseSlotNumber(%q) = (%d, %v), want (%d, %v)", c.in, n, ok, c.n, c.ok)
		}
	}
}
