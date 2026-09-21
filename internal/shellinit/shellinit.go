// Package shellinit serves the shell functions that make `dl <ref>` actually
// move the user's shell. The scripts are embedded so the binary stays a single
// file with nothing to install alongside it.
package shellinit

import (
	"embed"
	"fmt"
)

//go:embed zsh.sh bash.sh fish.fish
var files embed.FS

// scriptFiles maps a shell name to its embedded file.
var scriptFiles = map[string]string{
	"zsh":  "zsh.sh",
	"bash": "bash.sh",
	"fish": "fish.fish",
}

// Script returns the integration snippet for a shell.
func Script(shell string) (string, error) {
	name, ok := scriptFiles[shell]
	if !ok {
		return "", fmt.Errorf("unknown shell %q; supported: bash, fish, zsh", shell)
	}
	raw, err := files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read the embedded %s script: %w", shell, err)
	}
	return string(raw), nil
}
