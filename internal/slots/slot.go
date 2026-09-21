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
