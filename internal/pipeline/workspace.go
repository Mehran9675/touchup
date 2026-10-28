package pipeline

import (
	"io"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/paths"
)

// workspace is a scratch folder for one job, with in/ and out/ folders
// for the engine.
type workspace struct {
	root, in, out string
	keep          bool
}

func newWorkspace(prefix string, keep bool) (*workspace, error) {
	if err := os.MkdirAll(paths.Work(), 0o755); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(paths.Work(), prefix+"-")
	if err != nil {
		return nil, err
	}
	w := &workspace{root: root, in: filepath.Join(root, "in"), out: filepath.Join(root, "out"), keep: keep}
	for _, d := range []string{w.in, w.out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return w, nil
}

func (w *workspace) path(name string) string { return filepath.Join(w.root, name) }

func (w *workspace) close() {
	if !w.keep {
		os.RemoveAll(w.root)
	}
}

// ClearWork deletes scratch folders left behind by earlier runs.
func ClearWork() { os.RemoveAll(paths.Work()) }

// copyInto copies the file at from to to.
func copyInto(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
