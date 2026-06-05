//go:build windows

package autostart

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
)

const (
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName = "goto"
)

// Enabled reports whether the "goto" value exists in the Run key.
func Enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(appName)
	return err == nil
}

// Enable adds the executable path to the Registry's Run key with --paused.
func Enable(execPath string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	// Use quotes around the path in case it contains spaces.
	val := fmt.Sprintf(`"%s" --paused`, execPath)
	return k.SetStringValue(appName, val)
}

// Disable removes the "goto" value from the Registry's Run key.
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	err = k.DeleteValue(appName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete registry value: %w", err)
	}
	return nil
}
