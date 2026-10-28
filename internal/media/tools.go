package media

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Tools are the FFmpeg programs TouchUp drives.
type Tools struct {
	FFmpeg  string
	FFprobe string
	FFplay  string
}

// ErrNoFFmpeg is returned when FFmpeg can't be found.
var ErrNoFFmpeg = errors.New("FFmpeg was not found: install it or set its path in Settings")

// Locate resolves each tool: the configured path when set, otherwise
// next to TouchUp's executable, otherwise on PATH. Tools not found are
// left empty.
func Locate(ffmpeg, ffprobe, ffplay string) Tools {
	return Tools{
		FFmpeg:  find(ffmpeg, "ffmpeg"),
		FFprobe: find(ffprobe, "ffprobe"),
		FFplay:  find(ffplay, "ffplay"),
	}
}

// Ready reports whether FFmpeg and FFprobe were found; playback is
// optional.
func (t Tools) Ready() bool { return t.FFmpeg != "" && t.FFprobe != "" }

func find(configured, name string) string {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
		return ""
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}
