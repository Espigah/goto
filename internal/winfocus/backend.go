package winfocus

// Window describes a visible top-level window, in a platform-independent way.
// Handle is opaque and backend-specific (on X11 it's an xproto.Window; on
// Windows it would be an HWND, etc). Adapters must NOT inspect the Handle,
// only pass the whole Window back to Backend.Activate.
type Window struct {
	Title  string // _NET_WM_NAME (e.g. "myproject.go - ... - Visual Studio Code")
	Class  string // WM_CLASS  (e.g. "Code", "Slack", "Terminator")
	Handle any    // backend-internal identifier
}

// Backend is the platform abstraction for listing and focusing windows.
// To port goto to a new OS, implement this interface:
//   - Linux X11  -> x11.go (done)
//   - Wayland    -> depends on the compositor
//   - Windows    -> Win32 (EnumWindows / SetForegroundWindow)
//   - macOS      -> Accessibility / NSWorkspace
type Backend interface {
	// List returns the top-level windows managed by the system.
	List() ([]Window, error)
	// Activate brings the window to the front and gives it focus.
	Activate(w Window) error
}
