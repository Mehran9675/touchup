// Package paths locates the folders TouchUp keeps its data in.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

const appDir = "TouchUp"

// Data is the folder for settings, installed engines and custom engine
// manifests: ~/.local/share/TouchUp on Linux, ~/Library/Application
// Support/TouchUp on macOS and %LOCALAPPDATA%\TouchUp on Windows.
func Data() string {
	if dir := os.Getenv("TOUCHUP_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, appDir)
		}
		return filepath.Join(home, "AppData", "Local", appDir)
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", appDir)
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, appDir)
		}
		return filepath.Join(home, ".local", "share", appDir)
	}
}

// Engines is where engines are installed, one folder per engine ID.
func Engines() string { return filepath.Join(Data(), "engines") }

// Engine is the install folder of one engine.
func Engine(id string) string { return filepath.Join(Engines(), id) }

// CustomManifests holds the manifests of engines the user added.
func CustomManifests() string { return filepath.Join(Data(), "custom-engines") }

// Settings is the settings file.
func Settings() string { return filepath.Join(Data(), "settings.json") }

// Work is the scratch folder for frames, clips and previews. It is
// emptied when the app starts.
func Work() string { return filepath.Join(os.TempDir(), "touchup-work") }
