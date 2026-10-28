package engine

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
	return p
}

func TestExtractStripsSingleRoot(t *testing.T) {
	zipFile := writeZip(t, map[string]string{
		"tool-1.0/tool":         "binary",
		"tool-1.0/models/a.bin": "weights",
	})
	dir := t.TempDir()
	if err := extract(zipFile, dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"tool", "models/a.bin"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("%s missing after extract: %v", p, err)
		}
	}
}

func TestExtractKeepsFlatArchives(t *testing.T) {
	zipFile := writeZip(t, map[string]string{"tool": "binary", "models/a.bin": "weights"})
	dir := t.TempDir()
	if err := extract(zipFile, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "models", "a.bin")); err != nil {
		t.Error(err)
	}
}

func TestExtractRejectsEscapes(t *testing.T) {
	zipFile := writeZip(t, map[string]string{"a/../../evil": "x", "b/ok": "y"})
	dir := t.TempDir()
	extract(zipFile, dir)
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil")); err == nil {
		t.Fatal("an entry was written outside the install folder")
	}
}
