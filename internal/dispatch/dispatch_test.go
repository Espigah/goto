package dispatch

import (
	"testing"

	"goto/internal/winfocus"
)

// fakeBE: fake backend for tests (does not touch X).
type fakeBE struct{ wins []winfocus.Window }

func (f fakeBE) List() ([]winfocus.Window, error) { return f.wins, nil }
func (f fakeBE) Activate(winfocus.Window) error   { return nil }

// synthetic windows: a few projects in VS Code, Slack, terminals and Chrome.
var sample = []winfocus.Window{
	{Handle: 1, Class: "Code", Title: "MYPROJECT"},
	{Handle: 2, Class: "Code", Title: "FRONTEND"},
	{Handle: 3, Class: "Code", Title: "BACKEND"},
	{Handle: 4, Class: "Slack", Title: "John Doe - Slack"},
	{Handle: 5, Class: "Terminator", Title: "✳ logs"},
	{Handle: 6, Class: "Terminator", Title: "✳ build"},
	{Handle: 7, Class: "Terminator", Title: "⠂ deploy server"},
	{Handle: 8, Class: "Google-chrome", Title: "WhatsApp - Google Chrome"},
}

func TestResolve(t *testing.T) {
	be := fakeBE{sample}
	cases := []struct {
		query string
		want  string // expected title
	}{
		{"vscode myproject", "MYPROJECT"},
		{"vscode backend", "BACKEND"},
		{"slack", "John Doe - Slack"},
		{"terminal deploy", "⠂ deploy server"}, // <- terminal, not the Slack app
		{"terminal logs", "✳ logs"},
		{"terminal build", "✳ build"},
		{"myproject", "MYPROJECT"},           // generic, no explicit app
		{"to vscode myproject", "MYPROJECT"}, // leading "to" from the ASR is dropped
		{"o slack", "John Doe - Slack"},
		{`"vest code".`, "MYPROJECT"},       // quotes/period become spaces; "code" matches the Code class
		{"vscode, myproject!", "MYPROJECT"}, // punctuation does not get in the way
	}
	for _, c := range cases {
		win, handled, err := Resolve(c.query, be)
		if err != nil {
			t.Errorf("%q: error %v", c.query, err)
			continue
		}
		if handled {
			t.Errorf("%q: unexpected handled", c.query)
			continue
		}
		if win.Title != c.want {
			t.Errorf("%q: got %q, expected %q", c.query, win.Title, c.want)
		}
	}
}

func TestTerminalDoesNotPickSlackApp(t *testing.T) {
	be := fakeBE{sample}
	win, _, err := Resolve("terminal deploy", be)
	if err != nil || win.Class == "Slack" {
		t.Fatalf("wrongly fell into the Slack app: %+v (err=%v)", win, err)
	}
}
