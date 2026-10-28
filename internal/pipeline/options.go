// Package pipeline upscales whole files and builds previews, by running
// engines on files prepared with FFmpeg: images directly, videos frame
// by frame (or whole, for video engines), audio through WAV.
package pipeline

import (
	"errors"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/settings"
)

// Options select the engine and how it runs.
type Options struct {
	Engine   *engine.Manifest
	Model    *engine.Model
	Scale    int
	Settings settings.Settings
	Tools    media.Tools
}

// Update reports a job's progress.
type Update struct {
	Stage string  // what's happening now
	Frac  float64 // overall progress in [0, 1], or -1 when unknown
	Line  string  // the engine's latest output line, if any
}

// Reporter receives updates from a background goroutine.
type Reporter func(Update)

func (o Options) check() error {
	if o.Engine == nil {
		return errors.New("choose an engine first")
	}
	if !o.Engine.Installed() {
		return errors.New(o.Engine.Name + " is not installed")
	}
	if !o.Tools.Ready() {
		return media.ErrNoFFmpeg
	}
	return nil
}

func (o Options) job(in, out, inDir, outDir string, report Reporter, stage string, from, to float64) engine.Job {
	return engine.Job{
		Model: o.Model, Scale: o.Scale, GPU: o.Settings.GPU,
		Input: in, Output: out, InputDir: inDir, OutputDir: outDir,
		Progress: func(f float64) { report(Update{Stage: stage, Frac: from + (to-from)*f}) },
		Log:      func(line string) { report(Update{Stage: stage, Frac: -1, Line: line}) },
	}
}
