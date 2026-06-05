//go:build linux

// Package keyinject synthesizes keyboard input via XTEST (X11) to automate
// other apps' UIs. Used by the Chrome adapter to open the tab search
// (Ctrl+Shift+A), type the term and press Enter.
package keyinject

import (
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgb/xtest"
)

// keysyms used internally (X11 keysym codes).
const (
	ctrlL  xproto.Keysym = 0xffe3
	shiftL xproto.Keysym = 0xffe1
	altL   xproto.Keysym = 0xffe9
	retn   xproto.Keysym = 0xff0d
	space  xproto.Keysym = 0x0020
)

// Injector holds an X connection with the XTEST extension initialized.
type Injector struct {
	conn *xgb.Conn
	root xproto.Window
	min  xproto.Keycode
	per  int
	syms []xproto.Keysym
}

// New connects to X and prepares the keyboard map.
func New() (*Injector, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}
	if err := xtest.Init(conn); err != nil {
		conn.Close()
		return nil, err
	}
	setup := xproto.Setup(conn)
	root := setup.DefaultScreen(conn).Root
	min, max := setup.MinKeycode, setup.MaxKeycode
	km, err := xproto.GetKeyboardMapping(conn, min, byte(max-min+1)).Reply()
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &Injector{conn: conn, root: root, min: min, per: int(km.KeysymsPerKeycode), syms: km.Keysyms}, nil
}

// Close closes the X connection.
func (inj *Injector) Close() { inj.conn.Close() }

// keycodeFor finds a keysym's keycode and whether it needs Shift.
func (inj *Injector) keycodeFor(ks xproto.Keysym) (kc xproto.Keycode, shift, ok bool) {
	for i := 0; i+inj.per <= len(inj.syms); i += inj.per {
		if inj.syms[i] == ks {
			return inj.min + xproto.Keycode(i/inj.per), false, true
		}
		if inj.per > 1 && inj.syms[i+1] == ks {
			return inj.min + xproto.Keycode(i/inj.per), true, true
		}
	}
	return 0, false, false
}

func (inj *Injector) press(kc xproto.Keycode) {
	xtest.FakeInput(inj.conn, xproto.KeyPress, byte(kc), 0, inj.root, 0, 0, 0)
}
func (inj *Injector) release(kc xproto.Keycode) {
	xtest.FakeInput(inj.conn, xproto.KeyRelease, byte(kc), 0, inj.root, 0, 0, 0)
}

// flush forces delivery (round-trip).
func (inj *Injector) flush() { _, _ = xproto.GetInputFocus(inj.conn).Reply() }

func (inj *Injector) tap(ks xproto.Keysym) {
	kc, shift, ok := inj.keycodeFor(ks)
	if !ok {
		return
	}
	var sc xproto.Keycode
	if shift {
		sc, _, _ = inj.keycodeFor(shiftL)
		inj.press(sc)
	}
	inj.press(kc)
	inj.release(kc)
	if shift {
		inj.release(sc)
	}
	inj.flush()
}

// Chord presses the requested modifiers + the `key` (lowercase rune).
func (inj *Injector) Chord(ctrl, shift, alt bool, key rune) {
	var mcs []xproto.Keycode
	pressMod := func(ks xproto.Keysym) {
		if kc, _, ok := inj.keycodeFor(ks); ok {
			mcs = append(mcs, kc)
			inj.press(kc)
		}
	}
	if ctrl {
		pressMod(ctrlL)
	}
	if shift {
		pressMod(shiftL)
	}
	if alt {
		pressMod(altL)
	}
	if kc, _, ok := inj.keycodeFor(xproto.Keysym(key)); ok {
		inj.press(kc)
		inj.release(kc)
	}
	for i := len(mcs) - 1; i >= 0; i-- {
		inj.release(mcs[i])
	}
	inj.flush()
}

// Type types a string (letters/digits/space, lowercase; the rest is ignored).
func (inj *Injector) Type(s string) {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			inj.tap(xproto.Keysym(r - 'A' + 'a'))
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			inj.tap(xproto.Keysym(r))
		case r == ' ':
			inj.tap(space)
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// Enter presses Return.
func (inj *Injector) Enter() { inj.tap(retn) }
