package tui

import (
	"io"

	"github.com/Decapix/dl-organisation/internal/cli"
)

// editCommand runs the --edit action with the terminal handed back to the
// editor.
//
// bubbletea's tea.Exec suspends the program, restores the terminal to its
// normal state, runs this, and then takes the screen back. That is why the
// editor can be a full-screen program like vim.
type editCommand struct {
	session Session
	ref     string
}

// Run performs the edit. The action's own editor attaches to os.Stdin and
// os.Stdout, which at this point are the real terminal bubbletea has just
// released, so the Set* methods below have nothing to do.
func (c *editCommand) Run() error {
	return c.session.Run(cli.Command{Action: cli.ActionEdit, Ref: c.ref}, io.Discard)
}

// SetStdin, SetStdout and SetStderr satisfy tea.ExecCommand. They are no-ops
// because the editor process is spawned by internal/editor against the real
// standard streams rather than through this interface.
func (c *editCommand) SetStdin(io.Reader)  {}
func (c *editCommand) SetStdout(io.Writer) {}
func (c *editCommand) SetStderr(io.Writer) {}

// editFinishedMsg is delivered once the editor exits.
type editFinishedMsg struct{ err error }
