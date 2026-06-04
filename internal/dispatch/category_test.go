package dispatch

import (
	"testing"

	"goto/internal/winfocus"
)

func TestBrowserCategory(t *testing.T) {
	chromeOnly := fakeBE{[]winfocus.Window{
		{Handle: 1, Class: "Google-chrome", Title: "GitHub - Google Chrome"},
		{Handle: 2, Class: "Code", Title: "BACKEND"},
	}}
	firefoxOnly := fakeBE{[]winfocus.Window{
		{Handle: 1, Class: "firefox", Title: "Some page — Mozilla Firefox"},
	}}
	both := fakeBE{[]winfocus.Window{
		{Handle: 1, Class: "Google-chrome", Title: "GitHub - Google Chrome"},
		{Handle: 2, Class: "firefox", Title: "Some page — Mozilla Firefox"},
	}}

	// only chrome open -> "browser" focuses chrome
	if w, _, err := Resolve("browser", chromeOnly); err != nil || w.Class != "Google-chrome" {
		t.Errorf("only chrome: got %+v err=%v", w, err)
	}
	// only firefox open -> "browser" focuses firefox
	if w, _, err := Resolve("browser", firefoxOnly); err != nil || w.Class != "firefox" {
		t.Errorf("only firefox: got %+v err=%v", w, err)
	}
	// both open -> ambiguous, do nothing (error, no window)
	if w, _, err := Resolve("browser", both); err == nil {
		t.Errorf("ambiguous should error, but focused %+v", w)
	}
}
