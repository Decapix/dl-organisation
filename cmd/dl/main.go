// Command dl bookmarks directories in numbered slots and keeps a free-form
// note on each one. See docs/superpowers/specs for the design.
package main

import "fmt"

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "dev"

func main() {
	fmt.Println("dl", version)
}
