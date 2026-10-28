package engine

import (
	"embed"
	"os"
	"path/filepath"
)

// Runner scripts that let TouchUp drive Python libraries that come
// without a command line of their own.
//
//go:embed scripts/*.py
var scriptsFS embed.FS

// installScripts copies the engine's runner scripts into its folder.
func installScripts(m *Manifest) error {
	for _, name := range m.Install.Scripts {
		data, err := scriptsFS.ReadFile("scripts/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(m.Dir(), name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
