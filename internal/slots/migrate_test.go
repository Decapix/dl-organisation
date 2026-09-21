package slots

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLegacy builds a ~/.cdl tree shaped like the real one: a slots.json
// keyed by slot number with path/comment/updated, plus descriptions/<N>.txt.
func writeLegacy(t *testing.T, home string) string {
	t.Helper()
	cdl := filepath.Join(home, ".cdl")
	if err := os.MkdirAll(filepath.Join(cdl, "descriptions"), 0o755); err != nil {
		t.Fatal(err)
	}
	json := `{
  "7": {"path": "/tmp/exam", "comment": "entrainement exam 5 level2", "updated": "2026-09-15 12:07:45"},
  "8": {"path": "/tmp/home", "comment": "", "updated": "2026-09-21 13:02:32"},
  "11": {"path": "/tmp/inception", "comment": "42 inception", "updated": "2026-08-28 16:52:06"}
}`
	if err := os.WriteFile(filepath.Join(cdl, "slots.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	desc := "\n\non fait revision-plan.md\n\nla on a fait jusque ex04\n"
	if err := os.WriteFile(filepath.Join(cdl, "descriptions", "7.txt"), []byte(desc), 0o644); err != nil {
		t.Fatal(err)
	}
	return cdl
}

func TestMigrateMergesCommentAndDescription(t *testing.T) {
	home := t.TempDir()
	writeLegacy(t, home)
	dir := filepath.Join(home, ".config", "dl")

	n, err := Migrate(home, dir)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if n != 3 {
		t.Fatalf("Migrate imported %d slots, want 3", n)
	}

	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := st.Get(7)
	if !ok {
		t.Fatal("slot 7 missing after migration")
	}
	if got.Path != "/tmp/exam" {
		t.Errorf("Path = %q", got.Path)
	}
	if got.Name != "" {
		t.Errorf("Name = %q, want empty (the user fills it in later)", got.Name)
	}
	// The comment becomes the first line, the description follows after a
	// blank line.
	wantNote := "entrainement exam 5 level2\n\non fait revision-plan.md\n\nla on a fait jusque ex04"
	if got.Note != wantNote {
		t.Errorf("Note =\n%q\nwant\n%q", got.Note, wantNote)
	}
	if got.Updated.IsZero() {
		t.Error("Updated is zero; the legacy timestamp was not parsed")
	}
}

func TestMigrateSlotWithCommentOnly(t *testing.T) {
	home := t.TempDir()
	writeLegacy(t, home)
	dir := filepath.Join(home, ".config", "dl")
	if _, err := Migrate(home, dir); err != nil {
		t.Fatal(err)
	}
	st, _ := Open(dir)
	got, _ := st.Get(11)
	if got.Note != "42 inception" {
		t.Errorf("Note = %q, want %q", got.Note, "42 inception")
	}
}

func TestMigrateSlotWithNeitherCommentNorDescription(t *testing.T) {
	home := t.TempDir()
	writeLegacy(t, home)
	dir := filepath.Join(home, ".config", "dl")
	if _, err := Migrate(home, dir); err != nil {
		t.Fatal(err)
	}
	st, _ := Open(dir)
	got, _ := st.Get(8)
	if got.Note != "" {
		t.Errorf("Note = %q, want empty", got.Note)
	}
}

func TestMigrateRenamesTheLegacyDirectoryRatherThanDeletingIt(t *testing.T) {
	home := t.TempDir()
	cdl := writeLegacy(t, home)
	dir := filepath.Join(home, ".config", "dl")
	if _, err := Migrate(home, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cdl); !os.IsNotExist(err) {
		t.Error("~/.cdl still exists; it should have been renamed")
	}
	if _, err := os.Stat(cdl + ".bak"); err != nil {
		t.Errorf("~/.cdl.bak missing: %v", err)
	}
}

func TestMigrateIsSkippedWhenTheNewStoreExists(t *testing.T) {
	home := t.TempDir()
	writeLegacy(t, home)
	dir := filepath.Join(home, ".config", "dl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StoreFileName), []byte(`{"version":1,"slots":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := Migrate(home, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("Migrate imported %d slots, want 0 when a store already exists", n)
	}
}

func TestMigrateIsANoOpWithoutALegacyDirectory(t *testing.T) {
	home := t.TempDir()
	n, err := Migrate(home, filepath.Join(home, ".config", "dl"))
	if err != nil {
		t.Fatalf("Migrate with no ~/.cdl returned an error: %v", err)
	}
	if n != 0 {
		t.Fatalf("Migrate imported %d slots, want 0", n)
	}
}

func TestMigrateSkipsSlotsWithNoPath(t *testing.T) {
	home := t.TempDir()
	cdl := filepath.Join(home, ".cdl")
	os.MkdirAll(cdl, 0o755)
	os.WriteFile(filepath.Join(cdl, "slots.json"),
		[]byte(`{"1": {"path": "", "comment": "x"}, "2": {"path": "/tmp/ok"}}`), 0o644)
	dir := filepath.Join(home, ".config", "dl")
	n, err := Migrate(home, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("imported %d, want 1 (the empty-path slot must be skipped)", n)
	}
	st, _ := Open(dir)
	if _, ok := st.Get(1); ok {
		t.Error("slot 1 was imported despite having no path")
	}
}
