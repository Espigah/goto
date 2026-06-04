package adapter

import (
	"fmt"
	"strings"

	"goto/internal/textutil"
	"goto/internal/winfocus"
)

// generic is the fallback when the user does not name a known app
// (e.g. "goto myproject"). It free-matches on class+title across all
// windows. It does not register Names; dispatch uses it explicitly.
type generic struct{}

// Generic returns the fallback adapter.
func Generic() Adapter { return generic{} }

func (generic) Names() []string { return nil }

func (generic) Match(winfocus.Window) bool { return true }

func (generic) Resolve(target []string, wins []winfocus.Window, _ winfocus.Backend) (winfocus.Window, bool, error) {
	if len(target) == 0 {
		if len(wins) > 0 {
			return wins[0], false, nil
		}
		return winfocus.Window{}, false, fmt.Errorf("no windows open")
	}
	best := 0
	var bw winfocus.Window
	for _, w := range wins {
		class := textutil.Normalize(w.Class)
		title := textutil.Normalize(w.Title)
		s := 0
		for _, t := range target {
			if class != "" && strings.Contains(class, t) {
				s += 2 // class identifies the app: worth more
			}
			if title != "" && strings.Contains(title, t) {
				s++
			}
		}
		if s > best {
			best, bw = s, w
		}
	}
	if best == 0 {
		return winfocus.Window{}, false, fmt.Errorf("nothing matched %v", target)
	}
	return bw, false, nil
}
