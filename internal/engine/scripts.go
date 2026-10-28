package engine

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
)

// Runner scripts that let TouchUp drive Python libraries that come
// without a command line of their own.
//
//go:embed scripts/*.py
var scriptsFS embed.FS

// installScripts copies the engine's runner scripts into its folder,
// rewriting only those that differ. Runs call it too, so an engine
// installed by an older TouchUp gets the scripts that ship with this one.
func installScripts(m *Manifest) error {
	for _, name := range m.Install.Scripts {
		data, err := scriptsFS.ReadFile("scripts/" + name)
		if err != nil {
			return err
		}
		target := filepath.Join(m.Dir(), name)
		if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, data) {
			continue
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
