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
	dir  string
	data map[int]Slot
	lock *lockFile // nil when opened read-only
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
	return OpenForUpdateNotify(dir, nil)
}

// OpenForUpdateNotify is OpenForUpdate with a hook: onWait is called once if
// another process holds the lock, right before this one starts waiting for
// it. It is how the command line gets to print "waiting" instead of nothing.
func OpenForUpdateNotify(dir string, onWait func()) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	lk, err := acquireLock(filepath.Join(dir, LockFileName), onWait)
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

// Writable reports whether the store holds the write lock, i.e. it was opened
// with OpenForUpdate and not yet closed.
func (s *Store) Writable() bool { return s.lock != nil }

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
