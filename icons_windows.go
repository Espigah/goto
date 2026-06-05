//go:build windows

package main

import _ "embed"

// On Windows the system tray (Shell_NotifyIcon via fyne/systray) needs the icon
// in ICO format, not PNG.

//go:embed packaging/icons/goto.ico
var iconNormal []byte

//go:embed packaging/icons/goto-icon-listen.ico
var iconProcessing []byte
