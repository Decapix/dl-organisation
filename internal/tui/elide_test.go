package tui

import "testing"

func TestElidePath(t *testing.T) {
	const p = "~/Tocuments/wbiz/coloriage-app/code4-reconciliation/coloriage/svg"
	cases := []struct {
		max  int
		want string
	}{
		// Wide enough: untouched.
		{len(p), p},
		{100, p},
		// Narrower: drop leading components, keep the tail, which is the part
		// that identifies the project.
		{30, "~/…/coloriage/svg"},
		{20, "~/…/coloriage/svg"},
		{16, "~/…/svg"},
		// Too narrow for even one component: cut the tail as a last resort.
		{5, "~/…/s"},
		{1, "~"},
		{0, ""},
	}
	for _, c := range cases {
		if got := elidePath(p, c.max); got != c.want {
			t.Errorf("elidePath(max=%d) = %q, want %q", c.max, got, c.want)
		}
	}
}

// Whatever the width, the result must never exceed it.
func TestElidePathNeverExceedsTheBudget(t *testing.T) {
	paths := []string{
		"~/Tocuments/wbiz/coloriage-app/code4-reconciliation/coloriage/svg",
		"~",
		"/",
		"/one",
		"~/a/b/c",
		"/home/solenopsis/Tocuments/wpath/42/exams/exam5/solutions/s4/level1",
		"~/a-directory-with-one-very-long-single-component-name-and-nothing-else",
	}
	for _, p := range paths {
		for max := 0; max <= len(p)+2; max++ {
			if got := elidePath(p, max); len([]rune(got)) > max {
				t.Errorf("elidePath(%q, %d) = %q (%d runes), over budget", p, max, got, len([]rune(got)))
			}
		}
	}
}

// An absolute path keeps its leading slash rather than growing a "~".
func TestElidePathKeepsAnAbsoluteRoot(t *testing.T) {
	got := elidePath("/var/lib/docker/volumes/project/data", 20)
	if got[0] != '/' {
		t.Errorf("elidePath = %q, want it to start with /", got)
	}
	if !contains(got, "data") {
		t.Errorf("elidePath = %q, want the tail kept", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
