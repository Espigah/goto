//go:build windows

package winfocus

// New returns the platform backend for Windows (Win32). Mirrors new_linux.go.
func New() (Backend, error) { return newWin32() }
