package actions

import "github.com/charmbracelet/lipgloss"

// The two row colours of a listing: the terminal's own white and a soft grey.
// They alternate so the eye can follow a row across its columns. They are
// fixed values, not adaptive ones: an adaptive colour makes lipgloss ask the
// terminal for its background on every render, which is a round trip a
// listing does not need. White is ANSI 7, so it is whatever the user's theme
// calls white; the grey is a light 256-colour step below it.
var (
	rowColourA = lipgloss.Color("7")   // the terminal's basic white
	rowColourB = lipgloss.Color("247") // soft grey
)

// stripe colours a whole line for row i of a listing. Without a renderer the
// line comes back untouched, which is what pipes and tests get.
func (e *Env) stripe(i int, line string) string {
	if e.Renderer == nil {
		return line
	}
	colour := rowColourA
	if i%2 == 1 {
		colour = rowColourB
	}
	return e.Renderer.NewStyle().Foreground(colour).Render(line)
}
