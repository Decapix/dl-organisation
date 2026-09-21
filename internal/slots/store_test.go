package slots

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// openTestStore opens a store rooted in a fresh temp directory.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenForUpdate(t.TempDir())
	if err != nil {
		t.Fatalf("OpenForUpdate: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestOpenCreatesAnEmptyStore(t *testing.T) {
	st := openTestStore(t)
	if st.Len() != 0 {
		t.Fatalf("Len() = %d on a fresh store, want 0", st.Len())
	}
	if got := st.All(); len(got) != 0 {
		t.Fatalf("All() = %v, want empty", got)
	}
}

func TestPutGetRoundTripsThroughDisk(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenForUpdate(dir)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 9, 21, 13, 0, 0, 0, time.UTC)
	st.Put(Slot{Number: 7, Name: "exam42", Path: "/tmp/x", Note: "line1\nline2", Updated: when})
	if err := st.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	st.Close()

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reopened.Get(7)
	if !ok {
		t.Fatal("Get(7) after reopen: not found")
	}
	if got.Name != "exam42" || got.Path != "/tmp/x" || got.Note != "line1\nline2" {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if !got.Updated.Equal(when) {
		t.Fatalf("Updated = %v, want %v", got.Updated, when)
	}
	if got.Number != 7 {
		t.Fatalf("Number = %d, want 7 (it must be restored from the map key)", got.Number)
	}
}

func TestSaveWritesVersionedSchema(t *testing.T) {
	dir := t.TempDir()
	st, _ := OpenForUpdate(dir)
	st.Put(Slot{Number: 1, Path: "/tmp/a"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()

	raw, err := os.ReadFile(filepath.Join(dir, StoreFileName))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Version int                    `json:"version"`
		Slots   map[string]interface{} `json:"slots"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("store file is not valid JSON: %v", err)
	}
	if f.Version != SchemaVersion {
		t.Fatalf("version = %d, want %d", f.Version, SchemaVersion)
	}
	if _, ok := f.Slots["1"]; !ok {
		t.Fatalf("slots key %q missing, got %v", "1", f.Slots)
	}
}

func TestAllIsSortedByNumber(t *testing.T) {
	st := openTestStore(t)
	for _, n := range []int{12, 3, 7, 1} {
		st.Put(Slot{Number: n, Path: "/tmp"})
	}
	var got []int
	for _, s := range st.All() {
		got = append(got, s.Number)
	}
	want := []int{1, 3, 7, 12}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("All() order = %v, want %v", got, want)
		}
	}
}

func TestDelete(t *testing.T) {
	st := openTestStore(t)
	st.Put(Slot{Number: 4, Path: "/tmp"})
	if !st.Delete(4) {
		t.Fatal("Delete(4) = false, want true")
	}
	if _, ok := st.Get(4); ok {
		t.Fatal("Get(4) after Delete: still present")
	}
	if st.Delete(4) {
		t.Fatal("Delete(4) twice = true, want false")
	}
}

func TestFirstFree(t *testing.T) {
	st := openTestStore(t)
	if n, err := st.FirstFree(); err != nil || n != 1 {
		t.Fatalf("FirstFree() on empty = (%d, %v), want (1, nil)", n, err)
	}
	st.Put(Slot{Number: 1, Path: "/tmp"})
	st.Put(Slot{Number: 2, Path: "/tmp"})
	st.Put(Slot{Number: 4, Path: "/tmp"})
	if n, err := st.FirstFree(); err != nil || n != 3 {
		t.Fatalf("FirstFree() = (%d, %v), want (3, nil)", n, err)
	}
}

func TestOpenRejectsACorruptStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StoreFileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("Open on a corrupt store = nil error, want an error")
	}
}

func TestSaveIsAtomicAndLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	st, _ := OpenForUpdate(dir)
	st.Put(Slot{Number: 1, Path: "/tmp/a"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != StoreFileName && e.Name() != LockFileName {
			t.Fatalf("stray file left behind after Save: %q", e.Name())
		}
	}
}

func TestSaveOnAReadOnlyStoreFails(t *testing.T) {
	dir := t.TempDir()
	st, _ := OpenForUpdate(dir)
	st.Put(Slot{Number: 1, Path: "/tmp/a"})
	st.Save()
	st.Close()

	ro, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ro.Put(Slot{Number: 2, Path: "/tmp/b"})
	if err := ro.Save(); err == nil {
		t.Fatal("Save on a read-only store = nil, want an error")
	}
}
