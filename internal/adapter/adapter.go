// Package adapter isolates the logic for each target "program".
//
// PHILOSOPHY (built to be community-maintained):
// adding support for a new program = adding ONE file with an Adapter and
// registering it in an init(). The core (dispatch) does not change.
//
// There are two ways to implement an adapter:
//  1. Simple(...)  -> declarative, for apps that only need to match by
//     WM_CLASS and pick the window by title. One line. See builtin.go.
//  2. Full Adapter -> when the app needs a custom action (open a tab, run
//     the app's own command, etc). See vscode.go.
//
// Registration follows the idiomatic database/sql pattern: each adapter
// registers itself in init(), and dispatch only queries the registry.
package adapter

import (
	"goto/internal/textutil"
	"goto/internal/winfocus"
)

// Adapter encapsulates how to recognize and target a program.
type Adapter interface {
	// Names are the spoken terms that select this adapter
	// (e.g. "vscode", "code", "vs"). The command's 1st token matches here.
	Names() []string

	// Match reports whether a window belongs to this program.
	Match(w winfocus.Window) bool

	// Resolve picks the target among THIS program's windows (already
	// filtered by Match), given the target tokens (the rest of the command).
	//
	// Return (win, handled, err):
	//   handled=false -> the caller should activate `win` (default path)
	//   handled=true  -> the adapter already handled the focus/action (e.g. opened a file)
	Resolve(target []string, appWins []winfocus.Window, be winfocus.Backend) (win winfocus.Window, handled bool, err error)
}

var registry = map[string]Adapter{}

// Register registers an adapter under all of its Names. Call it in init().
func Register(a Adapter) {
	for _, n := range a.Names() {
		registry[textutil.Normalize(n)] = a
	}
}

// Lookup finds the adapter for a spoken app token.
func Lookup(appToken string) (Adapter, bool) {
	a, ok := registry[textutil.Normalize(appToken)]
	return a, ok
}

// All returns the registered adapters (for help/listing).
func All() map[string]Adapter { return registry }
