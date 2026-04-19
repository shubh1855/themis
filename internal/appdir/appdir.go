// Package appdir provides cross-platform paths for Themis data and config.
package appdir

import (
	"os"
	"path/filepath"
	"runtime"
)

// Data returns the platform-appropriate user data directory.
//   - Linux:   $XDG_DATA_HOME/themis  or  ~/.local/share/themis
//   - Windows: %APPDATA%\themis
//   - macOS:   ~/Library/Application Support/themis
func Data() string {
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "themis")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "themis")
		}
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "themis")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "themis")
	}
	return filepath.Join(".", "themis")
}

// Config returns the platform-appropriate config directory.
func Config() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "themis")
	}
	return Data()
}
