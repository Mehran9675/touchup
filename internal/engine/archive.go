package engine

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// entry is one file of an archive.
type entry struct {
	name string
	mode os.FileMode
	dir  bool
	open func() (io.ReadCloser, error)
}

// extract unpacks a .zip or .tar.gz into dir. When everything sits in a
// single top-level folder, that folder is stripped, so engines land in
// dir itself whichever way their archive was packed.
func extract(file, dir string) error {
	var entries []entry
	var closer io.Closer
	if strings.HasSuffix(file, ".zip") {
		zr, err := zip.OpenReader(file)
		if err != nil {
			return err
		}
		closer = zr
		for _, f := range zr.File {
			entries = append(entries, entry{
				name: f.Name, mode: f.Mode(), dir: f.FileInfo().IsDir(),
				open: func() (io.ReadCloser, error) { return f.Open() },
			})
		}
	} else {
		return extractTar(file, dir)
	}
	defer closer.Close()
	strip := commonRoot(entries)
	for _, e := range entries {
		if err := writeEntry(dir, strip, e); err != nil {
			return err
		}
	}
	return nil
}

func extractTar(file, dir string) error {
	// Tar is sequential, so read the names first to find the root.
	names, err := tarNames(file)
	if err != nil {
		return err
	}
	var probe []entry
	for _, n := range names {
		probe = append(probe, entry{name: n})
	}
	strip := commonRoot(probe)
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			continue
		}
		e := entry{name: h.Name, mode: h.FileInfo().Mode(), dir: h.Typeflag == tar.TypeDir,
			open: func() (io.ReadCloser, error) { return io.NopCloser(tr), nil }}
		if err := writeEntry(dir, strip, e); err != nil {
			return err
		}
	}
}

func tarNames(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return names, nil
		}
		if err != nil {
			return nil, err
		}
		names = append(names, h.Name)
	}
}

// commonRoot returns "root/" when every entry is inside one folder.
func commonRoot(entries []entry) string {
	root := ""
	for _, e := range entries {
		name := strings.TrimPrefix(path.Clean("/"+e.name), "/")
		first, _, nested := strings.Cut(name, "/")
		if !nested && !e.dir {
			return ""
		}
		if root == "" {
			root = first
		} else if root != first {
			return ""
		}
	}
	if root == "" {
		return ""
	}
	return root + "/"
}

func writeEntry(dir, strip string, e entry) error {
	name := strings.TrimPrefix(path.Clean("/"+e.name), "/")
	name = strings.TrimPrefix(name, strip)
	if name == "" || name == strings.TrimSuffix(strip, "/") {
		return nil
	}
	target := filepath.Join(dir, filepath.FromSlash(name))
	if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
		return fmt.Errorf("archive entry %q escapes the install folder", e.name)
	}
	if e.dir {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := e.open()
	if err != nil {
		return err
	}
	defer src.Close()
	mode := e.mode.Perm() | 0o600
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
