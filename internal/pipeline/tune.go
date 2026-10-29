package pipeline

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mehran9675/touchup/internal/engine"
)

// TuneTrial is one timed run while tuning.
type TuneTrial struct {
	Option, Value string        // what changed from the best so far; empty for the first run
	Took          time.Duration // the run, model loading included
	Err           error         // why it failed or was stopped
	Kept          bool          // it was faster and its settings were kept
}

// TuneResult is what tuning found: the fastest settings that worked,
// and every trial on the way there.
type TuneResult struct {
	Start, Best map[string]string
	Trials      []TuneTrial
	Frames      int
}

// errSlower stops a trial that has already taken longer than the best.
var errSlower = errors.New("stopped: already slower than the best")

// Tune finds the engine's fastest settings on this computer. It cuts a
// clip of the engine's Tune.Seconds from the video at path around at,
// upscales it with the current settings, then tries each tune step's
// values in turn, keeping each that's faster. A trial still running
// when it's well past the best time is stopped, since it can't win.
func Tune(ctx context.Context, o Options, path string, at float64, report Reporter) (TuneResult, error) {
	res := TuneResult{Start: maps.Clone(o.EngineOptions), Best: maps.Clone(o.EngineOptions)}
	if err := o.check(); err != nil {
		return res, err
	}
	spec := o.Engine.Tune
	if spec == nil {
		return res, fmt.Errorf("%s has nothing to tune", o.Engine.Name)
	}
	ws, err := newWorkspace("tune", o.Settings.KeepWork)
	if err != nil {
		return res, err
	}
	defer ws.close()

	report(Update{Stage: "Cutting a clip", Frac: 0.01})
	clip := filepath.Join(ws.in, "clip.mp4")
	if err := o.Tools.VideoClip(ctx, path, max(0, at-spec.Seconds/2), spec.Seconds, clip); err != nil {
		return res, err
	}
	info, err := o.Tools.Probe(ctx, clip)
	if err != nil {
		return res, err
	}
	res.Frames = info.Frames

	total := 1
	for _, st := range spec.Steps {
		total += len(st.Values)
	}
	n := 0
	run := func(values map[string]string, limit time.Duration, what string) (time.Duration, error) {
		n++
		stage := fmt.Sprintf("Timing %s (%d of up to %d)", what, n, total)
		report(Update{Stage: stage, Frac: float64(n-1) / float64(total)})
		dir := ws.path(fmt.Sprintf("trial-%d", n))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
		tctx, cancel := ctx, context.CancelFunc(func() {})
		if limit > 0 {
			tctx, cancel = context.WithTimeoutCause(ctx, limit, errSlower)
		}
		defer cancel()
		t := o
		t.EngineOptions = values
		j := t.sized(t.job(clip, filepath.Join(dir, "out"+o.Engine.OutputExt), ws.in, dir, report, stage, 0, 0), info.Width, info.Height)
		j.Progress = nil // the bar counts trials
		start := time.Now()
		_, err := o.Engine.Run(tctx, j)
		took := time.Since(start)
		if err != nil && ctx.Err() == nil && context.Cause(tctx) == errSlower {
			err = errSlower
		}
		return took, err
	}

	best, err := run(res.Best, 0, "the current settings")
	res.Trials = append(res.Trials, TuneTrial{Took: best, Err: err, Kept: err == nil})
	if err != nil {
		return res, err
	}
	for _, st := range spec.Steps {
		opt := o.Engine.OptionByID(st.Option)
		for _, v := range st.Values {
			if v == res.Best[st.Option] {
				continue
			}
			trial := maps.Clone(res.Best)
			trial[st.Option] = v
			// Model loading is part of every run, so a little slack keeps
			// a trial from being stopped just short of a tie.
			took, err := run(trial, best+best/4, strings.ToLower(opt.Name)+" "+opt.Label(v))
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			t := TuneTrial{Option: st.Option, Value: v, Took: took, Err: err}
			if err == nil && took < best*97/100 {
				t.Kept, best, res.Best = true, took, trial
			}
			res.Trials = append(res.Trials, t)
			if !t.Kept {
				break // values get more demanding: the next won't do better
			}
		}
	}
	report(Update{Stage: "Done", Frac: 1})
	return res, nil
}

// Describe writes up a tuning result: each trial's time per frame, what
// was kept, and the settings chosen.
func (r TuneResult) Describe(m *engine.Manifest) string {
	var b strings.Builder
	perFrame := func(d time.Duration) string {
		if r.Frames <= 0 {
			return d.Round(100 * time.Millisecond).String()
		}
		return fmt.Sprintf("%.2f s a frame", d.Seconds()/float64(r.Frames))
	}
	fmt.Fprintf(&b, "Timed on a %d-frame clip, model loading included.\n\n", r.Frames)
	for _, t := range r.Trials {
		name := "Current settings"
		if t.Option != "" {
			o := m.OptionByID(t.Option)
			name = o.Name + ": " + o.Label(t.Value)
		}
		switch {
		case t.Err != nil:
			fmt.Fprintf(&b, "✗ %s: %s\n", name, firstLine(t.Err.Error()))
		case t.Kept:
			fmt.Fprintf(&b, "✓ %s: %s (%s)\n", name, perFrame(t.Took), t.Took.Round(100*time.Millisecond))
		default:
			fmt.Fprintf(&b, "· %s: %s (%s), not faster\n", name, perFrame(t.Took), t.Took.Round(100*time.Millisecond))
		}
	}
	var changed []string
	for i := range m.Options {
		o := &m.Options[i]
		if v := r.Best[o.ID]; v != r.Start[o.ID] {
			changed = append(changed, fmt.Sprintf("  %s: %s → %s", o.Name, o.Label(r.Start[o.ID]), o.Label(v)))
		}
	}
	if len(changed) == 0 {
		b.WriteString("\nThe settings you had were the fastest that worked; nothing changed.\n")
	} else {
		b.WriteString("\nChanged:\n" + strings.Join(changed, "\n") + "\n")
	}
	if len(r.Trials) > 0 && r.Frames > 0 {
		first, last := r.Trials[0].Took, r.bestTime()
		if last < first {
			fmt.Fprintf(&b, "\n%.0f%% faster than before.\n", 100*(1-last.Seconds()/first.Seconds()))
		}
	}
	return b.String()
}

// bestTime is the time of the last kept trial.
func (r TuneResult) bestTime() time.Duration {
	var best time.Duration
	for _, t := range r.Trials {
		if t.Kept {
			best = t.Took
		}
	}
	return best
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
