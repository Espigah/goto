package adapter

import (
	"strings"

	"goto/internal/textutil"
	"goto/internal/winfocus"
)

// classContainsAny reports whether the window's WM_CLASS contains any of the
// classes (normalized, case-insensitive comparison).
func classContainsAny(winClass string, classes []string) bool {
	c := textutil.Normalize(winClass)
	for _, x := range classes {
		if strings.Contains(c, textutil.Normalize(x)) {
			return true
		}
	}
	return false
}

// pickByTitle picks the best window by title given the target tokens.
//   - empty target -> first window (e.g. "goto slack")
//   - no hit       -> ok=false
func pickByTitle(wins []winfocus.Window, target []string) (winfocus.Window, bool) {
	if len(wins) == 0 {
		return winfocus.Window{}, false
	}
	if len(target) == 0 {
		return wins[0], true
	}
	best := 0
	var bw winfocus.Window
	found := false
	for _, w := range wins {
		if s := textutil.ScoreTitle(w.Title, target); s > best {
			best, bw, found = s, w, true
		}
	}
	if !found {
		return winfocus.Window{}, false
	}
	return bw, true
}
