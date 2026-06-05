//go:build linux

package winfocus

// New returns the platform backend for Linux (X11). main.go calls this
// OS-neutral constructor; the concrete X11 type stays in x11.go.
func New() (Backend, error) { return NewX11() }
