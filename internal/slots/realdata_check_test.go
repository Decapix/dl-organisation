package slots

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRealDataMigration is a one-off check against a copy of the author's real
// ~/.cdl. It skips unless DL_REAL_CDL points at a home directory containing one.
func TestRealDataMigration(t *testing.T) {
	home := os.Getenv("DL_REAL_CDL")
	if home == "" {
		t.Skip("DL_REAL_CDL not set")
	}
	dir := filepath.Join(t.TempDir(), "dl")
	n, err := Migrate(home, dir)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("imported %d slots", n)
	for _, s := range st.All() {
		t.Logf("%3d  %-12s %s", s.Number, s.DisplayName(), s.Path)
		if s.HasNote() {
			t.Logf("       note: %q", s.Note)
		}
	}
}
