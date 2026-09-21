# dl Phase 1 (CLI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a `dl` binary that fully replaces the `stl`/`cdl`/`seedl` zsh functions — unified command surface, name + note data model, atomic locked JSON store, automatic migration from `~/.cdl`, and shell integration.

**Architecture:** Three layers with one-way dependencies. `internal/slots` owns the data (type, store, ref resolution, migration) and knows nothing about the CLI. `internal/cli` turns `argv` into a `Command` value and knows nothing about the store. `internal/actions` joins them: each action is `func(env *Env, cmd cli.Command) error`, takes its store and its output writer by injection, and never calls `os.Exit` or `fmt.Println`. `cmd/dl/main.go` is wiring only — build the env, dispatch, map errors to exit codes. Phase 2's TUI will call the same actions, so no behaviour gets written twice.

**Tech Stack:** Go 1.24, standard library only in phase 1 (`golang.org/x/sys/unix` for `flock`). Bubbletea arrives in phase 2.

**Spec:** `docs/superpowers/specs/2026-09-21-dl-design.md`

---

## File Structure

| File | Responsibility |
|------|----------------|
| `go.mod` | module `github.com/Decapix/dl-organisation` |
| `cmd/dl/main.go` | wiring: build env, dispatch, exit codes |
| `internal/slots/slot.go` | `Slot` type and its display helpers |
| `internal/slots/store.go` | load/save, atomic write, CRUD over slots |
| `internal/slots/lock.go` | `flock` around the read-modify-write cycle |
| `internal/slots/ref.go` | `"7" \| "exam42" \| "ex"` -> `Slot` |
| `internal/slots/migrate.go` | one-shot import of `~/.cdl` |
| `internal/cli/command.go` | `Action` enum, `Command` value, `UsageError` |
| `internal/cli/parse.go` | `argv` -> `Command` |
| `internal/cli/help.go` | embedded help text, global and per action |
| `internal/editor/editor.go` | `$EDITOR` resolution and temp-file round-trip |
| `internal/actions/env.go` | `Env` (injected dependencies) and shared helpers |
| `internal/actions/cd.go` | `--cd` |
| `internal/actions/set.go` | `--set` |
| `internal/actions/see.go` | `--see` and its rendering |
| `internal/actions/note.go` | `--edit`, `--reset`, `--delete`, `--name` |
| `internal/actions/path.go` | `--path` |
| `internal/shellinit/shellinit.go` | `go:embed` of the shell snippets |
| `internal/shellinit/{zsh.sh,bash.sh,fish.fish}` | the snippets themselves |

Tests sit next to their package as `*_test.go`, plus `cmd/dl/e2e_test.go` for the end-to-end pass.

---

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`, `.gitignore`, `cmd/dl/main.go`, `LICENSE`

- [ ] **Step 1: Initialise the module**

```bash
cd /home/solenopsis/Tocuments/camputing/dl-organisation
go mod init github.com/Decapix/dl-organisation
go get golang.org/x/sys@latest
```

- [ ] **Step 2: Write `.gitignore`**

```
/dl
/dist/
*.test
```

- [ ] **Step 3: Write a stub `cmd/dl/main.go` that compiles**

```go
// Command dl bookmarks directories in numbered slots and keeps a free-form
// note on each one. See docs/superpowers/specs for the design.
package main

import "fmt"

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "dev"

func main() {
	fmt.Println("dl", version)
}
```

- [ ] **Step 4: Verify it builds and runs**

Run: `go build ./... && go run ./cmd/dl`
Expected: prints `dl dev`

- [ ] **Step 5: Add the MIT LICENSE file**

Write a standard MIT licence with `Copyright (c) 2026 Decapix`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .gitignore cmd LICENSE
git commit -m "chore: scaffold the Go module"
```

---

### Task 2: The Slot type

**Files:**
- Create: `internal/slots/slot.go`
- Test: `internal/slots/slot_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/slots/ -run 'TestDisplayName|TestHasNote|TestExists|TestShortPath|TestNoteFirstLine' -v`
Expected: build failure, `undefined: Slot`

- [ ] **Step 3: Write the implementation**

```go
// Package slots owns dl's data: the Slot type, the on-disk store, reference
// resolution, and the one-shot import of the legacy ~/.cdl layout. It knows
// nothing about the command line or about rendering.
package slots

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MinSlot and MaxSlot bound the slot numbers a user may address.
const (
	MinSlot = 1
	MaxSlot = 1000
)

// Slot is one bookmarked directory.
//
// Number is the key and is not serialised: it is the map key in the store
// file. Name is an optional short label that doubles as a reference, so
// `dl exam42` works. Note is free-form multi-line text with no special first
// line — it is the whole point of the tool, not an afterthought.
type Slot struct {
	Number  int       `json:"-"`
	Name    string    `json:"name,omitempty"`
	Path    string    `json:"path"`
	Note    string    `json:"note,omitempty"`
	Updated time.Time `json:"updated"`
}

// DisplayName is what the user sees in a listing. An unnamed slot falls back
// to the directory's base name so that no row is ever unlabelled.
func (s Slot) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return filepath.Base(s.Path)
}

// HasNote reports whether the slot carries anything worth reading. Whitespace
// left behind by an aborted edit does not count.
func (s Slot) HasNote() bool {
	return strings.TrimSpace(s.Note) != ""
}

// NoteFirstLine is the first non-blank line of the note, used for one-line
// previews. It returns "" when there is no note.
func (s Slot) NoteFirstLine() string {
	for _, line := range strings.Split(s.Note, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// Exists reports whether the slot still points at a real directory. A path
// that has been deleted, or replaced by a regular file, is dead.
func (s Slot) Exists() bool {
	fi, err := os.Stat(s.Path)
	return err == nil && fi.IsDir()
}

// ShortPath renders p with the home directory collapsed to "~". The prefix
// must end at a separator, so /home/user2 is not shortened against /home/u.
func ShortPath(p, home string) string {
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/slots/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/slots/slot.go internal/slots/slot_test.go
git commit -m "feat(slots): add the Slot type and its display helpers"
```

---

### Task 3: The store — load, save, CRUD

**Files:**
- Create: `internal/slots/store.go`
- Test: `internal/slots/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/slots/ -run TestOpen -v`
Expected: build failure, `undefined: OpenForUpdate`

- [ ] **Step 3: Write the implementation**

```go
package slots

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Names and schema version of the on-disk store.
const (
	StoreFileName = "slots.json"
	LockFileName  = ".lock"
	SchemaVersion = 1
)

// ErrReadOnly is returned by Save when the store was opened with Open rather
// than OpenForUpdate, meaning no write lock is held.
var ErrReadOnly = errors.New("store is open read-only; use OpenForUpdate")

// fileFormat is the on-disk shape. The slot number is the map key, which keeps
// the file readable and lets a human edit it by hand in a pinch.
type fileFormat struct {
	Version int             `json:"version"`
	Slots   map[string]Slot `json:"slots"`
}

// Store holds every slot in memory. A command loads it once, mutates it, and
// calls Save exactly once, so there is no partial-write window.
type Store struct {
	dir   string
	data  map[int]Slot
	lock  *lockFile // nil when opened read-only
}

// DefaultDir is where the store lives: $DL_DIR when set, otherwise
// $XDG_CONFIG_HOME/dl, otherwise ~/.config/dl.
func DefaultDir(getenv func(string) string, home string) string {
	if d := getenv("DL_DIR"); d != "" {
		return d
	}
	if d := getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "dl")
	}
	return filepath.Join(home, ".config", "dl")
}

// Open loads the store for reading. It does not take the write lock, so it
// must not be used by a command that intends to Save.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, data: map[int]Slot{}}
	return s, s.load()
}

// OpenForUpdate takes an exclusive lock on the store directory and then loads.
// The lock is held until Close, which makes the whole read-modify-write cycle
// atomic against other dl processes. Callers must defer Close.
func OpenForUpdate(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	lk, err := acquireLock(filepath.Join(dir, LockFileName))
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir, data: map[int]Slot{}, lock: lk}
	if err := s.load(); err != nil {
		lk.release()
		return nil, err
	}
	return s, nil
}

// Close releases the write lock. It is safe to call on a read-only store.
func (s *Store) Close() error {
	if s.lock == nil {
		return nil
	}
	err := s.lock.release()
	s.lock = nil
	return err
}

// Dir reports the directory the store lives in.
func (s *Store) Dir() string { return s.dir }

// file is the full path of the store file.
func (s *Store) file() string { return filepath.Join(s.dir, StoreFileName) }

// load reads the store file into memory. A missing file is an empty store,
// which is the normal first-run case; malformed JSON is an error, because
// silently starting empty would look like data loss.
func (s *Store) load() error {
	raw, err := os.ReadFile(s.file())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", s.file(), err)
	}
	if len(raw) == 0 {
		return nil
	}
	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("parse %s: %w", s.file(), err)
	}
	for key, sl := range f.Slots {
		n, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("parse %s: slot key %q is not a number", s.file(), key)
		}
		sl.Number = n // Number is not serialised; restore it from the key.
		s.data[n] = sl
	}
	return nil
}

// Save writes the store atomically: a temp file in the same directory, fsynced,
// then renamed over the target. A crash can never leave a truncated store.
func (s *Store) Save() error {
	if s.lock == nil {
		return ErrReadOnly
	}
	f := fileFormat{Version: SchemaVersion, Slots: map[string]Slot{}}
	for n, sl := range s.data {
		f.Slots[strconv.Itoa(n)] = sl
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(s.dir, StoreFileName+".tmp*")
	if err != nil {
		return fmt.Errorf("create temp store: %w", err)
	}
	tmpName := tmp.Name()
	// Any failure from here on must not leave the temp file behind.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp store: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp store: %w", err)
	}
	if err := os.Rename(tmpName, s.file()); err != nil {
		return fmt.Errorf("replace store: %w", err)
	}
	return nil
}

// Get returns the slot with the given number.
func (s *Store) Get(n int) (Slot, bool) {
	sl, ok := s.data[n]
	return sl, ok
}

// Put inserts or replaces a slot. The caller is responsible for setting
// Updated; the store does not reach for the clock, which keeps it testable.
func (s *Store) Put(sl Slot) { s.data[sl.Number] = sl }

// Delete removes a slot and reports whether it was there.
func (s *Store) Delete(n int) bool {
	if _, ok := s.data[n]; !ok {
		return false
	}
	delete(s.data, n)
	return true
}

// Len is the number of occupied slots.
func (s *Store) Len() int { return len(s.data) }

// All returns every slot ordered by number, which is the order every listing
// uses.
func (s *Store) All() []Slot {
	out := make([]Slot, 0, len(s.data))
	for _, sl := range s.data {
		out = append(out, sl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// FirstFree is the lowest unoccupied slot number, used by `dl -z` with no ref.
func (s *Store) FirstFree() (int, error) {
	for n := MinSlot; n <= MaxSlot; n++ {
		if _, taken := s.data[n]; !taken {
			return n, nil
		}
	}
	return 0, fmt.Errorf("every slot from %d to %d is occupied", MinSlot, MaxSlot)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/slots/ -v`
Expected: build failure on `acquireLock` — that is Task 4. Write Task 4's `lock.go` first if the runner wants a green bar at this step; otherwise proceed to Task 4 and run both suites there.

- [ ] **Step 5: Commit**

```bash
git add internal/slots/store.go internal/slots/store_test.go
git commit -m "feat(slots): add the JSON store with atomic writes"
```

---

### Task 4: The store lock

**Files:**
- Create: `internal/slots/lock.go`
- Test: `internal/slots/lock_test.go`

- [ ] **Step 1: Write the failing test**

```go
package slots

import (
	"sync"
	"testing"
)

// TestConcurrentUpdatesDoNotLoseWrites is the regression test for the bug in
// the old zsh implementation: two shells saving at once clobbered each other,
// because read-modify-write was not serialised.
func TestConcurrentUpdatesDoNotLoseWrites(t *testing.T) {
	dir := t.TempDir()
	const writers = 8

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			st, err := OpenForUpdate(dir)
			if err != nil {
				errs <- err
				return
			}
			defer st.Close()
			st.Put(Slot{Number: n + 1, Path: "/tmp"})
			if err := st.Save(); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent writer failed: %v", err)
	}

	final, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if final.Len() != writers {
		t.Fatalf("Len() = %d after %d concurrent writers, want %d (writes were lost)",
			final.Len(), writers, writers)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	st, err := OpenForUpdate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/slots/ -run 'TestConcurrent|TestRelease' -v`
Expected: build failure, `undefined: acquireLock`

- [ ] **Step 3: Write the implementation**

```go
package slots

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile is an advisory exclusive lock held for the whole read-modify-write
// cycle of a dl command. It is a separate file from the store so that the
// atomic rename in Save never pulls the lock out from under us.
type lockFile struct {
	f *os.File
}

// acquireLock blocks until the lock is available. Commands are short, so a
// blocking wait is friendlier than failing with "try again".
func acquireLock(path string) (*lockFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", path, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &lockFile{f: f}, nil
}

// release unlocks and closes. Closing the descriptor drops the flock anyway;
// unlocking explicitly makes the intent obvious.
func (l *lockFile) release() error {
	if l == nil || l.f == nil {
		return nil
	}
	unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	err := l.f.Close()
	l.f = nil
	return err
}
```

- [ ] **Step 4: Run the whole package suite**

Run: `go test ./internal/slots/ -v -race`
Expected: all PASS, including Task 3's tests

- [ ] **Step 5: Commit**

```bash
git add internal/slots/lock.go internal/slots/lock_test.go
git commit -m "feat(slots): serialise updates with an flock"
```

---

### Task 5: Reference resolution

**Files:**
- Create: `internal/slots/ref.go`
- Test: `internal/slots/ref_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
		in   string
		n    int
		ok   bool
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/slots/ -run 'TestResolve|TestParseSlot|TestNumericRef|TestExactName' -v`
Expected: build failure, `undefined: ParseSlotNumber`

- [ ] **Step 3: Write the implementation**

```go
package slots

import (
	"fmt"
	"strconv"
	"strings"
)

// AmbiguousRefError is returned when a name prefix matches more than one slot.
// It carries the candidates so the caller can print them, which turns a dead
// end into a menu.
type AmbiguousRefError struct {
	Ref        string
	Candidates []Slot
}

func (e *AmbiguousRefError) Error() string {
	names := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		names = append(names, fmt.Sprintf("%d %s", c.Number, c.DisplayName()))
	}
	return fmt.Sprintf("%q is ambiguous: %s", e.Ref, strings.Join(names, ", "))
}

// ParseSlotNumber reports whether ref is a slot number in range. It is the
// first rule of Resolve and is also used by --set, which accepts a number for
// a slot that does not exist yet.
func ParseSlotNumber(ref string) (int, bool) {
	if ref == "" {
		return 0, false
	}
	for _, r := range ref {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(ref)
	if err != nil || n < MinSlot || n > MaxSlot {
		return 0, false
	}
	return n, true
}

// Resolve turns a user-supplied reference into a slot, in this order:
//
//  1. an in-range number   -> that slot, or an "empty" error
//  2. an exact name        -> that slot (case-insensitive)
//  3. a unique name prefix -> that slot
//  4. several prefixes     -> *AmbiguousRefError listing them
//  5. anything else        -> not found
//
// Rule 1 never falls through to the name rules, so numbers and names can never
// shadow each other.
func (s *Store) Resolve(ref string) (Slot, error) {
	if n, ok := ParseSlotNumber(ref); ok {
		sl, exists := s.Get(n)
		if !exists {
			return Slot{}, fmt.Errorf("slot %d is empty; save one with: dl -z %d", n, n)
		}
		return sl, nil
	}
	if ref == "" {
		return Slot{}, fmt.Errorf("missing slot reference")
	}

	needle := strings.ToLower(ref)
	var prefixes []Slot
	for _, sl := range s.All() {
		name := strings.ToLower(sl.Name)
		if name == "" {
			continue
		}
		if name == needle {
			return sl, nil // exact match wins outright
		}
		if strings.HasPrefix(name, needle) {
			prefixes = append(prefixes, sl)
		}
	}
	switch len(prefixes) {
	case 1:
		return prefixes[0], nil
	case 0:
		return Slot{}, fmt.Errorf("no slot matches %q; list them with: dl -s", ref)
	default:
		return Slot{}, &AmbiguousRefError{Ref: ref, Candidates: prefixes}
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/slots/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/slots/ref.go internal/slots/ref_test.go
git commit -m "feat(slots): resolve a ref by number, name, or unique prefix"
```

---

### Task 6: Migration from ~/.cdl

**Files:**
- Create: `internal/slots/migrate.go`
- Test: `internal/slots/migrate_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/slots/ -run TestMigrate -v`
Expected: build failure, `undefined: Migrate`

- [ ] **Step 3: Write the implementation**

```go
package slots

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// legacyTimeLayout is the format the zsh implementation wrote with
// `date '+%Y-%m-%d %H:%M:%S'`.
const legacyTimeLayout = "2006-01-02 15:04:05"

// legacySlot is one entry of the old ~/.cdl/slots.json.
type legacySlot struct {
	Path    string `json:"path"`
	Comment string `json:"comment"`
	Updated string `json:"updated"`
}

// Migrate imports ~/.cdl into the new store exactly once and reports how many
// slots it brought over.
//
// It does nothing (returning 0) when there is no legacy directory or when the
// new store already exists, so it is safe to call on every run. The old
// directory is renamed to ~/.cdl.bak rather than deleted: an import bug must
// never be the reason a user loses years of notes.
//
// The two legacy fields collapse into one note: the comment becomes the first
// line, and the description file follows after a blank line.
func Migrate(home, dir string) (int, error) {
	legacyDir := filepath.Join(home, ".cdl")
	legacyJSON := filepath.Join(legacyDir, "slots.json")

	if _, err := os.Stat(legacyJSON); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if _, err := os.Stat(filepath.Join(dir, StoreFileName)); err == nil {
		return 0, nil // already migrated, or the user started fresh
	}

	raw, err := os.ReadFile(legacyJSON)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", legacyJSON, err)
	}
	var legacy map[string]legacySlot
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return 0, fmt.Errorf("parse %s: %w", legacyJSON, err)
	}

	st, err := OpenForUpdate(dir)
	if err != nil {
		return 0, err
	}
	defer st.Close()

	imported := 0
	for key, old := range legacy {
		n, err := strconv.Atoi(key)
		if err != nil || n < MinSlot || n > MaxSlot {
			continue // unusable key, skip rather than abort the whole import
		}
		if strings.TrimSpace(old.Path) == "" {
			continue // the old tool left empty rows behind; they carry nothing
		}
		st.Put(Slot{
			Number:  n,
			Path:    old.Path,
			Note:    mergeLegacyNote(old.Comment, readLegacyDescription(legacyDir, n)),
			Updated: parseLegacyTime(old.Updated),
		})
		imported++
	}
	if err := st.Save(); err != nil {
		return 0, err
	}

	// Keep the originals. Renaming is atomic and reversible; deleting is not.
	if err := os.Rename(legacyDir, legacyDir+".bak"); err != nil {
		return imported, fmt.Errorf("imported %d slots but could not rename %s: %w",
			imported, legacyDir, err)
	}
	return imported, nil
}

// readLegacyDescription returns the body of descriptions/<n>.txt, or "".
func readLegacyDescription(legacyDir string, n int) string {
	raw, err := os.ReadFile(filepath.Join(legacyDir, "descriptions", strconv.Itoa(n)+".txt"))
	if err != nil {
		return ""
	}
	return string(raw)
}

// mergeLegacyNote folds the old comment and description into one note:
// comment first, then a blank line, then the description. Leading and
// trailing blank lines are dropped on both sides, because the old descriptions
// tend to start with them.
func mergeLegacyNote(comment, description string) string {
	comment = strings.TrimSpace(comment)
	description = strings.TrimSpace(description)
	switch {
	case comment == "" && description == "":
		return ""
	case description == "":
		return comment
	case comment == "":
		return description
	default:
		return comment + "\n\n" + description
	}
}

// parseLegacyTime reads the old timestamp format, falling back to the zero
// time when it is absent or malformed. A bad timestamp is not worth failing an
// import over.
func parseLegacyTime(s string) time.Time {
	t, err := time.ParseInLocation(legacyTimeLayout, strings.TrimSpace(s), time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/slots/ -v`
Expected: all PASS

- [ ] **Step 5: Dry-run the migration against a copy of the real data**

```bash
cp -r ~/.cdl /tmp/dl-migration-check
go test ./internal/slots/ -run TestMigrate -v
```

Expected: PASS. The real `~/.cdl` is untouched — migration only runs from the binary.

- [ ] **Step 6: Commit**

```bash
git add internal/slots/migrate.go internal/slots/migrate_test.go
git commit -m "feat(slots): import the legacy ~/.cdl store once"
```

---

### Task 7: The command value and the parser

The grammar has two rules that no flag library expresses well, which is why the
parser is hand-written:

1. **`-n`, `-m`, `-e`, `-r` are modifiers of `-z`; without `-z` each is an
   action of its own.** `dl -z 7 -e` saves then edits; `dl -e 7` only edits.
2. **A bare argument means `--cd`.** `dl 7` is the most frequent command and
   must stay four keystrokes.

Action flags never consume a value. Refs are always positionals, so `dl 7` and
`dl -c 7` take the identical path through the parser.

**Files:**
- Create: `internal/cli/command.go`, `internal/cli/parse.go`
- Test: `internal/cli/parse_test.go`

- [ ] **Step 1: Write `internal/cli/command.go`**

```go
// Package cli turns argv into a Command value. It knows nothing about the
// store, the filesystem, or rendering, which makes the whole grammar testable
// as a pure function.
package cli

import "fmt"

// Action is the single operation an invocation performs.
type Action int

const (
	ActionTUI     Action = iota // bare `dl`
	ActionCD                    // dl <ref>, dl -c <ref>
	ActionSet                   // dl -z [ref]
	ActionSee                   // dl -s
	ActionEdit                  // dl -e <ref>
	ActionReset                 // dl -r <ref>
	ActionDelete                // dl -d <ref>
	ActionPath                  // dl -p <ref>
	ActionRename                // dl -n <ref> <name>
	ActionSetNote               // dl -m <ref> <text>
	ActionDoctor                // dl doctor
	ActionInit                  // dl init <shell>
	ActionHelp                  // dl -h, dl <action> -h
	ActionVersion               // dl --version
)

// String is used by help lookup and by error messages.
func (a Action) String() string {
	switch a {
	case ActionTUI:
		return "tui"
	case ActionCD:
		return "--cd"
	case ActionSet:
		return "--set"
	case ActionSee:
		return "--see"
	case ActionEdit:
		return "--edit"
	case ActionReset:
		return "--reset"
	case ActionDelete:
		return "--delete"
	case ActionPath:
		return "--path"
	case ActionRename:
		return "--name"
	case ActionSetNote:
		return "--note"
	case ActionDoctor:
		return "doctor"
	case ActionInit:
		return "init"
	case ActionHelp:
		return "--help"
	case ActionVersion:
		return "--version"
	}
	return "unknown"
}

// Command is a fully parsed invocation.
type Command struct {
	Action Action
	Ref    string // slot reference, when the action takes one
	Name   string // -n value
	Note   string // -m value
	Edit   bool   // -e alongside -z
	Reset  bool   // -r alongside -z
	Long   bool   // -l on --see
	Quiet  bool   // -q on --see
	Shell  string // `init` argument

	// HelpFor is the action whose help to print when Action is ActionHelp.
	// ActionTUI means the global overview.
	HelpFor Action
}

// UsageError is a mistake in how the command was typed, as opposed to a
// runtime failure. main maps it to exit code 2 and prints the action's help
// pointer rather than the global help page.
type UsageError struct {
	Msg    string
	Action Action // the action the user was reaching for, for the help pointer
}

func (e *UsageError) Error() string { return e.Msg }

// usagef builds a UsageError tied to an action.
func usagef(a Action, format string, args ...any) *UsageError {
	return &UsageError{Msg: fmt.Sprintf(format, args...), Action: a}
}
```

- [ ] **Step 2: Write the failing parser test**

```go
package cli

import (
	"errors"
	"testing"
)

func TestParseTable(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want Command
	}{
		{"bare opens the TUI", nil, Command{Action: ActionTUI}},
		{"bare ref is a cd", []string{"7"}, Command{Action: ActionCD, Ref: "7"}},
		{"named ref is a cd", []string{"exam42"}, Command{Action: ActionCD, Ref: "exam42"}},
		{"explicit short cd", []string{"-c", "7"}, Command{Action: ActionCD, Ref: "7"}},
		{"explicit long cd", []string{"--cd", "7"}, Command{Action: ActionCD, Ref: "7"}},

		{"set with no ref", []string{"-z"}, Command{Action: ActionSet}},
		{"set with ref", []string{"-z", "7"}, Command{Action: ActionSet, Ref: "7"}},
		{"set long", []string{"--set", "7"}, Command{Action: ActionSet, Ref: "7"}},
		{"set with name", []string{"-z", "7", "-n", "exam42"},
			Command{Action: ActionSet, Ref: "7", Name: "exam42"}},
		{"set with note", []string{"-z", "7", "-m", "fix the loop"},
			Command{Action: ActionSet, Ref: "7", Note: "fix the loop"}},
		{"set with edit", []string{"-z", "7", "-e"},
			Command{Action: ActionSet, Ref: "7", Edit: true}},
		{"set with reset", []string{"-z", "7", "-r"},
			Command{Action: ActionSet, Ref: "7", Reset: true}},
		{"set with everything", []string{"-z", "7", "-r", "-n", "x", "-e"},
			Command{Action: ActionSet, Ref: "7", Reset: true, Name: "x", Edit: true}},
		{"flags may precede the ref", []string{"-z", "-n", "x", "7"},
			Command{Action: ActionSet, Ref: "7", Name: "x"}},
		{"long flag with equals", []string{"-z", "7", "--name=exam42"},
			Command{Action: ActionSet, Ref: "7", Name: "exam42"}},

		{"see", []string{"-s"}, Command{Action: ActionSee}},
		{"see long", []string{"--see"}, Command{Action: ActionSee}},
		{"see long form", []string{"-s", "-l"}, Command{Action: ActionSee, Long: true}},
		{"see quiet", []string{"-s", "-q"}, Command{Action: ActionSee, Quiet: true}},

		{"edit as an action", []string{"-e", "7"}, Command{Action: ActionEdit, Ref: "7"}},
		{"reset as an action", []string{"-r", "7"}, Command{Action: ActionReset, Ref: "7"}},
		{"delete", []string{"-d", "7"}, Command{Action: ActionDelete, Ref: "7"}},
		{"path", []string{"-p", "7"}, Command{Action: ActionPath, Ref: "7"}},
		{"rename as an action", []string{"-n", "7", "exam42"},
			Command{Action: ActionRename, Ref: "7", Name: "exam42"}},
		{"set note as an action", []string{"-m", "7", "fix the loop"},
			Command{Action: ActionSetNote, Ref: "7", Note: "fix the loop"}},

		{"doctor", []string{"doctor"}, Command{Action: ActionDoctor}},
		{"init", []string{"init", "zsh"}, Command{Action: ActionInit, Shell: "zsh"}},
		{"version", []string{"--version"}, Command{Action: ActionVersion}},

		{"global help", []string{"-h"}, Command{Action: ActionHelp, HelpFor: ActionTUI}},
		{"global help long", []string{"--help"}, Command{Action: ActionHelp, HelpFor: ActionTUI}},
		{"action help", []string{"-z", "-h"}, Command{Action: ActionHelp, HelpFor: ActionSet}},
		{"action help long", []string{"--cd", "--help"}, Command{Action: ActionHelp, HelpFor: ActionCD}},
		{"help before the action still works", []string{"-h", "-z"},
			Command{Action: ActionHelp, HelpFor: ActionSet}},
		{"init help", []string{"init", "-h"}, Command{Action: ActionHelp, HelpFor: ActionInit}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.argv)
			if err != nil {
				t.Fatalf("Parse(%q) returned an error: %v", c.argv, err)
			}
			if got != c.want {
				t.Fatalf("Parse(%q) =\n  %+v\nwant\n  %+v", c.argv, got, c.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"unknown short flag", []string{"-x"}},
		{"unknown long flag", []string{"--nope"}},
		{"two actions", []string{"-c", "-s", "7"}},
		{"two actions long", []string{"--set", "--delete", "7"}},
		{"cd without a ref", []string{"-c"}},
		{"delete without a ref", []string{"-d"}},
		{"path without a ref", []string{"-p"}},
		{"edit without a ref", []string{"-e"}},
		{"reset without a ref", []string{"-r"}},
		{"rename without a name", []string{"-n", "7"}},
		{"set note without text", []string{"-m", "7"}},
		{"too many positionals", []string{"7", "8"}},
		{"see takes no ref", []string{"-s", "7"}},
		{"long and quiet together", []string{"-s", "-l", "-q"}},
		{"name modifier without a value", []string{"-z", "7", "-n"}},
		{"note modifier without a value", []string{"-z", "7", "-m"}},
		{"init without a shell", []string{"init"}},
		{"init with an unknown shell", []string{"init", "csh"}},
		{"long form outside see", []string{"-z", "-l"}},
		{"quiet outside see", []string{"-c", "7", "-q"}},
		{"flag bundling is not supported", []string{"-sl"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.argv); err == nil {
				t.Fatalf("Parse(%q) = nil error, want a usage error", c.argv)
			}
		})
	}
}

// Every parse failure must be a *UsageError so main can exit 2 and point at
// the right help page.
func TestParseErrorsAreUsageErrors(t *testing.T) {
	_, err := Parse([]string{"-z", "-l"})
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("error type = %T, want *UsageError", err)
	}
	if ue.Action != ActionSet {
		t.Fatalf("UsageError.Action = %v, want %v", ue.Action, ActionSet)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/cli/ -v`
Expected: build failure, `undefined: Parse`

- [ ] **Step 4: Write `internal/cli/parse.go`**

```go
package cli

import "strings"

// validShells are the shells `dl init` can emit an integration for.
var validShells = map[string]bool{"zsh": true, "bash": true, "fish": true}

// Parse turns argv (without the program name) into a Command.
//
// It runs in two passes. The first pass splits tokens into flags and
// positionals and records which flags were seen; the second pass decides what
// the action is and validates the positional count. Two passes are what makes
// rule 1 (modifier-or-action) expressible: whether -e is a modifier depends on
// a flag that may appear after it.
func Parse(argv []string) (Command, error) {
	if len(argv) == 0 {
		return Command{Action: ActionTUI}, nil
	}

	// Subcommand words are recognised only in first position, so a slot named
	// "doctor" is still reachable as `dl -c doctor`.
	switch argv[0] {
	case "init":
		return parseInit(argv[1:])
	case "doctor":
		return parseDoctor(argv[1:])
	}

	// Pre-scan for -z. This is what makes rule 1 expressible: whether -n and
	// -m consume the next token depends on a flag that may appear after them,
	// so we have to know before walking the tokens.
	setMode := false
	for _, tok := range argv {
		if tok == "-z" || tok == "--set" {
			setMode = true
			break
		}
	}

	var (
		cmd         Command
		positionals []string
		help        bool
		// which flags were seen
		cd, set, see, edit, reset, del, path, rename, note bool
		long, quiet, version                               bool
	)

	// An explicit index rather than a range loop: value-taking flags advance
	// it themselves to swallow their operand.
	i := 0
	for i < len(argv) {
		tok := argv[i]

		// A lone "-" or anything not starting with "-" is a positional.
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			positionals = append(positionals, tok)
			i++
			continue
		}

		// Split --flag=value once, so --name=x and --name x are equivalent.
		flag, inline := tok, ""
		hasInline := false
		if eq := strings.Index(tok, "="); eq > 0 && strings.HasPrefix(tok, "--") {
			flag, inline, hasInline = tok[:eq], tok[eq+1:], true
		}

		// value returns the operand of a value-taking flag, from either
		// --flag=value or the following token, advancing i in the latter case.
		value := func() (string, bool) {
			if hasInline {
				return inline, true
			}
			if i+1 >= len(argv) {
				return "", false
			}
			i++
			return argv[i], true
		}

		switch flag {
		case "-h", "--help":
			help = true
		case "--version":
			version = true
		case "-c", "--cd":
			cd = true
		case "-z", "--set":
			set = true
		case "-s", "--see":
			see = true
		case "-d", "--delete":
			del = true
		case "-p", "--path":
			path = true
		case "-l", "--long":
			long = true
		case "-q", "--quiet":
			quiet = true
		case "-e", "--edit":
			edit = true
		case "-r", "--reset":
			reset = true

		case "-n", "--name":
			rename = true
			// As a modifier of -z the name is this flag's operand; as a
			// standalone action it is a positional, so that `dl -n 7 exam42`
			// reads left to right.
			if setMode {
				v, ok := value()
				if !ok {
					return Command{}, usagef(ActionSet, "%s needs a value", flag)
				}
				cmd.Name = v
			} else if hasInline {
				return Command{}, usagef(ActionRename, "use: dl -n <ref> <name>")
			}
		case "-m", "--note":
			note = true
			if setMode {
				v, ok := value()
				if !ok {
					return Command{}, usagef(ActionSet, "%s needs a value", flag)
				}
				cmd.Note = v
			} else if hasInline {
				return Command{}, usagef(ActionSetNote, "use: dl -m <ref> <text>")
			}

		default:
			return Command{}, usagef(ActionTUI, "unknown option %q", tok)
		}

		// Only -n and -m take a value; anything else with "=" is a mistake.
		if hasInline && flag != "-n" && flag != "--name" && flag != "-m" && flag != "--note" {
			return Command{}, usagef(ActionTUI, "%s does not take a value", flag)
		}
		i++
	}

	// Decide the action. -e, -r, -n and -m count as actions only when -z is
	// absent.
	actions := 0
	countIf := func(b bool, a Action) {
		if b {
			actions++
			cmd.Action = a
		}
	}
	countIf(cd, ActionCD)
	countIf(set, ActionSet)
	countIf(see, ActionSee)
	countIf(del, ActionDelete)
	countIf(path, ActionPath)
	if !set {
		countIf(edit, ActionEdit)
		countIf(reset, ActionReset)
		countIf(rename, ActionRename)
		countIf(note, ActionSetNote)
	}

	if actions > 1 {
		return Command{}, usagef(ActionTUI, "only one action per command")
	}
	if actions == 0 {
		switch {
		case version:
			return Command{Action: ActionVersion}, nil
		case help:
			return Command{Action: ActionHelp, HelpFor: ActionTUI}, nil
		case len(positionals) > 0:
			cmd.Action = ActionCD // rule 2: a bare argument means cd
		default:
			cmd.Action = ActionTUI
		}
	}

	// -h anywhere short-circuits into that action's help page.
	if help {
		return Command{Action: ActionHelp, HelpFor: cmd.Action}, nil
	}

	if set {
		cmd.Edit, cmd.Reset = edit, reset
	}
	cmd.Long, cmd.Quiet = long, quiet

	if err := validate(&cmd, positionals, long, quiet); err != nil {
		return Command{}, err
	}
	return cmd, nil
}

// validate checks the positional count and the flag combinations for the
// chosen action, and moves the positionals into the Command.
func validate(cmd *Command, pos []string, long, quiet bool) error {
	// -l and -q belong to --see only, and are mutually exclusive there.
	if (long || quiet) && cmd.Action != ActionSee {
		return usagef(cmd.Action, "-l and -q only apply to --see")
	}
	if long && quiet {
		return usagef(ActionSee, "-l and -q cannot be combined")
	}

	want := func(n int, what string) error {
		if len(pos) != n {
			return usagef(cmd.Action, "%s takes %s", cmd.Action, what)
		}
		return nil
	}

	switch cmd.Action {
	case ActionCD, ActionEdit, ActionReset, ActionDelete, ActionPath:
		if err := want(1, "exactly one slot reference"); err != nil {
			return err
		}
		cmd.Ref = pos[0]
	case ActionSet:
		if len(pos) > 1 {
			return usagef(ActionSet, "--set takes at most one slot reference")
		}
		if len(pos) == 1 {
			cmd.Ref = pos[0]
		}
	case ActionRename:
		if err := want(2, "a slot reference and a name"); err != nil {
			return err
		}
		cmd.Ref, cmd.Name = pos[0], pos[1]
	case ActionSetNote:
		if err := want(2, "a slot reference and a note"); err != nil {
			return err
		}
		cmd.Ref, cmd.Note = pos[0], pos[1]
	case ActionSee, ActionTUI:
		if err := want(0, "no arguments"); err != nil {
			return err
		}
	}
	return nil
}

// parseInit handles `dl init <shell>`.
func parseInit(rest []string) (Command, error) {
	for _, tok := range rest {
		if tok == "-h" || tok == "--help" {
			return Command{Action: ActionHelp, HelpFor: ActionInit}, nil
		}
	}
	if len(rest) != 1 {
		return Command{}, usagef(ActionInit, "init takes exactly one shell name (zsh, bash, fish)")
	}
	if !validShells[rest[0]] {
		return Command{}, usagef(ActionInit, "unknown shell %q; supported: zsh, bash, fish", rest[0])
	}
	return Command{Action: ActionInit, Shell: rest[0]}, nil
}

// parseDoctor handles `dl doctor`.
func parseDoctor(rest []string) (Command, error) {
	for _, tok := range rest {
		if tok == "-h" || tok == "--help" {
			return Command{Action: ActionHelp, HelpFor: ActionDoctor}, nil
		}
	}
	if len(rest) != 0 {
		return Command{}, usagef(ActionDoctor, "doctor takes no arguments")
	}
	return Command{Action: ActionDoctor}, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cli/ -v`
Expected: all PASS. If `flag bundling is not supported` fails, confirm `-sl`
reaches the `default:` branch and returns `unknown option`.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/command.go internal/cli/parse.go internal/cli/parse_test.go
git commit -m "feat(cli): parse the dl grammar"
```

---

### Task 8: Help text

**Files:**
- Create: `internal/cli/help.go`
- Test: `internal/cli/help_test.go`

- [ ] **Step 1: Write the failing test**

```go
package cli

import (
	"strings"
	"testing"
)

// Every action a user can type must have its own help page. This test is the
// guard that stops a new action shipping without documentation.
func TestEveryActionHasHelp(t *testing.T) {
	actions := []Action{
		ActionTUI, ActionCD, ActionSet, ActionSee, ActionEdit, ActionReset,
		ActionDelete, ActionPath, ActionRename, ActionSetNote, ActionDoctor,
		ActionInit,
	}
	for _, a := range actions {
		got := Help(a)
		if strings.TrimSpace(got) == "" {
			t.Errorf("Help(%v) is empty", a)
		}
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("Help(%v) does not end with a newline", a)
		}
	}
}

func TestGlobalHelpListsEveryAction(t *testing.T) {
	got := Help(ActionTUI)
	for _, want := range []string{
		"--cd", "--set", "--see", "--edit", "--reset", "--delete", "--path",
		"--name", "--note", "doctor", "init", "--help", "--version",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("global help does not mention %q", want)
		}
	}
}

func TestActionHelpHasOptionsAndExamples(t *testing.T) {
	got := Help(ActionSet)
	for _, want := range []string{"OPTIONS", "EXAMPLES", "-n, --name", "-r, --reset", "dl -z 7"} {
		if !strings.Contains(got, want) {
			t.Errorf("--set help does not contain %q", want)
		}
	}
}

// An unknown action must not panic; it falls back to the overview.
func TestHelpFallsBackToTheOverview(t *testing.T) {
	if Help(Action(999)) != Help(ActionTUI) {
		t.Error("Help for an unknown action should fall back to the global overview")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cli/ -run TestHelp -v`
Expected: build failure, `undefined: Help`

- [ ] **Step 3: Write `internal/cli/help.go`**

```go
package cli

// Help returns the help page for an action. ActionTUI means the global
// overview, which is deliberately short: it is a page you re-read, not a
// manual you read once.
func Help(a Action) string {
	if page, ok := helpPages[a]; ok {
		return page
	}
	return helpPages[ActionTUI]
}

var helpPages = map[Action]string{
	ActionTUI: `dl — directory bookmarks with notes

USAGE
  dl                      open the interactive browser
  dl <ref>                cd into a slot          ref: 7 | exam42 | ex
  dl -z [ref]             save the current directory
  dl -s                   list every slot

ACTIONS
  -c, --cd     <ref>      cd into a slot
  -z, --set    [ref]      save $PWD into a slot (default: lowest free slot)
  -s, --see               list every slot
  -e, --edit   <ref>      edit a slot's note in $EDITOR
  -r, --reset  <ref>      clear a slot's name and note, keep the path
  -d, --delete <ref>      remove a slot entirely
  -p, --path   <ref>      print a slot's path only
  -n, --name   <ref> <name>   rename a slot
  -m, --note   <ref> <text>   replace a slot's note inline

OTHER
  doctor                  report slots whose path is gone
  init <shell>            print the shell integration (zsh, bash, fish)
  -h, --help              this page; -h after an action for its details
      --version

Run "dl -z --help" (or -c, -s, -e ...) for an action's own options.
`,

	ActionCD: `dl -c, --cd <ref> — cd into a slot

  <ref> is a slot number, a slot name, or a unique prefix of a name.
  The bare form "dl <ref>" does the same thing and is shorter.

  This needs the shell integration to actually move your shell:
  add   eval "$(dl init zsh)"   to your shell rc file.

EXAMPLES
  dl 7                    by number
  dl exam42               by name
  dl ex                   by unique prefix
  dl -c 7                 explicit, for scripts
`,

	ActionSet: `dl -z, --set [ref] — save the current directory into a slot

  ref is optional: with no ref, dl picks the lowest free slot.
  An occupied slot is overwritten directly. The name and note are
  kept unless you pass -r.

OPTIONS
  -n, --name <name>   set the slot's short name
  -m, --note <text>   set the note inline (replaces the current one)
  -e, --edit          open $EDITOR on the note after saving
  -r, --reset         clear the name and the note

EXAMPLES
  dl -z                          save here, lowest free slot
  dl -z 7                        save here into slot 7
  dl -z 7 -n exam42              save and name it
  dl -z 7 -n exam42 -e           save, name it, then write the note
  dl -z 7 -m "waiting on the API fix"
  dl -z 7 -r -n scraping         reuse slot 7 for something else
`,

	ActionSee: `dl -s, --see — list every slot

  Slots are listed in number order. A leading marker shows the state:
    *   the slot has a note
    x   the path no longer exists

OPTIONS
  -l, --long    print each note in full under its slot
  -q, --quiet   print paths only, one per line, for piping

  -l and -q cannot be combined.

EXAMPLES
  dl -s
  dl -s -l
  dl -s -q | fzf
`,

	ActionEdit: `dl -e, --edit <ref> — edit a slot's note

  Opens the note in $DL_EDITOR, $VISUAL, $EDITOR, or vi, in that order.
  If the editor exits non-zero the stored note is left untouched.

EXAMPLES
  dl -e 7
  dl -e exam42
`,

	ActionReset: `dl -r, --reset <ref> — clear a slot's name and note

  The path is kept. To clear the whole slot use --delete.
  As a modifier of --set, -r saves the current directory and starts clean.

EXAMPLES
  dl -r 7                 wipe slot 7's name and note
  dl -z 7 -r              save here into slot 7, starting clean
`,

	ActionDelete: `dl -d, --delete <ref> — remove a slot entirely

  Path, name and note all go. This is the only command that removes a
  slot; --set never does.

EXAMPLES
  dl -d 7
  dl -d exam42
`,

	ActionPath: `dl -p, --path <ref> — print a slot's path

  Prints the path and nothing else, so it composes with other commands.

EXAMPLES
  dl -p 7
  cp report.pdf "$(dl -p 7)"
  ls "$(dl -p exam42)"
`,

	ActionRename: `dl -n, --name <ref> <name> — rename a slot

  The name is a second way to address a slot: once slot 7 is named
  exam42, "dl exam42" and "dl ex" both reach it.
  Pass an empty name to remove it: dl -n 7 ""

EXAMPLES
  dl -n 7 exam42
  dl -n 12 scraping
  dl -n 7 ""              drop the name
`,

	ActionSetNote: `dl -m, --note <ref> <text> — replace a slot's note

  The text replaces the whole note. For multi-line notes use --edit.

EXAMPLES
  dl -m 7 "waiting on the API fix"
  dl -m exam42 ""         clear the note
`,

	ActionDoctor: `dl doctor — report slots whose path is gone

  Lists every slot pointing at a directory that no longer exists, so you
  can repoint it with --set or drop it with --delete. It changes nothing
  on its own.

EXAMPLES
  dl doctor
`,

	ActionInit: `dl init <shell> — print the shell integration

  A program cannot change its parent shell's directory, so dl ships a
  small function that does the cd for you. Add this to your rc file:

    zsh    eval "$(dl init zsh)"      in ~/.zshrc
    bash   eval "$(dl init bash)"     in ~/.bashrc
    fish   dl init fish | source      in ~/.config/fish/config.fish

  Without it, "dl 7" prints the path instead of moving you.
`,
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/help.go internal/cli/help_test.go
git commit -m "feat(cli): add per-action help pages"
```

---

### Task 9: The editor round-trip

**Files:**
- Create: `internal/editor/editor.go`, `internal/editor/fake.go`
- Test: `internal/editor/editor_test.go`

- [ ] **Step 1: Write the failing test**

```go
package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEnv builds a getenv function from a map.
func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolvePrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"DL_EDITOR wins", map[string]string{"DL_EDITOR": "a", "VISUAL": "b", "EDITOR": "c"}, "a"},
		{"VISUAL next", map[string]string{"VISUAL": "b", "EDITOR": "c"}, "b"},
		{"EDITOR next", map[string]string{"EDITOR": "c"}, "c"},
		{"vi is the floor", map[string]string{}, "vi"},
		{"blank values are skipped", map[string]string{"DL_EDITOR": "", "EDITOR": "c"}, "c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Resolve(fakeEnv(c.env)); got != c.want {
				t.Fatalf("Resolve() = %q, want %q", got, c.want)
			}
		})
	}
}

// writeScript creates an executable shell script and returns its path.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-editor")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEditRoundTrip(t *testing.T) {
	// The editor appends a line to whatever it is given.
	script := writeScript(t, `printf 'appended\n' >> "$1"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	got, err := e.Edit("original")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got != "original\nappended" {
		t.Fatalf("Edit() = %q, want %q", got, "original\nappended")
	}
}

func TestEditSeesTheInitialContent(t *testing.T) {
	// The editor copies its input somewhere we can inspect.
	out := filepath.Join(t.TempDir(), "seen")
	script := writeScript(t, `cat "$1" > `+out)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	if _, err := e.Edit("line1\nline2"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(raw), "\n") != "line1\nline2" {
		t.Fatalf("editor saw %q", string(raw))
	}
}

func TestEditTrimsTrailingWhitespace(t *testing.T) {
	script := writeScript(t, `printf '  \n\n\n' >> "$1"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	got, err := e.Edit("note")
	if err != nil {
		t.Fatal(err)
	}
	if got != "note" {
		t.Fatalf("Edit() = %q, want %q (trailing blank lines must go)", got, "note")
	}
}

func TestEditSupportsAnEditorWithArguments(t *testing.T) {
	script := writeScript(t, `printf 'flag=%s\n' "$1" >> "$2"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script + " -w"})}

	got, err := e.Edit("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "flag=-w" {
		t.Fatalf("Edit() = %q, want %q", got, "flag=-w")
	}
}

func TestEditReturnsAnErrorWhenTheEditorFails(t *testing.T) {
	script := writeScript(t, `exit 3`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	if _, err := e.Edit("note"); err == nil {
		t.Fatal("Edit() = nil error after a non-zero editor exit, want an error")
	}
}

func TestEditLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	script := writeScript(t, `printf 'x' >> "$1"`)
	e := OS{Getenv: fakeEnv(map[string]string{"EDITOR": script})}

	if _, err := e.Edit("note"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), "dl-note") {
			t.Fatalf("temp file left behind: %s", en.Name())
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/editor/ -v`
Expected: build failure, `undefined: Resolve`

- [ ] **Step 3: Write `internal/editor/editor.go`**

```go
// Package editor runs the user's text editor over a slot's note. The round
// trip goes through a temp file because that is the only interface every
// editor agrees on.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Editor is the seam the actions depend on. Production code uses OS; tests
// use Fake.
type Editor interface {
	// Edit shows initial to the user and returns what they saved. A non-nil
	// error means the note must be left untouched.
	Edit(initial string) (string, error)
}

// Resolve picks the editor command: $DL_EDITOR, then $VISUAL, then $EDITOR,
// then vi. Honouring $EDITOR is what the old zsh implementation failed to do —
// it hardcoded vim.
func Resolve(getenv func(string) string) string {
	for _, key := range []string{"DL_EDITOR", "VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
	}
	return "vi"
}

// OS is the real editor: it spawns a process attached to the terminal.
type OS struct {
	// Getenv is injected so tests do not have to mutate the process
	// environment. Nil means os.Getenv.
	Getenv func(string) string

	// Stdin, Stdout and Stderr are what the editor process inherits. Nil
	// means the corresponding os.Std* file.
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

// Edit writes initial to a temp file, runs the editor on it, and reads the
// result back with trailing whitespace removed.
func (e OS) Edit(initial string) (string, error) {
	getenv := e.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	// The .md suffix gives editors a reason to turn on soft wrap and
	// spell-checking, which is what a note wants.
	f, err := os.CreateTemp("", "dl-note-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp note: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)

	body := initial
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n" // leave the cursor on a fresh line
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return "", fmt.Errorf("write temp note: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp note: %w", err)
	}

	// The editor setting may carry arguments, e.g. "code -w" or "emacsclient
	// -nw", so split it rather than treating it as a bare program name.
	parts := strings.Fields(Resolve(getenv))
	cmd := exec.Command(parts[0], append(parts[1:], name)...)
	cmd.Stdin = orStd(e.Stdin, os.Stdin)
	cmd.Stdout = orStd(e.Stdout, os.Stdout)
	cmd.Stderr = orStd(e.Stderr, os.Stderr)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", parts[0], err)
	}

	raw, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read temp note: %w", err)
	}
	// Editors add a trailing newline; some users leave blank lines behind.
	// Neither is content.
	return strings.TrimRight(string(raw), " \t\r\n"), nil
}

// orStd returns f, or fallback when f is nil.
func orStd(f, fallback *os.File) *os.File {
	if f == nil {
		return fallback
	}
	return f
}
```

- [ ] **Step 4: Write `internal/editor/fake.go`**

```go
package editor

// Fake is a test double for Editor. It lives in the production package rather
// than a _test.go file because the actions package needs it too.
type Fake struct {
	// Result is what Edit returns.
	Result string
	// Err, when non-nil, is what Edit returns instead.
	Err error
	// Seen records the content Edit was handed, so tests can assert that the
	// current note was passed in.
	Seen string
	// Calls counts invocations.
	Calls int
}

// Edit implements Editor.
func (f *Fake) Edit(initial string) (string, error) {
	f.Calls++
	f.Seen = initial
	if f.Err != nil {
		return "", f.Err
	}
	return f.Result, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/editor/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/editor
git commit -m "feat(editor): honour \$EDITOR with a temp-file round trip"
```

---

### Task 10: The action environment, --cd and --path

**Files:**
- Create: `internal/actions/env.go`, `internal/actions/cd.go`, `internal/actions/path.go`
- Test: `internal/actions/actions_test.go`

- [ ] **Step 1: Write the failing test**

```go
package actions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// testEnv builds an Env backed by a temp store, with buffers for output and a
// fake editor. It returns the env and the two buffers.
func testEnv(t *testing.T) (*Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	st, err := slots.OpenForUpdate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	var out, errb bytes.Buffer
	env := &Env{
		Store:  st,
		Out:    &out,
		Err:    &errb,
		Cwd:    "/tmp/cwd",
		Home:   "/home/u",
		Editor: &editor.Fake{},
		Now:    func() time.Time { return time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) },
	}
	return env, &out, &errb
}

func TestCDWritesTheTargetToTheCDFile(t *testing.T) {
	env, out, _ := testEnv(t)
	dir := t.TempDir()
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: dir})

	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err != nil {
		t.Fatalf("cd: %v", err)
	}
	raw, err := os.ReadFile(env.CDFile)
	if err != nil {
		t.Fatalf("cd file not written: %v", err)
	}
	if string(raw) != dir {
		t.Fatalf("cd file = %q, want %q", string(raw), dir)
	}
	if out.String() != "" {
		t.Fatalf("cd printed %q on a slot with no note, want nothing", out.String())
	}
}

// Jumping into a slot prints its note. This is the whole point of the tool:
// you arrive and immediately see where you left off.
func TestCDPrintsTheNote(t *testing.T) {
	env, out, _ := testEnv(t)
	dir := t.TempDir()
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Path: dir, Note: "fix the retry loop\nthen export to csv"})

	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fix the retry loop") {
		t.Fatalf("cd output = %q, want the note", out.String())
	}
	if !strings.Contains(out.String(), "then export to csv") {
		t.Fatalf("cd output = %q, want the whole note", out.String())
	}
}

// Without the shell integration there is no cd file, so print the path and say
// how to fix it rather than failing silently.
func TestCDWithoutTheShellIntegrationPrintsThePath(t *testing.T) {
	env, out, errb := testEnv(t)
	dir := t.TempDir()
	env.CDFile = ""
	env.Store.Put(slots.Slot{Number: 7, Path: dir})

	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != dir {
		t.Fatalf("stdout = %q, want the path", out.String())
	}
	if !strings.Contains(errb.String(), "dl init") {
		t.Fatalf("stderr = %q, want a hint about dl init", errb.String())
	}
}

func TestCDOnADeadPathFails(t *testing.T) {
	env, _, _ := testEnv(t)
	env.CDFile = filepath.Join(t.TempDir(), "cdfile")
	env.Store.Put(slots.Slot{Number: 7, Path: "/definitely/not/here"})

	err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"})
	if err == nil {
		t.Fatal("cd into a dead path = nil error, want an error")
	}
	if _, statErr := os.Stat(env.CDFile); statErr == nil {
		t.Fatal("the cd file was written despite the failure")
	}
}

func TestCDOnAnEmptySlotFails(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionCD, Ref: "7"}); err == nil {
		t.Fatal("cd into an empty slot = nil error, want an error")
	}
}

func TestPathPrintsOnlyThePath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "a note that must not be printed"})

	if err := Run(env, cli.Command{Action: cli.ActionPath, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "/tmp/x\n" {
		t.Fatalf("path output = %q, want %q", out.String(), "/tmp/x\n")
	}
}

// --path is for scripting, so it must work even when the directory is gone;
// the caller decides what that means.
func TestPathWorksOnADeadPath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/gone"})
	if err := Run(env, cli.Command{Action: cli.ActionPath, Ref: "7"}); err != nil {
		t.Fatalf("path on a dead slot returned an error: %v", err)
	}
	if out.String() != "/gone\n" {
		t.Fatalf("output = %q", out.String())
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -v`
Expected: build failure, `undefined: Env`

- [ ] **Step 3: Write `internal/actions/env.go`**

```go
// Package actions implements one function per user-visible operation. Every
// action takes its dependencies through Env rather than reaching for globals,
// so each one is testable with a temp store and a byte buffer. No action calls
// os.Exit or prints to os.Stdout directly; that is main's job.
//
// Phase 2's TUI will call these same functions, which is why they do not
// assume they are running in a one-shot process.
package actions

import (
	"fmt"
	"io"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Env carries everything an action needs from the outside world.
type Env struct {
	Store *slots.Store
	Out   io.Writer // normal output
	Err   io.Writer // hints and warnings; never parsed by anything

	Cwd  string // the directory --set records
	Home string // used to shorten paths for display

	// CDFile is $DL_CD_FILE: the path the shell wrapper reads to perform the
	// cd. Empty means the integration is not installed.
	CDFile string

	Editor editor.Editor
	Now    func() time.Time // injected so tests get stable timestamps
}

// now returns the current time through the injected clock.
func (e *Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

// Run dispatches a parsed command to its action. Keeping the switch here
// rather than in main means the TUI can reuse it unchanged.
func Run(env *Env, cmd cli.Command) error {
	switch cmd.Action {
	case cli.ActionCD:
		return CD(env, cmd)
	case cli.ActionSet:
		return Set(env, cmd)
	case cli.ActionSee:
		return See(env, cmd)
	case cli.ActionEdit:
		return Edit(env, cmd)
	case cli.ActionReset:
		return Reset(env, cmd)
	case cli.ActionDelete:
		return Delete(env, cmd)
	case cli.ActionPath:
		return Path(env, cmd)
	case cli.ActionRename:
		return Rename(env, cmd)
	case cli.ActionSetNote:
		return SetNote(env, cmd)
	case cli.ActionDoctor:
		return Doctor(env, cmd)
	}
	return fmt.Errorf("action %v is not implemented", cmd.Action)
}

// saveChanges persists the store. Actions call it once, at the end.
func saveChanges(env *Env) error {
	return env.Store.Save()
}
```

- [ ] **Step 4: Write `internal/actions/cd.go`**

```go
package actions

import (
	"fmt"
	"os"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// CD resolves a reference and hands the target directory to the shell wrapper
// through $DL_CD_FILE, then prints the slot's note.
//
// Printing the note is the feature the old tool only half had: you jump back
// into a project and immediately read what you were in the middle of.
func CD(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	if !sl.Exists() {
		return fmt.Errorf("slot %d points at %s, which no longer exists; repoint it with: dl -z %d",
			sl.Number, sl.Path, sl.Number)
	}

	if env.CDFile == "" {
		// No shell integration. Print the path so the user can still use it,
		// and say how to make the cd automatic.
		fmt.Fprintln(env.Out, sl.Path)
		fmt.Fprintln(env.Err, `hint: add eval "$(dl init zsh)" to your shell rc to cd automatically`)
		return nil
	}
	if err := os.WriteFile(env.CDFile, []byte(sl.Path), 0o600); err != nil {
		return fmt.Errorf("write the cd file: %w", err)
	}

	if sl.HasNote() {
		fmt.Fprintln(env.Out, sl.Note)
	}
	return nil
}
```

- [ ] **Step 5: Write `internal/actions/path.go`**

```go
package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// Path prints a slot's path and nothing else, so it composes:
//
//	cp report.pdf "$(dl -p 7)"
//
// It deliberately succeeds on a dead path: the caller decides what a missing
// directory means for their command.
func Path(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Out, sl.Path)
	return nil
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/actions/ -run 'TestCD|TestPath' -v`
Expected: build failure on `Set`, `See`, etc. — add temporary stubs returning
`nil` for the actions built in Tasks 11-13, then re-run. Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/actions
git commit -m "feat(actions): add the action env, --cd and --path"
```

---

### Task 11: --set

**Files:**
- Create: `internal/actions/set.go`
- Test: `internal/actions/set_test.go`

- [ ] **Step 1: Write the failing test**

```go
package actions

import (
	"errors"
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestSetSavesTheCurrentDirectory(t *testing.T) {
	env, out, _ := testEnv(t)
	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7"})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := env.Store.Get(7)
	if !ok {
		t.Fatal("slot 7 was not created")
	}
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the cwd", got.Path)
	}
	if got.Updated != env.Now() {
		t.Fatalf("Updated = %v, want the injected clock value", got.Updated)
	}
	if !strings.Contains(out.String(), "slot 7") {
		t.Fatalf("output = %q, want a confirmation naming the slot", out.String())
	}
}

func TestSetWithNoRefUsesTheFirstFreeSlot(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Path: "/a"})
	env.Store.Put(slots.Slot{Number: 2, Path: "/b"})

	if err := Run(env, cli.Command{Action: cli.ActionSet}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Store.Get(3); !ok {
		t.Fatal("slot 3 was not chosen as the first free slot")
	}
}

// The regression test for the destructive bug in the zsh version: overwriting
// a slot must keep the name and the note.
func TestSetKeepsTheNameAndNoteOnOverwrite(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/old/path", Note: "where I left off"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the new cwd", got.Path)
	}
	if got.Name != "exam42" {
		t.Fatalf("Name = %q, want it kept", got.Name)
	}
	if got.Note != "where I left off" {
		t.Fatalf("Note = %q, want it kept", got.Note)
	}
	// The user must be told what moved and that the note survived.
	if !strings.Contains(out.String(), "/old/path") {
		t.Errorf("output = %q, want the previous path", out.String())
	}
	if !strings.Contains(out.String(), "kept") {
		t.Errorf("output = %q, want a note-kept line", out.String())
	}
}

// Re-saving the same directory is routine; it should not print a "was" line.
func TestSetOnTheSamePathIsQuiet(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/cwd", Note: "n"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "was") {
		t.Fatalf("output = %q, want no \"was\" line when the path is unchanged", out.String())
	}
}

func TestSetWithReset(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/old", Note: "stale"})

	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Reset: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Name != "" || got.Note != "" {
		t.Fatalf("after -r: Name = %q, Note = %q, want both empty", got.Name, got.Note)
	}
}

func TestSetWithName(t *testing.T) {
	env, _, _ := testEnv(t)
	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Name: "exam42"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Name != "exam42" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestSetWithNote(t *testing.T) {
	env, _, _ := testEnv(t)
	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Note: "fix the loop"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Note != "fix the loop" {
		t.Fatalf("Note = %q", got.Note)
	}
}

// -r runs before -n and -m, so `dl -z 7 -r -n new` leaves the new name in
// place rather than wiping it.
func TestSetResetThenNameKeepsTheNewName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "old", Path: "/old", Note: "stale"})

	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Reset: true, Name: "new"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Name != "new" {
		t.Fatalf("Name = %q, want %q", got.Name, "new")
	}
	if got.Note != "" {
		t.Fatalf("Note = %q, want empty", got.Note)
	}
}

func TestSetWithEditOpensTheEditorOnTheCurrentNote(t *testing.T) {
	env, _, _ := testEnv(t)
	fake := &editor.Fake{Result: "brand new note"}
	env.Editor = fake
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/cwd", Note: "existing"})

	err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	if fake.Seen != "existing" {
		t.Fatalf("editor was handed %q, want the existing note", fake.Seen)
	}
	if got, _ := env.Store.Get(7); got.Note != "brand new note" {
		t.Fatalf("Note = %q", got.Note)
	}
}

// If the editor fails, the path was still saved but the note must survive.
func TestSetWithEditKeepsTheNoteWhenTheEditorFails(t *testing.T) {
	env, _, errb := testEnv(t)
	env.Editor = &editor.Fake{Err: errors.New("editor exploded")}
	env.Store.Put(slots.Slot{Number: 7, Path: "/old", Note: "precious"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "7", Edit: true}); err != nil {
		t.Fatalf("set should not fail because the editor did: %v", err)
	}
	got, _ := env.Store.Get(7)
	if got.Note != "precious" {
		t.Fatalf("Note = %q, want it untouched", got.Note)
	}
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the save to have happened anyway", got.Path)
	}
	if !strings.Contains(errb.String(), "editor") {
		t.Fatalf("stderr = %q, want a warning about the editor", errb.String())
	}
}

func TestSetRejectsAnOutOfRangeSlot(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "0"}); err == nil {
		t.Fatal("slot 0 was accepted")
	}
	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "1001"}); err == nil {
		t.Fatal("slot 1001 was accepted")
	}
}

// A name may be used as the ref for --set, so `dl -z exam42` re-points an
// existing named slot without having to remember its number.
func TestSetAcceptsANameAsTheRef(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/old"})

	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "exam42"}); err != nil {
		t.Fatal(err)
	}
	got, _ := env.Store.Get(7)
	if got.Path != "/tmp/cwd" {
		t.Fatalf("Path = %q, want the cwd", got.Path)
	}
}

func TestSetRejectsAnUnknownName(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionSet, Ref: "nope"}); err == nil {
		t.Fatal("--set with an unknown name was accepted")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run TestSet -v`
Expected: failures — the stub `Set` does nothing

- [ ] **Step 3: Write `internal/actions/set.go`**

```go
package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Set saves the current directory into a slot.
//
// An occupied slot is overwritten without asking — that is the design choice
// that keeps the common case one command long. What makes it safe is that
// nothing is ever destroyed implicitly: the name and the note survive the
// overwrite, and only an explicit -r or --delete removes them.
func Set(env *Env, cmd cli.Command) error {
	number, err := resolveSetTarget(env, cmd.Ref)
	if err != nil {
		return err
	}

	sl, existed := env.Store.Get(number)
	previousPath := sl.Path

	sl.Number = number
	sl.Path = env.Cwd
	sl.Updated = env.now()

	// -r runs first so that a -n or -m in the same command survives it.
	if cmd.Reset {
		sl.Name, sl.Note = "", ""
	}
	if cmd.Name != "" {
		sl.Name = cmd.Name
	}
	if cmd.Note != "" {
		sl.Note = cmd.Note
	}

	// The editor runs before the save so a failed edit cannot leave a
	// half-written note on disk.
	if cmd.Edit {
		edited, editErr := env.Editor.Edit(sl.Note)
		if editErr != nil {
			// A broken editor must not cost the user the save they asked for.
			fmt.Fprintf(env.Err, "warning: the editor failed, the note is unchanged: %v\n", editErr)
		} else {
			sl.Note = edited
		}
	}

	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}

	fmt.Fprintf(env.Out, "slot %d -> %s\n", number, slots.ShortPath(sl.Path, env.Home))
	// Only report a replacement when something actually moved.
	if existed && previousPath != "" && previousPath != sl.Path {
		fmt.Fprintf(env.Out, "  was %s\n", slots.ShortPath(previousPath, env.Home))
		if !cmd.Reset && (sl.Name != "" || sl.HasNote()) {
			fmt.Fprintf(env.Out, "  name and note kept — dl -e %d to update, dl -z %d -r to start clean\n",
				number, number)
		}
	}
	return nil
}

// resolveSetTarget turns --set's optional ref into a slot number.
//
// Unlike the other actions it accepts a number for a slot that does not exist
// yet, because creating slot 7 out of nothing is the normal case. A name, on
// the other hand, must already exist — there is nothing to name yet otherwise.
func resolveSetTarget(env *Env, ref string) (int, error) {
	if ref == "" {
		return env.Store.FirstFree()
	}
	if n, ok := slots.ParseSlotNumber(ref); ok {
		return n, nil
	}
	// Not a number: it has to be an existing name.
	sl, err := env.Store.Resolve(ref)
	if err != nil {
		return 0, fmt.Errorf("%w (a new slot must be given a number between %d and %d)",
			err, slots.MinSlot, slots.MaxSlot)
	}
	return sl.Number, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/actions/ -run TestSet -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/actions/set.go internal/actions/set_test.go
git commit -m "feat(actions): add --set with non-destructive overwrite"
```

---

### Task 12: --see

The listing is the only view of the store until the TUI lands, so its format is
pinned by exact-string tests rather than "contains" assertions.

```
12 slots

  *  7  exam42      ~/Tocuments/wpath/42/exams/exam5/solutions/s4/level1
     8  home        ~
  x 12  scraping    ~/Tocuments/wbiz/ou.you/dev/scrapping
```

The leading marker is `*` for a slot with a note, `x` for a path that no longer
exists, and a space otherwise. A dead path takes priority over the note marker,
because the broken thing is the one worth seeing.

**Files:**
- Create: `internal/actions/see.go`
- Test: `internal/actions/see_test.go`

- [ ] **Step 1: Write the failing test**

```go
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

func TestSeeLayout(t *testing.T) {
	env, out, _ := testEnv(t)
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
	env.Store.Put(slots.Slot{Number: 1, Name: "gone", Path: "/definitely/not/here", Note: "n"})
	real := t.TempDir()
	env.Store.Put(slots.Slot{Number: 2, Name: "here", Path: real})

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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run TestSee -v`
Expected: failures — the stub `See` prints nothing

- [ ] **Step 3: Write `internal/actions/see.go`**

```go
package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Row markers.
const (
	markerNote = "*" // the slot carries a note
	markerDead = "x" // the path no longer exists
	markerNone = " "
)

// See lists every slot in number order.
func See(env *Env, cmd cli.Command) error {
	all := env.Store.All()

	// --quiet is for pipes: full paths, nothing else, no header, and no
	// "nothing here" message that a downstream command would choke on.
	if cmd.Quiet {
		for _, sl := range all {
			fmt.Fprintln(env.Out, sl.Path)
		}
		return nil
	}

	if len(all) == 0 {
		fmt.Fprintln(env.Out, "no slots yet — save one with: dl -z")
		return nil
	}

	fmt.Fprintf(env.Out, "%d slot%s\n\n", len(all), plural(len(all)))

	numWidth, nameWidth := columnWidths(all)
	for i, sl := range all {
		fmt.Fprintf(env.Out, "  %s %*d  %-*s  %s\n",
			marker(sl),
			numWidth, sl.Number,
			nameWidth, sl.DisplayName(),
			slots.ShortPath(sl.Path, env.Home),
		)
		if !cmd.Long {
			continue
		}
		// In long form the note sits under its row, indented past the name
		// column, with a blank line before the next slot.
		if sl.HasNote() {
			// Indent past the name column: 2 (margin) + 1 (marker) +
			// 1 (space) + the number column + 2.
			for _, line := range strings.Split(sl.Note, "\n") {
				fmt.Fprintf(env.Out, "%*s%s\n", numWidth+6, "", line)
			}
		}
		if i < len(all)-1 {
			fmt.Fprintln(env.Out)
		}
	}
	return nil
}

// marker is the one-character state flag at the start of a row. A dead path
// outranks a note: the broken thing is the one worth spotting.
func marker(sl slots.Slot) string {
	switch {
	case !sl.Exists():
		return markerDead
	case sl.HasNote():
		return markerNote
	default:
		return markerNone
	}
}

// columnWidths measures the number and name columns so the table lines up
// whatever the data.
func columnWidths(all []slots.Slot) (numWidth, nameWidth int) {
	for _, sl := range all {
		if w := len(strconv.Itoa(sl.Number)); w > numWidth {
			numWidth = w
		}
		if w := len([]rune(sl.DisplayName())); w > nameWidth {
			nameWidth = w
		}
	}
	return numWidth, nameWidth
}

// plural returns the "s" for a count.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/actions/ -run TestSee -v`
Expected: all PASS. If a width assertion fails, print `out.String()` and compare
against the format string above rather than adjusting the test to match a bug.

- [ ] **Step 5: Commit**

```bash
git add internal/actions/see.go internal/actions/see_test.go
git commit -m "feat(actions): add --see with aligned columns and state markers"
```

---

### Task 13: --edit, --reset, --delete, --name, --note and doctor

`doctor` is built here rather than in phase 3: it is fifteen lines, and leaving
it parsed-but-unimplemented would be a trap.

**Files:**
- Create: `internal/actions/note.go`, `internal/actions/doctor.go`
- Test: `internal/actions/note_test.go`, `internal/actions/doctor_test.go`

- [ ] **Step 1: Write the failing test for the note actions**

```go
package actions

import (
	"errors"
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestEditOpensTheEditorAndStoresTheResult(t *testing.T) {
	env, _, _ := testEnv(t)
	fake := &editor.Fake{Result: "rewritten"}
	env.Editor = fake
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "before"})

	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if fake.Seen != "before" {
		t.Fatalf("editor was handed %q, want the existing note", fake.Seen)
	}
	if got, _ := env.Store.Get(7); got.Note != "rewritten" {
		t.Fatalf("Note = %q", got.Note)
	}
}

// --edit must not touch the path or the timestamp of the directory itself;
// only the note changes.
func TestEditDoesNotMoveTheSlot(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Editor = &editor.Fake{Result: "x"}
	env.Store.Put(slots.Slot{Number: 7, Path: "/original", Note: ""})

	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Path != "/original" {
		t.Fatalf("Path = %q, want it unchanged", got.Path)
	}
}

func TestEditFailsWhenTheEditorFails(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Editor = &editor.Fake{Err: errors.New("no editor")}
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "precious"})

	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err == nil {
		t.Fatal("edit = nil error after an editor failure, want an error")
	}
	if got, _ := env.Store.Get(7); got.Note != "precious" {
		t.Fatalf("Note = %q, want it untouched", got.Note)
	}
}

func TestEditOnAnEmptySlotFails(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionEdit, Ref: "7"}); err == nil {
		t.Fatal("editing an empty slot = nil error, want an error")
	}
}

func TestResetClearsTheNameAndNoteButKeepsThePath(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/keep/me", Note: "stale"})

	if err := Run(env, cli.Command{Action: cli.ActionReset, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	got, ok := env.Store.Get(7)
	if !ok {
		t.Fatal("the slot was removed; --reset must keep it")
	}
	if got.Path != "/keep/me" {
		t.Fatalf("Path = %q, want it kept", got.Path)
	}
	if got.Name != "" || got.Note != "" {
		t.Fatalf("Name = %q, Note = %q, want both empty", got.Name, got.Note)
	}
	if !strings.Contains(out.String(), "slot 7") {
		t.Fatalf("output = %q, want a confirmation", out.String())
	}
}

func TestDeleteRemovesTheSlot(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionDelete, Ref: "7"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Store.Get(7); ok {
		t.Fatal("slot 7 is still there")
	}
	if !strings.Contains(out.String(), "deleted") {
		t.Fatalf("output = %q, want a confirmation", out.String())
	}
}

func TestDeleteOnAnEmptySlotFails(t *testing.T) {
	env, _, _ := testEnv(t)
	if err := Run(env, cli.Command{Action: cli.ActionDelete, Ref: "7"}); err == nil {
		t.Fatal("deleting an empty slot = nil error, want an error")
	}
}

func TestRename(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "exam42"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Name != "exam42" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestRenameWithAnEmptyNameDropsTheName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/tmp/x"})

	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: ""}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Name != "" {
		t.Fatalf("Name = %q, want empty", got.Name)
	}
}

// A name that looks like a slot number would make the ref grammar ambiguous
// for the user even though the parser handles it, so it is rejected outright.
func TestRenameRejectsANumericName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x"})
	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "42"}); err == nil {
		t.Fatal("a purely numeric name was accepted")
	}
}

func TestRenameRejectsADuplicateName(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/a"})
	env.Store.Put(slots.Slot{Number: 8, Path: "/b"})

	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "8", Name: "EXAM42"}); err == nil {
		t.Fatal("a duplicate name was accepted; refs would become ambiguous")
	}
}

// Renaming a slot to the name it already has is a no-op, not a duplicate.
func TestRenameToTheSameNameSucceeds(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Name: "exam42", Path: "/a"})
	if err := Run(env, cli.Command{Action: cli.ActionRename, Ref: "7", Name: "exam42"}); err != nil {
		t.Fatalf("renaming to the same name failed: %v", err)
	}
}

func TestSetNoteReplacesTheNote(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "old"})

	err := Run(env, cli.Command{Action: cli.ActionSetNote, Ref: "7", Note: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Note != "new" {
		t.Fatalf("Note = %q", got.Note)
	}
}

func TestSetNoteWithEmptyTextClearsIt(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 7, Path: "/tmp/x", Note: "old"})

	if err := Run(env, cli.Command{Action: cli.ActionSetNote, Ref: "7", Note: ""}); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.Store.Get(7); got.Note != "" {
		t.Fatalf("Note = %q, want empty", got.Note)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/actions/ -run 'TestEdit|TestReset|TestDelete|TestRename|TestSetNote' -v`
Expected: failures — the stubs do nothing

- [ ] **Step 3: Write `internal/actions/note.go`**

```go
package actions

import (
	"fmt"
	"strings"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Edit opens a slot's note in the user's editor. Only the note changes: the
// path and the name are left exactly as they were.
func Edit(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	edited, err := env.Editor.Edit(sl.Note)
	if err != nil {
		// Unlike --set, editing is the entire point of this command, so a
		// failed editor is a failed command.
		return fmt.Errorf("the note is unchanged: %w", err)
	}
	sl.Note = edited
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	if sl.HasNote() {
		fmt.Fprintf(env.Out, "slot %d note saved\n", sl.Number)
	} else {
		fmt.Fprintf(env.Out, "slot %d note is now empty\n", sl.Number)
	}
	return nil
}

// Reset clears a slot's name and note while keeping where it points. It is the
// "same shelf, new project" command.
func Reset(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	sl.Name, sl.Note = "", ""
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "slot %d name and note cleared, still pointing at %s\n",
		sl.Number, slots.ShortPath(sl.Path, env.Home))
	return nil
}

// Delete removes a slot outright. It is the only command that does.
func Delete(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	env.Store.Delete(sl.Number)
	if err := saveChanges(env); err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "slot %d deleted (%s)\n", sl.Number, slots.ShortPath(sl.Path, env.Home))
	return nil
}

// Rename sets or clears a slot's name.
func Rename(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(cmd.Name)
	if err := validateName(env, sl.Number, name); err != nil {
		return err
	}
	sl.Name = name
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	if name == "" {
		fmt.Fprintf(env.Out, "slot %d name removed\n", sl.Number)
	} else {
		fmt.Fprintf(env.Out, "slot %d is now %q\n", sl.Number, name)
	}
	return nil
}

// SetNote replaces a slot's note with the given text.
func SetNote(env *Env, cmd cli.Command) error {
	sl, err := env.Store.Resolve(cmd.Ref)
	if err != nil {
		return err
	}
	sl.Note = strings.TrimRight(cmd.Note, " \t\r\n")
	env.Store.Put(sl)
	if err := saveChanges(env); err != nil {
		return err
	}
	if sl.HasNote() {
		fmt.Fprintf(env.Out, "slot %d note set\n", sl.Number)
	} else {
		fmt.Fprintf(env.Out, "slot %d note cleared\n", sl.Number)
	}
	return nil
}

// validateName rejects names that would make references ambiguous: a purely
// numeric name would collide with slot numbers, and a duplicate would make
// both slots unreachable by name.
func validateName(env *Env, number int, name string) error {
	if name == "" {
		return nil
	}
	if _, numeric := slots.ParseSlotNumber(name); numeric {
		return fmt.Errorf("%q is a slot number, not a usable name", name)
	}
	if strings.ContainsAny(name, " \t") {
		return fmt.Errorf("a name cannot contain spaces")
	}
	for _, other := range env.Store.All() {
		if other.Number != number && strings.EqualFold(other.Name, name) {
			return fmt.Errorf("slot %d is already named %q", other.Number, other.Name)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the note tests**

Run: `go test ./internal/actions/ -run 'TestEdit|TestReset|TestDelete|TestRename|TestSetNote' -v`
Expected: all PASS

- [ ] **Step 5: Write the failing doctor test**

```go
package actions

import (
	"strings"
	"testing"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

func TestDoctorReportsOnlyDeadPaths(t *testing.T) {
	env, out, _ := testEnv(t)
	alive := t.TempDir()
	env.Store.Put(slots.Slot{Number: 1, Name: "alive", Path: alive})
	env.Store.Put(slots.Slot{Number: 2, Name: "dead", Path: "/definitely/not/here"})

	if err := Run(env, cli.Command{Action: cli.ActionDoctor}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "/definitely/not/here") {
		t.Errorf("output = %q, want the dead path", got)
	}
	if strings.Contains(got, alive) {
		t.Errorf("output = %q, must not list the live path", got)
	}
	// The report must say how to fix each one.
	if !strings.Contains(got, "dl -z 2") || !strings.Contains(got, "dl -d 2") {
		t.Errorf("output = %q, want repair hints", got)
	}
}

func TestDoctorOnAHealthyStore(t *testing.T) {
	env, out, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 1, Path: t.TempDir()})
	if err := Run(env, cli.Command{Action: cli.ActionDoctor}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "every slot") {
		t.Fatalf("output = %q, want an all-clear message", out.String())
	}
}

// doctor only reports; it must never modify the store.
func TestDoctorChangesNothing(t *testing.T) {
	env, _, _ := testEnv(t)
	env.Store.Put(slots.Slot{Number: 2, Path: "/gone"})
	if err := Run(env, cli.Command{Action: cli.ActionDoctor}); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Store.Get(2); !ok {
		t.Fatal("doctor removed a slot; it must only report")
	}
}
```

- [ ] **Step 6: Write `internal/actions/doctor.go`**

```go
package actions

import (
	"fmt"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Doctor reports slots whose directory has disappeared. It only reports:
// deciding whether a moved project should be repointed or dropped is the
// user's call, not the tool's.
func Doctor(env *Env, cmd cli.Command) error {
	var dead []slots.Slot
	for _, sl := range env.Store.All() {
		if !sl.Exists() {
			dead = append(dead, sl)
		}
	}
	if len(dead) == 0 {
		fmt.Fprintln(env.Out, "every slot points at a directory that exists")
		return nil
	}

	fmt.Fprintf(env.Out, "%d slot%s point at a missing directory\n\n", len(dead), plural(len(dead)))
	for _, sl := range dead {
		fmt.Fprintf(env.Out, "  %d  %s\n", sl.Number, sl.DisplayName())
		fmt.Fprintf(env.Out, "     %s\n", sl.Path)
		fmt.Fprintf(env.Out, "     repoint: cd <somewhere> && dl -z %d      drop: dl -d %d\n\n",
			sl.Number, sl.Number)
	}
	return nil
}
```

- [ ] **Step 7: Run the whole actions suite**

Run: `go test ./internal/actions/ -v`
Expected: all PASS. Remove any leftover stubs from Task 10 Step 6.

- [ ] **Step 8: Commit**

```bash
git add internal/actions/note.go internal/actions/note_test.go internal/actions/doctor.go internal/actions/doctor_test.go
git commit -m "feat(actions): add --edit, --reset, --delete, --name, --note and doctor"
```

---

### Task 14: Shell integration

**Files:**
- Create: `internal/shellinit/shellinit.go`, `internal/shellinit/zsh.sh`, `internal/shellinit/bash.sh`, `internal/shellinit/fish.fish`
- Test: `internal/shellinit/shellinit_test.go`

- [ ] **Step 1: Write the failing test**

```go
package shellinit

import (
	"os/exec"
	"strings"
	"testing"
)

func TestScriptForEachShell(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		got, err := Script(shell)
		if err != nil {
			t.Fatalf("Script(%q): %v", shell, err)
		}
		if !strings.Contains(got, "DL_CD_FILE") {
			t.Errorf("%s script does not set DL_CD_FILE", shell)
		}
		if !strings.Contains(got, "command dl") {
			t.Errorf("%s script does not call the binary with `command`, so it would recurse", shell)
		}
	}
}

func TestScriptRejectsAnUnknownShell(t *testing.T) {
	if _, err := Script("csh"); err == nil {
		t.Fatal("Script(\"csh\") = nil error, want an error")
	}
}

// The emitted shell code must actually be valid. Skip a shell that is not
// installed rather than failing on a machine that lacks it.
func TestScriptsAreSyntacticallyValid(t *testing.T) {
	cases := []struct{ shell, bin string; args []string }{
		{"bash", "bash", []string{"-n"}},
		{"zsh", "zsh", []string{"-n"}},
		{"fish", "fish", []string{"--no-execute"}},
	}
	for _, c := range cases {
		t.Run(c.shell, func(t *testing.T) {
			bin, err := exec.LookPath(c.bin)
			if err != nil {
				t.Skipf("%s is not installed", c.bin)
			}
			script, err := Script(c.shell)
			if err != nil {
				t.Fatal(err)
			}
			f := t.TempDir() + "/init"
			if err := os.WriteFile(f, []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(bin, append(c.args, f)...).CombinedOutput()
			if err != nil {
				t.Fatalf("%s syntax check failed: %v\n%s", c.shell, err, out)
			}
		})
	}
}
```

The test file imports `os`, `os/exec`, `strings` and `testing`.

- [ ] **Step 2: Write `internal/shellinit/zsh.sh`**

```sh
# dl shell integration for zsh.
#
# A program cannot change its parent shell's directory, so dl writes the target
# path into the file named by $DL_CD_FILE and this function performs the cd.
# A temp file is used rather than capturing stdout because capturing stdout
# would break the interactive browser, which needs the real terminal.
dl() {
  local cdfile rc
  cdfile="$(mktemp "${TMPDIR:-/tmp}/dl-cd.XXXXXX")" || return 1
  DL_CD_FILE="$cdfile" command dl "$@"
  rc=$?
  if [[ -s "$cdfile" ]]; then
    builtin cd -- "$(<"$cdfile")" || rc=$?
  fi
  command rm -f "$cdfile"
  return $rc
}
```

- [ ] **Step 3: Write `internal/shellinit/bash.sh`**

```sh
# dl shell integration for bash. See the zsh version for why a temp file is
# used instead of capturing stdout.
dl() {
  local cdfile rc
  cdfile="$(mktemp "${TMPDIR:-/tmp}/dl-cd.XXXXXX")" || return 1
  DL_CD_FILE="$cdfile" command dl "$@"
  rc=$?
  if [ -s "$cdfile" ]; then
    builtin cd -- "$(cat "$cdfile")" || rc=$?
  fi
  command rm -f "$cdfile"
  return $rc
}
```

- [ ] **Step 4: Write `internal/shellinit/fish.fish`**

```fish
# dl shell integration for fish. See the zsh version for why a temp file is
# used instead of capturing stdout.
function dl
    set -l cdfile (mktemp)
    DL_CD_FILE=$cdfile command dl $argv
    set -l rc $status
    if test -s $cdfile
        builtin cd (cat $cdfile); or set rc $status
    end
    command rm -f $cdfile
    return $rc
end
```

- [ ] **Step 5: Write `internal/shellinit/shellinit.go`**

```go
// Package shellinit serves the shell functions that make `dl <ref>` actually
// move the user's shell. The scripts are embedded so the binary stays a single
// file with nothing to install alongside it.
package shellinit

import (
	"embed"
	"fmt"
)

//go:embed zsh.sh bash.sh fish.fish
var files embed.FS

// scriptFiles maps a shell name to its embedded file.
var scriptFiles = map[string]string{
	"zsh":  "zsh.sh",
	"bash": "bash.sh",
	"fish": "fish.fish",
}

// Script returns the integration snippet for a shell.
func Script(shell string) (string, error) {
	name, ok := scriptFiles[shell]
	if !ok {
		return "", fmt.Errorf("unknown shell %q; supported: bash, fish, zsh", shell)
	}
	raw, err := files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read the embedded %s script: %w", shell, err)
	}
	return string(raw), nil
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/shellinit/ -v`
Expected: all PASS (fish likely SKIPped if it is not installed)

- [ ] **Step 7: Commit**

```bash
git add internal/shellinit
git commit -m "feat(shellinit): embed the zsh, bash and fish integrations"
```

---

### Task 15: Wiring main

**Files:**
- Modify: `cmd/dl/main.go` (replace the Task 1 stub entirely)

- [ ] **Step 1: Write `cmd/dl/main.go`**

```go
// Command dl bookmarks directories in numbered slots and keeps a free-form
// note on each one.
//
// This file is wiring only: parse argv, build the environment, dispatch, and
// turn errors into exit codes. All behaviour lives in internal/actions so that
// the interactive browser can reuse it unchanged.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Decapix/dl-organisation/internal/actions"
	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/shellinit"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

// Exit codes. The shell wrapper propagates these, so `dl 7 && make` behaves.
const (
	exitOK      = 0
	exitFailure = 1 // a runtime failure: bad ref, dead path, store I/O
	exitUsage   = 2 // the command was typed wrong
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is main's body, extracted so the end-to-end test can call it directly.
func run(argv []string) int {
	cmd, err := cli.Parse(argv)
	if err != nil {
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "dl: %s\n", ue.Msg)
			fmt.Fprintf(os.Stderr, "see: dl %s--help\n", helpPointer(ue.Action))
			return exitUsage
		}
		fmt.Fprintf(os.Stderr, "dl: %v\n", err)
		return exitUsage
	}

	// These three answer without touching the store, so they keep working even
	// when the config directory is unreadable.
	switch cmd.Action {
	case cli.ActionHelp:
		fmt.Fprint(os.Stdout, cli.Help(cmd.HelpFor))
		return exitOK
	case cli.ActionVersion:
		fmt.Fprintf(os.Stdout, "dl %s\n", version)
		return exitOK
	case cli.ActionInit:
		script, err := shellinit.Script(cmd.Shell)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dl: %v\n", err)
			return exitFailure
		}
		fmt.Fprint(os.Stdout, script)
		return exitOK
	}

	if err := runStoreCommand(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "dl: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// runStoreCommand opens the store and dispatches everything that needs it.
func runStoreCommand(cmd cli.Command) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locate the home directory: %w", err)
	}
	dir := slots.DefaultDir(os.Getenv, home)

	// One-time import of the legacy ~/.cdl layout. It is a no-op once the new
	// store exists, so calling it on every run costs a single stat.
	if n, err := slots.Migrate(home, dir); err != nil {
		fmt.Fprintf(os.Stderr, "dl: could not import ~/.cdl: %v\n", err)
	} else if n > 0 {
		fmt.Fprintf(os.Stderr, "dl: imported %d slot%s from ~/.cdl (kept as ~/.cdl.bak)\n",
			n, pluralS(n))
	}

	// Read-only commands skip the write lock so they never block behind a
	// long-running edit in another shell.
	var store *slots.Store
	if writes(cmd.Action) {
		store, err = slots.OpenForUpdate(dir)
	} else {
		store, err = slots.Open(dir)
	}
	if err != nil {
		return err
	}
	defer store.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate the current directory: %w", err)
	}

	env := &actions.Env{
		Store:  store,
		Out:    os.Stdout,
		Err:    os.Stderr,
		Cwd:    cwd,
		Home:   home,
		CDFile: os.Getenv("DL_CD_FILE"),
		Editor: editor.OS{},
		Now:    time.Now,
	}

	// The interactive browser arrives in phase 2. Until then a bare `dl` does
	// the most useful thing it can, which is to list.
	if cmd.Action == cli.ActionTUI {
		cmd.Action = cli.ActionSee
	}
	return actions.Run(env, cmd)
}

// writes reports whether an action needs the write lock.
func writes(a cli.Action) bool {
	switch a {
	case cli.ActionSet, cli.ActionEdit, cli.ActionReset, cli.ActionDelete,
		cli.ActionRename, cli.ActionSetNote:
		return true
	}
	return false
}

// helpPointer renders the action name for the "see: dl ... --help" line.
// ActionTUI has no flag, so a bare `dl --help` is the right pointer.
func helpPointer(a cli.Action) string {
	if a == cli.ActionTUI {
		return ""
	}
	return a.String() + " "
}

// pluralS returns the "s" for a count.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

- [ ] **Step 2: Build and smoke-test against a throwaway store**

```bash
go build -o /tmp/dl ./cmd/dl
export DL_DIR=$(mktemp -d)
cd /tmp && /tmp/dl -z 1 -n scratch -m "first note"
/tmp/dl -s
/tmp/dl -s -l
/tmp/dl -p 1
/tmp/dl -h
/tmp/dl -z --help
/tmp/dl doctor
/tmp/dl init zsh
```

Expected: the save prints `slot 1 -> /tmp`, the listing shows `*  1  scratch`,
`-p` prints `/tmp`, both help pages render, `doctor` reports all clear, and
`init zsh` prints the function.

- [ ] **Step 3: Verify the exit codes**

```bash
/tmp/dl -x           ; echo "usage error -> $?"   # expect 2
/tmp/dl -c 999       ; echo "empty slot -> $?"    # expect 1
/tmp/dl -s           ; echo "success   -> $?"     # expect 0
```

Expected: `2`, `1`, `0`

- [ ] **Step 4: Verify the cd handshake by hand**

```bash
export DL_CD_FILE=$(mktemp)
/tmp/dl 1
cat "$DL_CD_FILE"    # expect /tmp
unset DL_CD_FILE
/tmp/dl 1            # expect the path on stdout and a `dl init` hint on stderr
```

- [ ] **Step 5: Commit**

```bash
git add cmd/dl/main.go
git commit -m "feat(cmd): wire the binary and map errors to exit codes"
```

---

### Task 16: End-to-end test

**Files:**
- Create: `cmd/dl/e2e_test.go`

- [ ] **Step 1: Write the test**

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureRun runs the program with argv and returns stdout, stderr and the
// exit code. It redirects the real os.Stdout/os.Stderr because run writes to
// them directly, which is exactly what the test needs to exercise.
func captureRun(t *testing.T, argv ...string) (string, string, int) {
	t.Helper()

	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	code := run(argv)

	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = origOut, origErr

	var outB, errB strings.Builder
	buf := make([]byte, 4096)
	for _, pair := range []struct {
		f *os.File
		b *strings.Builder
	}{{outR, &outB}, {errR, &errB}} {
		for {
			n, err := pair.f.Read(buf)
			pair.b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		pair.f.Close()
	}
	return outB.String(), errB.String(), code
}

// setupEnv points dl at a throwaway store and home for the duration of a test.
func setupEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DL_DIR", filepath.Join(home, ".config", "dl"))
	return home
}

// TestFullLifecycle walks the path a real user takes: save, list, note, jump,
// rename, jump by name, reset, delete.
func TestFullLifecycle(t *testing.T) {
	setupEnv(t)
	work := t.TempDir()
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}

	if out, errS, code := captureRun(t, "-z", "7"); code != 0 {
		t.Fatalf("save failed: code=%d out=%q err=%q", code, out, errS)
	}

	out, _, code := captureRun(t, "-s")
	if code != 0 || !strings.Contains(out, "1 slot") {
		t.Fatalf("list after save: code=%d out=%q", code, out)
	}

	if _, _, code := captureRun(t, "-m", "7", "pick up at step 3"); code != 0 {
		t.Fatalf("setting the note failed with code %d", code)
	}

	// Jumping writes the path to the cd file and prints the note.
	cdfile := filepath.Join(t.TempDir(), "cd")
	t.Setenv("DL_CD_FILE", cdfile)
	out, _, code = captureRun(t, "7")
	if code != 0 {
		t.Fatalf("cd failed with code %d", code)
	}
	if !strings.Contains(out, "pick up at step 3") {
		t.Fatalf("cd did not print the note: %q", out)
	}
	raw, err := os.ReadFile(cdfile)
	if err != nil || string(raw) != work {
		t.Fatalf("cd file = %q (err %v), want %q", string(raw), err, work)
	}

	if _, _, code := captureRun(t, "-n", "7", "project"); code != 0 {
		t.Fatalf("rename failed with code %d", code)
	}
	// The name and a unique prefix of it both resolve.
	for _, ref := range []string{"project", "proj"} {
		if _, _, code := captureRun(t, ref); code != 0 {
			t.Fatalf("cd by %q failed with code %d", ref, code)
		}
	}

	if _, _, code := captureRun(t, "-r", "7"); code != 0 {
		t.Fatalf("reset failed with code %d", code)
	}
	out, _, _ = captureRun(t, "-s", "-l")
	if strings.Contains(out, "pick up at step 3") {
		t.Fatalf("the note survived --reset: %q", out)
	}

	if _, _, code := captureRun(t, "-d", "7"); code != 0 {
		t.Fatalf("delete failed with code %d", code)
	}
	out, _, _ = captureRun(t, "-s")
	if !strings.Contains(out, "no slots yet") {
		t.Fatalf("store is not empty after delete: %q", out)
	}
}

func TestExitCodes(t *testing.T) {
	setupEnv(t)
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{"success", []string{"-s"}, 0},
		{"unknown flag", []string{"-x"}, 2},
		{"two actions", []string{"-s", "-d", "1"}, 2},
		{"missing ref", []string{"-d"}, 2},
		{"empty slot", []string{"-c", "500"}, 1},
		{"unknown name", []string{"nope"}, 1},
		{"help", []string{"-h"}, 0},
		{"version", []string{"--version"}, 0},
		{"init", []string{"init", "zsh"}, 0},
		{"unknown shell", []string{"init", "csh"}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, code := captureRun(t, c.argv...); code != c.want {
				t.Fatalf("exit code = %d, want %d", code, c.want)
			}
		})
	}
}

// A usage error must point at the relevant help page, not the global one.
func TestUsageErrorPointsAtTheActionHelp(t *testing.T) {
	setupEnv(t)
	_, errS, code := captureRun(t, "-z", "-l")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errS, "dl --set --help") {
		t.Fatalf("stderr = %q, want a pointer to the --set help", errS)
	}
}

// The legacy store must be imported on the first run of the real binary.
func TestMigrationHappensOnFirstRun(t *testing.T) {
	home := setupEnv(t)
	cdl := filepath.Join(home, ".cdl")
	if err := os.MkdirAll(filepath.Join(cdl, "descriptions"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cdl, "slots.json"),
		[]byte(`{"7": {"path": "/tmp", "comment": "old comment"}}`), 0o644)
	os.WriteFile(filepath.Join(cdl, "descriptions", "7.txt"),
		[]byte("old description"), 0o644)

	out, errS, code := captureRun(t, "-s", "-l")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errS)
	}
	if !strings.Contains(errS, "imported 1 slot") {
		t.Errorf("stderr = %q, want an import notice", errS)
	}
	for _, want := range []string{"old comment", "old description"} {
		if !strings.Contains(out, want) {
			t.Errorf("listing = %q, want it to contain %q", out, want)
		}
	}
	if _, err := os.Stat(cdl + ".bak"); err != nil {
		t.Errorf("~/.cdl.bak missing: %v", err)
	}
}
```

- [ ] **Step 2: Run the end-to-end suite**

Run: `go test ./cmd/dl/ -v`
Expected: all PASS

- [ ] **Step 3: Run everything, with the race detector**

Run: `go test ./... -race && go vet ./...`
Expected: all PASS, no vet findings

- [ ] **Step 4: Commit**

```bash
git add cmd/dl/e2e_test.go
git commit -m "test: add the end-to-end lifecycle and exit-code suite"
```

---

### Task 17: README and the switchover

**Files:**
- Create: `README.md`, `.github/workflows/ci.yml`

- [ ] **Step 1: Write `README.md`**

Structure it in this order, because that is the order a reader needs it:

1. One-sentence description and a terminal transcript showing save -> list ->
   jump-with-note.
2. **Install**: `go install github.com/Decapix/dl-organisation/cmd/dl@latest`,
   then `eval "$(dl init zsh)"` in `~/.zshrc`, with the bash and fish lines.
3. **Usage**: paste the output of `dl -h` verbatim.
4. **Notes**: explain that a slot holds a path, an optional name and a
   free-form note, and that `dl <ref>` prints the note when you arrive.
5. **Coming from stl/cdl/seedl**: the old -> new table from the spec, plus a
   line saying `~/.cdl` is imported automatically and kept as `~/.cdl.bak`.
6. **How the cd works**: the temp-file handshake, in three sentences.
7. **Storage**: `~/.config/dl/slots.json`, `$DL_DIR` to move it.
8. MIT licence line.

- [ ] **Step 2: Write `.github/workflows/ci.yml`**

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go vet ./...
      - run: go test ./... -race
      - run: go build ./cmd/dl
```

- [ ] **Step 3: Commit and push**

```bash
git add README.md .github
git commit -m "docs: add the README and CI workflow"
git push -u origin main
```

- [ ] **Step 4: Install the binary and switch the shell over**

```bash
go install ./cmd/dl
```

Then replace lines 95-311 of `~/.zshrc` — everything from the
`# CDL — directory bookmarks` banner through `unset i` — with:

```zsh
# dl — directory bookmarks with notes (github.com/Decapix/dl-organisation)
eval "$(dl init zsh)"
```

Keep the existing `~/.zshrc.bak-cdl` as the fallback.

- [ ] **Step 5: Verify the real migration in a fresh shell**

```bash
exec zsh
dl -s
```

Expected: the 12 real slots are listed, `~/.cdl.bak` exists, and slots 7 and 11
show their old descriptions under `dl -s -l`.

- [ ] **Step 6: Commit any README corrections the real run turned up**

```bash
git add -A && git commit -m "docs: correct the README against a real run"
git push
```

---

## Phase 1 done when

- `go test ./... -race` and `go vet ./...` are clean.
- All 9 problems from the spec's table are fixed: notes are visible (`-s -l`),
  names and notes are editable on their own (`-n`, `-m`, `-e`), overwriting
  keeps them, slots have names and prefix matching, no aliases are generated,
  `$EDITOR` is honoured, `doctor` reports dead paths, the listing aligns, and
  `jq` is gone.
- `~/.zshrc` is down to one line for this feature.

## Deferred to phase 2 and 3

Phase 2 is the two-pane browser (`internal/tui`), which replaces the
`ActionTUI -> ActionSee` fallback in `runStoreCommand`. Phase 3 is shell
completions, goreleaser and the README GIF. `doctor` moved into phase 1 because
shipping it parsed-but-unimplemented would have been worse than building it.
