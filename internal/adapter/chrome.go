package adapter

import (
	"fmt"
	"strings"
	"time"

	"goto/internal/keyinject"
	"goto/internal/winfocus"
)

// chrome is a rich adapter: besides focusing the window, it switches TABS
// using Chrome's native tab search (Ctrl+Shift+A, type the term, Enter).
// So "goto chrome github" jumps to the GitHub tab, not just the window.
type chrome struct{}

func (chrome) Names() []string { return []string{"chrome"} }

func (chrome) Match(w winfocus.Window) bool {
	return classContainsAny(w.Class, []string{"google-chrome", "chrome", "chromium"})
}

func (chrome) Resolve(target []string, appWins []winfocus.Window, be winfocus.Backend) (winfocus.Window, bool, error) {
	if len(appWins) == 0 {
		return winfocus.Window{}, false, fmt.Errorf("Chrome is not running")
	}
	// If the target is already the ACTIVE tab of some window, the title
	// matches: just focus it.
	if w, ok := pickByTitle(appWins, target); ok {
		return w, false, nil
	}
	// No target: focus the first Chrome window.
	if len(target) == 0 {
		return appWins[0], false, nil
	}

	// Target is in a non-active tab: focus the window and use tab search.
	if err := be.Activate(appWins[0]); err != nil {
		return winfocus.Window{}, false, err
	}
	inj, err := keyinject.New()
	if err != nil {
		// without keyboard injection, at least leave the window focused
		return winfocus.Window{}, true, nil
	}
	defer inj.Close()

	time.Sleep(250 * time.Millisecond) // let the focus settle
	inj.Chord(true, true, false, 'a')  // Ctrl+Shift+A -> tab search
	time.Sleep(220 * time.Millisecond)
	inj.Type(strings.Join(target, " "))
	time.Sleep(260 * time.Millisecond) // let Chrome filter
	inj.Enter()
	return winfocus.Window{}, true, nil
}
