package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mehran9675/touchup/internal/media"
)

// Video upscales the video at in and writes it to out, keeping its
// audio. Image engines run on every frame; video engines on the file.
func Video(ctx context.Context, o Options, in, out string, info media.Info, report Reporter) error {
	if err := o.check(); err != nil {
		return err
	}
	ws, err := newWorkspace("video", o.Settings.KeepWork)
	if err != nil {
		return err
	}
	defer ws.close()
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if o.Engine.Input == "video" {
		return wholeVideo(ctx, o, ws, in, out, info, report)
	}
	return frameVideo(ctx, o, ws, in, out, info, report)
}

// frameVideo splits the video into frames, upscales them and encodes
// them back with the original audio.
func frameVideo(ctx context.Context, o Options, ws *workspace, in, out string, info media.Info, report Reporter) error {
	if err := o.Tools.SplitFrames(ctx, in, ws.in, info.Duration, func(f float64) {
		report(Update{Stage: "Extracting frames", Frac: 0.08 * f})
	}); err != nil {
		return err
	}
	frames, _ := filepath.Glob(filepath.Join(ws.in, "*.png"))
	if len(frames) == 0 {
		return errors.New("no frames could be read from the video")
	}
	total := float64(len(frames))
	const from, to = 0.08, 0.9
	stage := "Upscaling frames"

	if o.Engine.Batch {
		// One run over the folder; follow progress by counting results.
		stop := make(chan struct{})
		go func() {
			t := time.NewTicker(400 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					done, _ := filepath.Glob(filepath.Join(ws.out, "*"+o.Engine.OutputExt))
					report(Update{Stage: stage, Frac: from + (to-from)*float64(len(done))/total})
				}
			}
		}()
		j := o.job(ws.in, ws.out, ws.in, ws.out, report, stage, from, to)
		j.Progress = nil // per-frame percentages would jump around
		_, err := o.Engine.Run(ctx, j)
		close(stop)
		if err != nil {
			return err
		}
	} else {
		// Engines that name their own output write into frameDir; the
		// result is moved in line with the other frames.
		frameDir := ws.path("frame-out")
		if err := os.MkdirAll(frameDir, 0o755); err != nil {
			return err
		}
		for i, frame := range frames {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := filepath.Base(frame)
			target := filepath.Join(ws.out, name[:len(name)-len(filepath.Ext(name))]+o.Engine.OutputExt)
			j := o.job(frame, target, ws.in, frameDir, report, stage, from, to)
			j.Progress = nil
			result, err := o.Engine.Run(ctx, j)
			if err != nil {
				return err
			}
			if result != target {
				if err := os.Rename(result, target); err != nil {
					return err
				}
			}
			os.Remove(frame) // keep disk use in check on long videos
			report(Update{Stage: stage, Frac: from + (to-from)*float64(i+1)/total})
		}
	}
	return o.Tools.JoinFrames(ctx, ws.out, out, media.JoinOptions{
		FPS: info.FPS, FrameExt: o.Engine.OutputExt, Codec: o.Settings.VideoCodec, Quality: o.Settings.VideoQuality,
		AudioSrc: in, Duration: info.Duration,
	}, func(f float64) {
		report(Update{Stage: "Encoding", Frac: to + (1-to)*f})
	})
}

// wholeVideo hands the file to a video engine, then copies the source's
// audio into the result.
func wholeVideo(ctx context.Context, o Options, ws *workspace, in, out string, info media.Info, report Reporter) error {
	src := filepath.Join(ws.in, "source"+filepath.Ext(in))
	if err := os.Link(in, src); err != nil {
		if err := copyInto(in, src); err != nil {
			return err
		}
	}
	result, err := o.Engine.Run(ctx, o.job(src, filepath.Join(ws.out, "source"+o.Engine.OutputExt),
		ws.in, ws.out, report, "Upscaling", 0.02, 0.92))
	if err != nil {
		return err
	}
	report(Update{Stage: "Adding audio", Frac: 0.93})
	return o.Tools.Remux(ctx, result, in, out)
}
