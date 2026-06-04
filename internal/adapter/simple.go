package adapter

import (
	"fmt"

	"goto/internal/winfocus"
)

// simple is the declarative adapter: it matches by WM_CLASS and picks the
// window by title. Covers most apps (Slack, Postman, browser, terminal...).
//
// To add an app like this, just one line in builtin.go:
//
//	Register(Simple([]string{"firefox"}, []string{"firefox"}))
type simple struct {
	names   []string
	classes []string
}

// Simple creates a declarative adapter. names = spoken terms;
// classes = WM_CLASS(es) that identify the app.
func Simple(names, classes []string) Adapter {
	return &simple{names: names, classes: classes}
}

func (s *simple) Names() []string { return s.names }

func (s *simple) Match(w winfocus.Window) bool {
	return classContainsAny(w.Class, s.classes)
}

func (s *simple) Resolve(target []string, appWins []winfocus.Window, _ winfocus.Backend) (winfocus.Window, bool, error) {
	if w, ok := pickByTitle(appWins, target); ok {
		return w, false, nil
	}
	return winfocus.Window{}, false, fmt.Errorf("%v: no window matched %v", s.names, target)
}
