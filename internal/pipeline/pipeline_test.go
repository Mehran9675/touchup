package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/settings"
)

// These tests run the real pipeline with stand-in engines: shell scripts
// that scale with FFmpeg. They need FFmpeg and a POSIX shell.

func setup(t *testing.T) media.Tools {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stand-in engines are shell scripts")
	}
	tools := media.Locate("", "", "")
	if !tools.Ready() {
		t.Skip("FFmpeg is not installed")
	}
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	return tools
}

// script writes an executable shell script and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "engine.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nset -e\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func ffmpeg(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-loglevel", "error", "-y"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
}

func options(t *testing.T, tools media.Tools, m *engine.Manifest) Options {
	t.Helper()
	m.Install = engine.InstallSpec{Type: "none"}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	return Options{Engine: m, Scale: 2, Settings: settings.Default(), Tools: tools}
}

func noReport(Update) {}

func TestImage(t *testing.T) {
	tools := setup(t)
	exe := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" "$2"`+"\n")
	o := options(t, tools, &engine.Manifest{
		ID: "scale", Name: "Scale", Media: []string{"image", "video"}, Input: "image",
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	in := filepath.Join(t.TempDir(), "in.png")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48", "-frames:v", "1", in)
	out := filepath.Join(t.TempDir(), "out.jpg")
	if err := Image(context.Background(), o, in, out, noReport); err != nil {
		t.Fatal(err)
	}
	info, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 128 || info.Height != 96 {
		t.Fatalf("got %dx%d, want 128x96", info.Width, info.Height)
	}
}

func TestVideoBatch(t *testing.T) {
	tools := setup(t)
	// A batch engine: folders in, folders out, same file names.
	exe := script(t, `for f in "$1"/*.png; do
  ffmpeg -loglevel error -y -i "$f" -vf "scale=iw*$3:ih*$3" "$2/$(basename "$f")"
done
`)
	o := options(t, tools, &engine.Manifest{
		ID: "batch", Name: "Batch", Media: []string{"image", "video"}, Input: "image", Batch: true,
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	in := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=10", "-f", "lavfi", "-i", "sine", "-t", "1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", in)
	info, err := tools.Probe(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := Video(context.Background(), o, in, out, info, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 128 || got.Height != 96 {
		t.Errorf("got %dx%d, want 128x96", got.Width, got.Height)
	}
	if !got.HasAudio {
		t.Error("the audio was dropped")
	}
	if got.Frames != info.Frames {
		t.Errorf("got %d frames, want %d", got.Frames, info.Frames)
	}
}

func TestPreviews(t *testing.T) {
	tools := setup(t)
	img := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" "$2"`+"\n")
	o := options(t, tools, &engine.Manifest{
		ID: "scale", Name: "Scale", Media: []string{"image", "video"}, Input: "image",
		Executable: map[string]string{"*": img}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	video := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=10", "-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)
	p, err := PreviewFrame(context.Background(), o, video, 1.2, noReport)
	if err != nil {
		t.Fatal(err)
	}
	if p.Before.Bounds().Dx() != 64 || p.After.Bounds().Dx() != 128 {
		t.Errorf("frame preview sizes %v and %v", p.Before.Bounds(), p.After.Bounds())
	}

	// An audio engine that writes its own file name into {outputDir}.
	snd := script(t, `ffmpeg -loglevel error -y -i "$1" -ar 48000 "$2/enhanced_$(basename "$1")"`+"\n")
	ao := options(t, tools, &engine.Manifest{
		ID: "rate", Name: "Rate", Media: []string{"audio"}, Input: "audio",
		Executable: map[string]string{"*": snd}, Args: []string{"{input}", "{outputDir}"}, OutputExt: ".wav",
	})
	audio := filepath.Join(t.TempDir(), "in.wav")
	ffmpeg(t, "-f", "lavfi", "-i", "sine=sample_rate=22050", "-t", "6", audio)
	ap, err := PreviewAudio(context.Background(), ao, audio, 1, 3, noReport)
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Discard()
	if ap.BeforeRate != 22050 || ap.AfterRate != 48000 {
		t.Errorf("rates %d and %d, want 22050 and 48000", ap.BeforeRate, ap.AfterRate)
	}
	if ap.BeforeSpec == nil || ap.AfterSpec == nil {
		t.Error("spectrograms missing")
	}
}

func TestOutputPathAvoidsOverwriting(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "song.flac")
	first := OutputPath(in, settings.Default(), 1, ".wav", nil)
	if filepath.Base(first) != "song (TouchUp).wav" {
		t.Fatalf("got %q", first)
	}
	os.WriteFile(first, nil, 0o644)
	if second := OutputPath(in, settings.Default(), 1, ".wav", nil); filepath.Base(second) != "song (TouchUp 2).wav" {
		t.Fatalf("got %q", second)
	}
	taken := func(p string) bool { return filepath.Base(p) == "song (TouchUp 2).wav" }
	if third := OutputPath(in, settings.Default(), 1, ".wav", taken); filepath.Base(third) != "song (TouchUp 3).wav" {
		t.Fatalf("got %q", third)
	}
}
