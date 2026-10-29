package engine

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/paths"
)

// pythonHooks installs TouchUp's sitecustomize.py, which Python runs at
// start-up, and returns the environment that puts it on PYTHONPATH and
// turns its GPU memory limit on or off.
func pythonHooks(strictGPUMemory bool) ([]string, error) {
	data, err := scriptsFS.ReadFile("scripts/sitecustomize.py")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(paths.Data(), "python-hooks")
	target := filepath.Join(dir, "sitecustomize.py")
	if current, err := os.ReadFile(target); err != nil || !bytes.Equal(current, data) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return nil, err
		}
	}
	pythonPath := dir
	if existing := os.Getenv("PYTHONPATH"); existing != "" {
		pythonPath += string(os.PathListSeparator) + existing
	}
	strict := "0"
	if strictGPUMemory {
		strict = "1"
	}
	return []string{"PYTHONPATH=" + pythonPath, "TOUCHUP_STRICT_GPU_MEMORY=" + strict}, nil
}
