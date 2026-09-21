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
