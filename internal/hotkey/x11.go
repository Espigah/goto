//go:build linux

// Package hotkey registers a global shortcut and drives push-to-talk.
//
// Pure X11 implementation (XGrabKey on the root window) via xgb, so the build
// does not require libX11-dev. Supports "hold": OnPress when the key goes
// down, OnRelease when it comes up, filtering out X auto-repeat.
package hotkey

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

// Hotkey listens for a global key combination.
type Hotkey struct {
	OnPress   func()
	OnRelease func()

	conn    *xgb.Conn
	root    xproto.Window
	keycode xproto.Keycode
	mods    uint16
	done    chan struct{}
}

var modNames = map[string]uint16{
	"ctrl": xproto.ModMaskControl, "control": xproto.ModMaskControl,
	"alt": xproto.ModMask1, "shift": xproto.ModMaskShift,
	"super": xproto.ModMask4, "win": xproto.ModMask4, "cmd": xproto.ModMask4,
}

var keysyms = map[string]uint32{
	"space": 0x0020, "return": 0xff0d, "enter": 0xff0d,
	"tab": 0xff09, "escape": 0xff1b, "esc": 0xff1b,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keysyms[string(c)] = uint32(c)
	}
	for n := 1; n <= 12; n++ {
		keysyms[fmt.Sprintf("f%d", n)] = uint32(0xffbe + n - 1)
	}
}

// New parses the combo (e.g. "ctrl+alt+space"), connects to X and resolves
// the keycode.
func New(combo string, onPress, onRelease func()) (*Hotkey, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connect to X: %w", err)
	}
	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	var mods uint16
	var keyName string
	for _, part := range strings.Split(strings.ToLower(combo), "+") {
		part = strings.TrimSpace(part)
		if m, ok := modNames[part]; ok {
			mods |= m
		} else {
			keyName = part
		}
	}
	ks, ok := keysyms[keyName]
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("unknown key in combo %q: %q", combo, keyName)
	}
	kc, err := keysymToKeycode(conn, ks)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &Hotkey{
		OnPress: onPress, OnRelease: onRelease,
		conn: conn, root: screen.Root, keycode: kc, mods: mods,
		done: make(chan struct{}),
	}, nil
}

func keysymToKeycode(conn *xgb.Conn, target uint32) (xproto.Keycode, error) {
	setup := xproto.Setup(conn)
	min, max := setup.MinKeycode, setup.MaxKeycode
	reply, err := xproto.GetKeyboardMapping(conn, min, byte(max-min+1)).Reply()
	if err != nil {
		return 0, fmt.Errorf("keyboard mapping: %w", err)
	}
	per := int(reply.KeysymsPerKeycode)
	for i, ks := range reply.Keysyms {
		if uint32(ks) == target {
			return min + xproto.Keycode(i/per), nil
		}
	}
	return 0, fmt.Errorf("keysym 0x%x not on the keyboard", target)
}

// lockCombos: variations with NumLock/CapsLock so the grab always applies.
var lockCombos = []uint16{0, xproto.ModMaskLock, xproto.ModMask2, xproto.ModMaskLock | xproto.ModMask2}

// Start performs the grab and starts the event loop.
func (h *Hotkey) Start() error {
	for _, extra := range lockCombos {
		xproto.GrabKey(h.conn, false, h.root, h.mods|extra, h.keycode,
			xproto.GrabModeAsync, xproto.GrabModeAsync)
	}
	go h.loop()
	return nil
}

func (h *Hotkey) loop() {
	pressed := false
	for {
		ev, err := h.conn.WaitForEvent()
		if err != nil {
			continue
		}
		if ev == nil { // connection closed
			return
		}
		switch e := ev.(type) {
		case xproto.KeyPressEvent:
			if e.Detail == h.keycode && !pressed {
				pressed = true
				if h.OnPress != nil {
					h.OnPress()
				}
			}
		case xproto.KeyReleaseEvent:
			if e.Detail != h.keycode {
				continue
			}
			// filter auto-repeat: X sends Release+Press with the same Time.
			if next, _ := h.conn.PollForEvent(); next != nil {
				if kp, ok := next.(xproto.KeyPressEvent); ok && kp.Detail == e.Detail && kp.Time == e.Time {
					continue // it's a repeat; keep it held
				}
			}
			if pressed {
				pressed = false
				if h.OnRelease != nil {
					h.OnRelease()
				}
			}
		}
	}
}

// Close undoes the grab and closes the connection.
func (h *Hotkey) Close() {
	for _, extra := range lockCombos {
		xproto.UngrabKey(h.conn, h.keycode, h.root, h.mods|extra)
	}
	h.conn.Close()
}
