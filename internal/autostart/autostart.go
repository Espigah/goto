// Package autostart manages the XDG autostart entry that launches goto in the
// tray on login (~/.config/autostart/goto.desktop). The tray exposes a toggle
// so the user can turn "start at login" on or off.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// path returns ~/.config/autostart/goto.desktop.
func path() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "autostart", "goto.desktop")
}

// Enabled reports whether the autostart entry exists.
func Enabled() bool {
	_, err := os.Stat(path())
	return err == nil
}

// Enable writes the autostart entry. It launches the given binary with
// `--paused` so the mic does not turn on by itself at login.
func Enable(execPath string) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=goto
Comment=Voice window control
Exec=%s --paused
Icon=goto
Terminal=false
X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=3
`, execPath)
	return os.WriteFile(p, []byte(content), 0o644)
}

// Disable removes the autostart entry.
func Disable() error {
	err := os.Remove(path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
