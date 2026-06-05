//go:build windows

// Windows backend for keyinject, using SendInput (user32) via syscalls, no
// cgo. Equivalent of x11.go's XTEST path: synthesize modifier chords, type
// text and press Enter into whatever window currently has focus.
//
// Behind the `windows` build tag, so the Linux build is untouched.
package keyinject

import (
	"time"
	"unicode"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32        = windows.NewLazySystemDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

const (
	inputKeyboard    = 1
	keyeventfKeyup   = 0x0002
	keyeventfUnicode = 0x0004

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12 // Alt
	vkReturn  = 0x0D
)

// matching the Win32 INPUT/KEYBDINPUT layout on amd64 (40 bytes total).
type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type input struct {
	typ uint32
	_   [4]byte
	ki  keybdInput
	_   [8]byte // pad the union up to MOUSEINPUT's size
}

// Injector synthesizes keyboard input. On Windows it holds no OS resources.
type Injector struct{}

// New returns an injector (no setup needed on Windows).
func New() (*Injector, error) { return &Injector{}, nil }

// Close is a no-op on Windows.
func (inj *Injector) Close() {}

func keyEvent(vk, scan uint16, flags uint32) input {
	return input{typ: inputKeyboard, ki: keybdInput{wVk: vk, wScan: scan, dwFlags: flags}}
}

func (inj *Injector) send(ev []input) {
	if len(ev) == 0 {
		return
	}
	procSendInput.Call(uintptr(len(ev)), uintptr(unsafe.Pointer(&ev[0])), unsafe.Sizeof(ev[0]))
}

// Chord presses the requested modifiers + the `key` (a letter), then releases
// them in reverse, e.g. Chord(true,false,false,'p') == Ctrl+P.
func (inj *Injector) Chord(ctrl, shift, alt bool, key rune) {
	var ev []input
	if ctrl {
		ev = append(ev, keyEvent(vkControl, 0, 0))
	}
	if shift {
		ev = append(ev, keyEvent(vkShift, 0, 0))
	}
	if alt {
		ev = append(ev, keyEvent(vkMenu, 0, 0))
	}
	vk := uint16(unicode.ToUpper(key))
	ev = append(ev, keyEvent(vk, 0, 0), keyEvent(vk, 0, keyeventfKeyup))
	if alt {
		ev = append(ev, keyEvent(vkMenu, 0, keyeventfKeyup))
	}
	if shift {
		ev = append(ev, keyEvent(vkShift, 0, keyeventfKeyup))
	}
	if ctrl {
		ev = append(ev, keyEvent(vkControl, 0, keyeventfKeyup))
	}
	inj.send(ev)
}

// typeRune sends one character as a layout-independent Unicode keystroke.
func (inj *Injector) typeRune(r rune) {
	inj.send([]input{
		keyEvent(0, uint16(r), keyeventfUnicode),
		keyEvent(0, uint16(r), keyeventfUnicode|keyeventfKeyup),
	})
}

// Type types a string (letters lowercased, digits and space; the rest is
// ignored), matching the X11 backend's behavior.
func (inj *Injector) Type(s string) {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			inj.typeRune(r - 'A' + 'a')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			inj.typeRune(r)
		case r == ' ':
			inj.typeRune(' ')
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// Enter presses Return.
func (inj *Injector) Enter() {
	inj.send([]input{keyEvent(vkReturn, 0, 0), keyEvent(vkReturn, 0, keyeventfKeyup)})
}
