// Package winfocus finds and activates (focuses) system windows.
//
// This file is the Linux/X11 backend, speaking EWMH directly via xgbutil
// (no dependency on wmctrl/xdotool being installed). It is the equivalent,
// in our app, of Handy's `input.rs`: instead of typing text, it acts on the
// window.
//
// The Backend interface (backend.go) isolates the platform; this is only X11.
package winfocus

import (
	"fmt"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
)

// X11 is the EWMH backend. Implements winfocus.Backend.
type X11 struct {
	x *xgbutil.XUtil
}

// NewX11 connects to the current display.
func NewX11() (*X11, error) {
	x, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connect to X: %w", err)
	}
	return &X11{x: x}, nil
}

// List returns all top-level windows managed by the WM.
func (b *X11) List() ([]Window, error) {
	ids, err := ewmh.ClientListGet(b.x)
	if err != nil {
		return nil, fmt.Errorf("_NET_CLIENT_LIST: %w", err)
	}
	out := make([]Window, 0, len(ids))
	for _, id := range ids {
		w := Window{Handle: id}
		// _NET_WM_NAME (utf8) is the "good" title; fall back to WM_NAME.
		if name, err := ewmh.WmNameGet(b.x, id); err == nil && name != "" {
			w.Title = name
		} else if name, err := icccm.WmNameGet(b.x, id); err == nil {
			w.Title = name
		}
		if cls, err := icccm.WmClassGet(b.x, id); err == nil && cls != nil {
			w.Class = cls.Class
		}
		out = append(out, w)
	}
	return out, nil
}

// Activate brings the window to the front and focuses it.
func (b *X11) Activate(w Window) error {
	id, ok := w.Handle.(xproto.Window)
	if !ok {
		return fmt.Errorf("invalid window handle for the X11 backend")
	}
	// _NET_ACTIVE_WINDOW is the correct way under EWMH (the WM handles
	// un-minimizing, switching workspace, raising and focusing).
	if err := ewmh.ActiveWindowReq(b.x, id); err != nil {
		return fmt.Errorf("activate window: %w", err)
	}
	return nil
}
