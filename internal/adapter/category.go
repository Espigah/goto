package adapter

import (
	"fmt"

	"goto/internal/winfocus"
)

// category is an "umbrella" adapter for generic terms (e.g. "browser" covers
// chrome + firefox). Disambiguation rule:
//   - only 1 member open -> act on it (delegate Resolve, with the target)
//   - none open          -> error
//   - 2+ members open    -> do NOTHING (ambiguous; ask to be specific)
type category struct {
	names   []string
	members []Adapter
}

// Category creates a category adapter over the member adapters.
func Category(names []string, members ...Adapter) Adapter {
	return &category{names: names, members: members}
}

func (c *category) Names() []string { return c.names }

func (c *category) Match(w winfocus.Window) bool {
	for _, m := range c.members {
		if m.Match(w) {
			return true
		}
	}
	return false
}

func (c *category) Resolve(target []string, appWins []winfocus.Window, be winfocus.Backend) (winfocus.Window, bool, error) {
	var present []Adapter
	var windows [][]winfocus.Window
	for _, m := range c.members {
		var ws []winfocus.Window
		for _, w := range appWins {
			if m.Match(w) {
				ws = append(ws, w)
			}
		}
		if len(ws) > 0 {
			present = append(present, m)
			windows = append(windows, ws)
		}
	}

	switch len(present) {
	case 0:
		return winfocus.Window{}, false, fmt.Errorf("no %s open", c.names[0])
	case 1:
		return present[0].Resolve(target, windows[0], be)
	default:
		return winfocus.Window{}, false, fmt.Errorf("%d %ss open, be more specific", len(present), c.names[0])
	}
}
