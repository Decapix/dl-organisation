package tui

// View renders the browser. The real two-pane layout arrives in a later task;
// this is enough for the model tests to compile and run.
func (m Model) View() string {
	if m.mode == modeHelp {
		return helpOverlay
	}
	var b []byte
	for i, sl := range m.view {
		if i == m.cursor {
			b = append(b, '>')
		} else {
			b = append(b, ' ')
		}
		b = append(b, ' ')
		b = append(b, sl.DisplayName()...)
		b = append(b, '\n')
	}
	return string(b)
}
