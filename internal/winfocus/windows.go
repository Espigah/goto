//go:build windows

// Windows backend for winfocus, using the Win32 API (user32) via syscalls,
// no cgo. It is the equivalent of x11.go: List() enumerates top-level windows
// (EnumWindows) and Activate() brings one to the front (SetForegroundWindow).
//
// Kept entirely behind the `windows` build tag so it never touches the Linux
// build.
package winfocus

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procGetWindow                = user32.NewProc("GetWindow")
	procIsIconic                 = user32.NewProc("IsIconic")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
)

const (
	gwlExStyle     = uintptr(0xFFFFFFEC) // GWL_EXSTYLE (-20) as a 32-bit word
	wsExToolWindow = 0x00000080          // WS_EX_TOOLWINDOW
	gwOwner        = 4                   // GW_OWNER
	swRestore      = 9                   // SW_RESTORE
	swShow         = 5                   // SW_SHOW
)

// win32 implements winfocus.Backend on Windows.
type win32 struct{}

func newWin32() (*win32, error) { return &win32{}, nil }

// EnumWindows is synchronous, so we guard a single shared destination with a
// mutex instead of creating a new syscall callback (which would leak) per call.
var (
	enumMu  sync.Mutex
	enumDst *[]Window
	enumCB  = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if enumDst != nil && isAppWindow(hwnd) {
			*enumDst = append(*enumDst, Window{
				Title:  windowText(hwnd),
				Class:  className(hwnd),
				Handle: hwnd,
			})
		}
		return 1 // keep enumerating
	})
)

func (win32) List() ([]Window, error) {
	enumMu.Lock()
	defer enumMu.Unlock()
	out := make([]Window, 0, 64)
	enumDst = &out
	procEnumWindows.Call(enumCB, 0)
	enumDst = nil
	// drop windows with no title (background/system windows slip through).
	filtered := out[:0]
	for _, w := range out {
		if w.Title != "" {
			filtered = append(filtered, w)
		}
	}
	return filtered, nil
}

func (win32) Activate(w Window) error {
	hwnd, ok := w.Handle.(uintptr)
	if !ok {
		return fmt.Errorf("invalid window handle for the Windows backend")
	}

	// 1. Restore if minimized
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
	} else {
		procShowWindow.Call(hwnd, swShow)
	}

	// 2. Bring to top
	procBringWindowToTop.Call(hwnd)

	// 3. Try SetForegroundWindow
	if r, _, _ := procSetForegroundWindow.Call(hwnd); r != 0 {
		return nil
	}

	// 4. Force focus "hack": attach to the current foreground thread
	// This is often needed when the calling process (goto) doesn't have focus.
	fgHwnd, _, _ := procGetForegroundWindow.Call()
	if fgHwnd == 0 || fgHwnd == hwnd {
		return nil
	}

	fgThread, _, _ := procGetWindowThreadProcessId.Call(fgHwnd, 0)
	myThread, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)

	if fgThread != myThread {
		procAttachThreadInput.Call(myThread, fgThread, 1)
		procSetForegroundWindow.Call(hwnd)
		procAttachThreadInput.Call(myThread, fgThread, 0)
	}

	return nil
}

// isAppWindow keeps visible, un-owned, non-tool top-level windows, the set a
// user would expect in Alt+Tab.
func isAppWindow(hwnd uintptr) bool {
	if r, _, _ := procIsWindowVisible.Call(hwnd); r == 0 {
		return false
	}
	if owner, _, _ := procGetWindow.Call(hwnd, gwOwner); owner != 0 {
		return false
	}
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	return ex&wsExToolWindow == 0
}

func windowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLength.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func className(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}
