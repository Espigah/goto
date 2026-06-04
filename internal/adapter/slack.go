package adapter

import (
	"fmt"
	"strings"
	"time"

	"goto/internal/keyinject"
	"goto/internal/winfocus"
)

// slack is a rich adapter: besides focusing the window, it opens a DM/channel
// using Slack's native "Jump to" (Ctrl+K, type the name, Enter).
// So "goto slack john doe" goes straight to the conversation with that person.
type slack struct{}

func (slack) Names() []string { return []string{"slack"} }

func (slack) Match(w winfocus.Window) bool {
	return classContainsAny(w.Class, []string{"slack"})
}

func (slack) Resolve(target []string, appWins []winfocus.Window, be winfocus.Backend) (winfocus.Window, bool, error) {
	if len(appWins) == 0 {
		return winfocus.Window{}, false, fmt.Errorf("Slack is not running")
	}
	// no target: just focus the Slack window.
	if len(target) == 0 {
		return appWins[0], false, nil
	}

	// with a target: focus and use "Jump to" (Ctrl+K) to go to the person/channel.
	if err := be.Activate(appWins[0]); err != nil {
		return winfocus.Window{}, false, err
	}
	inj, err := keyinject.New()
	if err != nil {
		return winfocus.Window{}, true, nil // at least focused the window
	}
	defer inj.Close()

	time.Sleep(250 * time.Millisecond) // let the focus settle
	inj.Chord(true, false, false, 'k') // Ctrl+K -> Jump to
	time.Sleep(250 * time.Millisecond)
	inj.Type(strings.Join(target, " "))
	time.Sleep(350 * time.Millisecond) // Slack filters (sometimes over the network)
	inj.Enter()
	return winfocus.Window{}, true, nil
}
