package actions

import (
	"io"
	"time"

	"github.com/Decapix/dl-organisation/internal/cli"
	"github.com/Decapix/dl-organisation/internal/editor"
	"github.com/Decapix/dl-organisation/internal/slots"
)

// Session owns the store's lifecycle so that no caller has to.
//
// It opens the store for the duration of one command and closes it again. A
// one-shot CLI invocation could just as well hold it open, but the browser
// runs for minutes at a time, and an exclusive flock held across keystrokes
// would block every other shell. Both callers therefore go through the same
// type, and there is one place that decides when the lock is taken.
type Session struct {
	Dir  string // the store directory
	Home string // for shortening paths in output
	Cwd  string // what --set records

	// CDFile is $DL_CD_FILE: the file the shell wrapper reads to perform the
	// cd. Empty means the integration is not installed.
	CDFile string

	Editor editor.Editor
	Now    func() time.Time
	Err    io.Writer // warnings and hints
}

// Slots reads the store without taking the write lock and returns every slot
// in number order. It re-reads each time, so a save made in another shell
// shows up on the next call.
func (s *Session) Slots() ([]slots.Slot, error) {
	store, err := slots.Open(s.Dir)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.All(), nil
}

// Run executes one command, writing the action's output to out.
//
// The store is opened for update only when the action writes, so listing and
// jumping never wait behind an edit in progress elsewhere.
func (s *Session) Run(cmd cli.Command, out io.Writer) error {
	var (
		store *slots.Store
		err   error
	)
	if writesStore(cmd.Action) {
		store, err = slots.OpenForUpdate(s.Dir)
	} else {
		store, err = slots.Open(s.Dir)
	}
	if err != nil {
		return err
	}
	defer store.Close()

	return Run(&Env{
		Store:  store,
		Out:    out,
		Err:    s.Err,
		Cwd:    s.Cwd,
		Home:   s.Home,
		CDFile: s.CDFile,
		Editor: s.Editor,
		Now:    s.Now,
	}, cmd)
}

// CurrentDir reports the directory --set would record. It exists so that
// Session satisfies the browser's interface; the browser cannot read the
// struct field because that would make it depend on the concrete type.
func (s *Session) CurrentDir() string { return s.Cwd }

// HomeDir reports the home directory, used to shorten paths for display.
func (s *Session) HomeDir() string { return s.Home }

// writesStore reports whether an action needs the write lock.
func writesStore(a cli.Action) bool {
	switch a {
	case cli.ActionSet, cli.ActionEdit, cli.ActionReset, cli.ActionDelete,
		cli.ActionRename, cli.ActionSetNote, cli.ActionCompact, cli.ActionMove:
		return true
	}
	return false
}
