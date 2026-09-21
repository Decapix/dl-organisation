package tui

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// Run opens the browser and blocks until the user leaves it.
//
// Anything the final action printed — the note a jump shows — is written to
// out after the alternate screen is torn down, so it is still on screen in the
// directory you land in.
func Run(s Session, out io.Writer) error {
	p := tea.NewProgram(New(s), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	if m, ok := final.(Model); ok && m.finalOutput != "" {
		fmt.Fprint(out, m.finalOutput)
	}
	return nil
}
