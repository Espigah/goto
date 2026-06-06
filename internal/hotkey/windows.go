//go:build windows

// Windows backend for hotkey: a global low-level keyboard hook
// (WH_KEYBOARD_LL) driving push-to-talk. Equivalent of x11.go's XGrabKey loop:
// OnPress when the combo goes down, OnRelease when the key comes up, with
// auto-repeat filtered out.
//
// RegisterHotKey can't do this (it never reports key-up and has no hold
// semantics), so we use a hook + message loop, which is the standard Win32
// way to observe hold-to-talk. Behind the `windows` build tag, so the Linux
// build is untouched.
package hotkey

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

const (
	whKeyboardLL = 13
	wmKeydown    = 0x0100
	wmKeyup      = 0x0101
	wmSyskeydown = 0x0104
	wmSyskeyup   = 0x0105
	wmQuit       = 0x0012

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12 // Alt
	vkLWin    = 0x5B
)

type kbdllhookstruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type win32msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// Hotkey observes a global key combination for push-to-talk.
type Hotkey struct {
	OnPress   func()
	OnRelease func()

	vk       uint16   // the main key's virtual-key code
	mods     []uint16 // modifier VKs that must be held
	pressed  bool
	hook     uintptr
	threadID uintptr
}

var modVK = map[string]uint16{
	"ctrl": vkControl, "control": vkControl,
	"alt": vkMenu, "shift": vkShift,
	"super": vkLWin, "win": vkLWin, "cmd": vkLWin,
}

// keyVK resolves a key name to its virtual-key code.
func keyVK(name string) (uint16, bool) {
	switch name {
	case "space":
		return 0x20, true
	case "return", "enter":
		return 0x0D, true
	case "tab":
		return 0x09, true
	case "escape", "esc":
		return 0x1B, true
	}
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return uint16(name[0] - 'a' + 'A'), true // VK for a letter is its uppercase ASCII
	}
	if len(name) >= 2 && name[0] == 'f' {
		var n int
		if _, err := fmt.Sscanf(name[1:], "%d", &n); err == nil && n >= 1 && n <= 12 {
			return uint16(0x70 + n - 1), true // VK_F1..VK_F12
		}
	}
	return 0, false
}

// New parses the combo (e.g. "ctrl+alt+space") into a main key + modifiers.
func New(combo string, onPress, onRelease func()) (*Hotkey, error) {
	h := &Hotkey{OnPress: onPress, OnRelease: onRelease}
	var keyName string
	for _, part := range strings.Split(strings.ToLower(combo), "+") {
		part = strings.TrimSpace(part)
		if m, ok := modVK[part]; ok {
			h.mods = append(h.mods, m)
		} else {
			keyName = part
		}
	}
	vk, ok := keyVK(keyName)
	if !ok {
		return nil, fmt.Errorf("unknown key in combo %q: %q", combo, keyName)
	}
	h.vk = vk
	return h, nil
}

func (h *Hotkey) modsHeld() bool {
	for _, m := range h.mods {
		if r, _, _ := procGetAsyncKeyState.Call(uintptr(m)); r&0x8000 == 0 {
			return false
		}
	}
	return true
}

// only one hotkey is active at a time (main.go creates a single one); the hook
// callback reads it.
var active *Hotkey

var hookCB = windows.NewCallback(func(nCode, wParam, lParam uintptr) uintptr {
	if int32(nCode) >= 0 && active != nil {
		k := (*kbdllhookstruct)(unsafe.Pointer(lParam))
		if uint16(k.vkCode) == active.vk {
			switch wParam {
			case wmKeydown, wmSyskeydown:
				if active.modsHeld() {
					if !active.pressed {
						active.pressed = true
						if active.OnPress != nil {
							active.OnPress()
						}
					}
					return 1 // swallow, so the key doesn't reach other apps
				}
			case wmKeyup, wmSyskeyup:
				if active.pressed {
					active.pressed = false
					if active.OnRelease != nil {
						active.OnRelease()
					}
					return 1
				}
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, nCode, wParam, lParam)
	return r
})

// Start installs the hook on a dedicated, message-pumping OS thread (required
// for low-level hooks to fire).
func (h *Hotkey) Start() error {
	active = h
	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		h.threadID, _, _ = procGetCurrentThreadId.Call()
		hMod, _, _ := procGetModuleHandleW.Call(0)
		hook, _, err := procSetWindowsHookExW.Call(whKeyboardLL, hookCB, hMod, 0)
		if hook == 0 {
			errc <- fmt.Errorf("SetWindowsHookEx: %v", err)
			return
		}
		h.hook = hook
		errc <- nil

		var m win32msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
				break
			}
		}
		procUnhookWindowsHookEx.Call(h.hook)
	}()
	return <-errc
}

// Close ends the message loop (which unhooks) and clears the active hotkey.
func (h *Hotkey) Close() {
	if h.threadID != 0 {
		procPostThreadMessageW.Call(h.threadID, wmQuit, 0, 0)
	}
	active = nil
}
