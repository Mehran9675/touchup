package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
)

func serve(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestModelInstallSharesFiles(t *testing.T) {
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	srv := serve(t, map[string]string{"/a": "weights a", "/b": "weights b", "/vae": "shared vae"})
	vae := ModelFile{URL: srv.URL + "/vae", Name: "vae.bin", SHA256: sum("shared vae")}
	m := &Manifest{ID: "test-models", Models: []Model{
		{ID: "a", Files: []ModelFile{{URL: srv.URL + "/a", Name: "a.bin", SHA256: sum("weights a")}, vae}},
		{ID: "b", Files: []ModelFile{{URL: srv.URL + "/b", Name: "sub/b.bin"}, vae}},
	}}
	a, b := &m.Models[0], &m.Models[1]
	noop := func(Status) {}
	for _, mdl := range []*Model{a, b} {
		if m.ModelInstalled(mdl) {
			t.Fatalf("%s installed before downloading", mdl.ID)
		}
		if err := InstallModel(context.Background(), m, mdl, noop); err != nil {
			t.Fatal(err)
		}
		if !m.ModelInstalled(mdl) {
			t.Fatalf("%s not installed after downloading", mdl.ID)
		}
	}
	// Removing a keeps the VAE b still needs.
	if err := UninstallModel(m, a); err != nil {
		t.Fatal(err)
	}
	if m.ModelInstalled(a) || !m.ModelInstalled(b) {
		t.Fatal("removing a model removed files another model uses")
	}
	if got := m.ModelPath(b); got != m.ModelsDir()+string(os.PathSeparator)+"sub"+string(os.PathSeparator)+"b.bin" {
		t.Errorf("ModelPath = %q", got)
	}
}

func TestModelInstallRejectsCorruptFiles(t *testing.T) {
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	srv := serve(t, map[string]string{"/a": "tampered"})
	m := &Manifest{ID: "test-corrupt", Models: []Model{
		{ID: "a", Files: []ModelFile{{URL: srv.URL + "/a", Name: "a.bin", SHA256: sum("weights a")}}},
	}}
	if err := InstallModel(context.Background(), m, &m.Models[0], func(Status) {}); err == nil {
		t.Fatal("a file with the wrong checksum was accepted")
	}
	if m.ModelInstalled(&m.Models[0]) {
		t.Fatal("a corrupt model counts as installed")
	}
}

func TestResolutionPlaceholders(t *testing.T) {
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	m := &Manifest{ID: "x", Args: []string{"--resolution", "{outShort}", "--size", "{outWidth}x{outHeight}", "--model", "{modelPath}"}}
	mdl := &Model{ID: "m", Files: []ModelFile{{URL: "u", Name: "w.safetensors"}}}
	got := m.Command(Job{Model: mdl, Scale: 2, GPU: -1, InWidth: 1920, InHeight: 1080})
	want := []string{"--resolution", "2160", "--size", "3840x2160", "--model", m.ModelPath(mdl)}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
