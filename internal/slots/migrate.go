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
