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
