package adapter

import (
	"fmt"
	"strings"
	"time"

	"goto/internal/keyinject"
	"goto/internal/winfocus"
)

// vscode is a rich adapter. It resolves in layers:
//  1. target matches a window title (project, e.g. "backend") -> focus the window
//  2. no target -> focus the first VS Code window
//  3. target is a file -> focus and open via Ctrl+P (Quick Open), type, Enter
//
// Same pattern as chrome/slack: focus the app + native shortcut + type.
type vscode struct{}

func (vscode) Names() []string { return []string{"vscode", "code", "vs"} }

func (vscode) Match(w winfocus.Window) bool {
	return classContainsAny(w.Class, []string{"code"})
}

func (vscode) Resolve(target []string, appWins []winfocus.Window, be winfocus.Backend) (winfocus.Window, bool, error) {
	if len(appWins) == 0 {
		return winfocus.Window{}, false, fmt.Errorf("VS Code is not running")
	}
	// 1. target matches a PROJECT (window title)? focus the window.
	if w, ok := pickByTitle(appWins, target); ok {
		return w, false, nil
	}
	// 2. no target: focus the first VS Code window.
	if len(target) == 0 {
		return appWins[0], false, nil
	}
	// 3. target is a FILE: focus a window and open via Ctrl+P (Quick Open).
	if err := be.Activate(appWins[0]); err != nil {
		return winfocus.Window{}, false, err
	}
	inj, err := keyinject.New()
	if err != nil {
		return winfocus.Window{}, true, nil // at least focused the window
	}
	defer inj.Close()

	time.Sleep(250 * time.Millisecond) // let the focus settle
	inj.Chord(true, false, false, 'p') // Ctrl+P -> Quick Open
	time.Sleep(220 * time.Millisecond)
	inj.Type(strings.Join(target, " "))
	time.Sleep(300 * time.Millisecond) // Quick Open filters
	inj.Enter()
	return winfocus.Window{}, true, nil
}
