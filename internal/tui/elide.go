package tui

import "strings"

// ellipsis marks where components were dropped.
const ellipsis = "…"

// elidePath fits a path into max columns by dropping leading components
// rather than cutting the tail.
//
// The tail is what identifies a project: given
// ~/Tocuments/wbiz/coloriage-app/code4-reconciliation/coloriage/svg,
// "~/…/coloriage/svg" tells you what it is and "~/Tocuments/wbiz/coloriage-"
// does not. Components are dropped from the front until what remains fits,
// and only when a single component still will not fit is the tail cut.
func elidePath(p string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(p)
	if len(runes) <= max {
		return p
	}

	// Split off the root ("~", "/" or ""), then the components.
	root := ""
	rest := p
	switch {
	case strings.HasPrefix(p, "~/"):
		root, rest = "~", p[2:]
	case strings.HasPrefix(p, "/"):
		root, rest = "", p[1:]
	}
	parts := strings.Split(rest, "/")

	// Try keeping ever fewer trailing components.
	for keep := len(parts) - 1; keep >= 1; keep-- {
		candidate := root + "/" + ellipsis + "/" + strings.Join(parts[len(parts)-keep:], "/")
		if len([]rune(candidate)) <= max {
			return candidate
		}
	}

	// Even one component is too long: keep the prefix and cut the tail, which
	// at least shows the ellipsis so the truncation is visible.
	prefix := root + "/" + ellipsis + "/"
	if len([]rune(prefix)) >= max {
		return string([]rune(root + "/" + ellipsis)[:max])
	}
	last := []rune(parts[len(parts)-1])
	room := max - len([]rune(prefix))
	if room > len(last) {
		room = len(last)
	}
	return prefix + string(last[:room])
}
