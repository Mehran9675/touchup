package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTarGz(t *testing.T, add func(tw *tar.Writer)) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	add(tw)
	tw.Close()
	gz.Close()
	f.Close()
	return p
}

func TestExtractTarLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	file := func(tw *tar.Writer, name, body string) {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	archive := writeTarGz(t, func(tw *tar.Writer) {
		file(tw, "python/bin/python3.12", "interpreter")
		tw.WriteHeader(&tar.Header{Name: "python/bin/python3", Linkname: "python3.12", Typeflag: tar.TypeSymlink})
		tw.WriteHeader(&tar.Header{Name: "python/lib/hard", Linkname: "python/bin/python3.12", Typeflag: tar.TypeLink})
	})
	dir := t.TempDir()
	if err := extract(archive, dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"bin/python3", "lib/hard"} {
		data, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil || string(data) != "interpreter" {
			t.Errorf("%s: %q, %v", p, data, err)
		}
	}

	evil := writeTarGz(t, func(tw *tar.Writer) {
		file(tw, "x/ok", "y")
		tw.WriteHeader(&tar.Header{Name: "x/escape", Linkname: "../../../etc/passwd", Typeflag: tar.TypeSymlink})
	})
	if err := extract(evil, t.TempDir()); err == nil {
		t.Fatal("a link pointing outside the folder was accepted")
	}
}

// TestManagedPython downloads TouchUp's Python and makes an engine
// environment with it. It needs the network: TOUCHUP_NETWORK_TESTS=1.
func TestManagedPython(t *testing.T) {
	if os.Getenv("TOUCHUP_NETWORK_TESTS") == "" {
		t.Skip("set TOUCHUP_NETWORK_TESTS=1 to download Python")
	}
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	py, err := EnsurePython(context.Background(), "", func(Status) {})
	if err != nil {
		t.Fatal(err)
	}
	venv := filepath.Join(t.TempDir(), "venv")
	if out, err := exec.Command(py, "-m", "venv", venv).CombinedOutput(); err != nil {
		t.Fatalf("venv: %v: %s", err, out)
	}
	vpy := filepath.Join(venv, "bin", "python")
	if runtime.GOOS == "windows" {
		vpy = filepath.Join(venv, "Scripts", "python.exe")
	}
	out, err := exec.Command(vpy, "-m", "pip", "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("pip: %v: %s", err, out)
	}
	t.Logf("%s", out)
	// A second call reuses the download.
	if again, err := EnsurePython(context.Background(), "", func(Status) { t.Error("downloaded again") }); err != nil || again != py {
		t.Fatalf("second call: %q, %v", again, err)
	}
}

func TestCheckPythonRejectsStoreShortcut(t *testing.T) {
	err := checkPython(context.Background(), `C:\Users\me\AppData\Local\Microsoft\WindowsApps\python.exe`)
	if err == nil {
		t.Fatal("the Store shortcut was accepted")
	}
}
