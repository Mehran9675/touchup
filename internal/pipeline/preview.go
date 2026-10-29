package pipeline

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
)

// FramePreview is a single video frame before and after upscaling.
type FramePreview struct {
	Before, After image.Image
}

// clipLength is how much video a video engine gets for a preview.
const clipLength = 1.0

// PreviewFrame upscales the single frame at t seconds. Image engines get
// the frame itself; video engines get a one-second clip, so they still
// see motion, and the frame is taken from its middle.
func PreviewFrame(ctx context.Context, o Options, path string, at float64, report Reporter) (FramePreview, error) {
	if err := o.check(); err != nil {
		return FramePreview{}, err
	}
	ws, err := newWorkspace("preview", false)
	if err != nil {
		return FramePreview{}, err
	}
	defer ws.close()

	// Engines that read whole videos get a short clip around the frame,
	// unless they also take images: then the frame alone is enough, and a
	// clip would be dozens of frames of work for one.
	if o.Engine.Input == "video" && !o.Engine.Handles("image") {
		start := max(0, at-clipLength/2)
		clip := filepath.Join(ws.in, "clip.mp4")
		report(Update{Stage: "Cutting a clip", Frac: 0.05})
		if err := o.Tools.VideoClip(ctx, path, start, clipLength, clip); err != nil {
			return FramePreview{}, err
		}
		info, err := o.Tools.Probe(ctx, clip)
		if err != nil {
			return FramePreview{}, err
		}
		report(Update{Stage: "Upscaling", Frac: 0.1})
		result, err := o.Engine.Run(ctx, o.sized(o.job(clip, filepath.Join(ws.out, "clip"+o.Engine.OutputExt),
			ws.in, ws.out, report, "Upscaling", 0.1, 0.95), info.Width, info.Height))
		if err != nil {
			return FramePreview{}, err
		}
		before, err := o.Tools.FrameAt(ctx, clip, at-start, 0)
		if err != nil {
			return FramePreview{}, err
		}
		after, err := o.Tools.FrameAt(ctx, result, at-start, 0)
		return FramePreview{Before: before, After: after}, err
	}

	frame := filepath.Join(ws.in, "frame.png")
	report(Update{Stage: "Grabbing the frame", Frac: 0.05})
	if err := o.Tools.SaveFrameAt(ctx, path, at, frame); err != nil {
		return FramePreview{}, err
	}
	before, err := o.Tools.LoadImage(ctx, frame)
	if err != nil {
		return FramePreview{}, err
	}
	b := before.Bounds()
	sz := o.size(b.Dx(), b.Dy())
	stage := fmt.Sprintf("Upscaling to %d × %d", sz.Width, sz.Height)
	report(Update{Stage: stage, Frac: 0.1})
	j := o.job(frame, filepath.Join(ws.out, "frame"+o.imageExt()), ws.in, ws.out, report, stage, 0.1, 0.95)
	j.OutputExt = o.imageExt()
	result, err := o.Engine.Run(ctx, o.sized(j, b.Dx(), b.Dy()))
	if err != nil {
		return FramePreview{}, err
	}
	if sz.Exact {
		sized := filepath.Join(ws.out, "frame-sized.png")
		if err := o.Tools.ConvertImageSized(ctx, result, sized, sz.Width, sz.Height); err != nil {
			return FramePreview{}, err
		}
		result = sized
	}
	after, err := o.Tools.LoadImage(ctx, result)
	return FramePreview{Before: before, After: after}, err
}

// AudioPreview is a short section of audio before and after enhancing,
// as playable files with their spectrograms.
type AudioPreview struct {
	Before, After         string // WAV files, kept until Discard
	BeforeSpec, AfterSpec image.Image
	BeforeRate, AfterRate int
	dir                   string
}

// Discard deletes the preview's files.
func (p *AudioPreview) Discard() {
	if p != nil && p.dir != "" {
		os.RemoveAll(p.dir)
	}
}

// PreviewAudio enhances length seconds of audio starting at start.
func PreviewAudio(ctx context.Context, o Options, path string, start, length float64, report Reporter) (*AudioPreview, error) {
	if err := o.check(); err != nil {
		return nil, err
	}
	ws, err := newWorkspace("listen", true)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(ws.root)
		}
	}()

	clip := filepath.Join(ws.in, "section.wav")
	report(Update{Stage: "Cutting the section", Frac: 0.03})
	if err := o.Tools.AudioClip(ctx, path, start, length, o.Engine.InputRate, clip, nil); err != nil {
		return nil, err
	}
	report(Update{Stage: "Enhancing", Frac: 0.06})
	result, err := o.Engine.Run(ctx, o.job(clip, filepath.Join(ws.out, "section"+o.Engine.OutputExt),
		ws.in, ws.out, report, "Enhancing", 0.06, 0.9))
	if err != nil {
		return nil, err
	}
	report(Update{Stage: "Drawing spectrograms", Frac: 0.92})
	before, _ := o.Tools.Probe(ctx, clip)
	after, _ := o.Tools.Probe(ctx, result)
	rate := max(before.SampleRate, after.SampleRate, 8000)
	const w, h = 1200, 420
	beforeSpec, err := o.Tools.Spectrogram(ctx, clip, rate, w, h)
	if err != nil {
		return nil, err
	}
	afterSpec, err := o.Tools.Spectrogram(ctx, result, rate, w, h)
	if err != nil {
		return nil, err
	}
	ok = true
	return &AudioPreview{
		Before: clip, After: result,
		BeforeSpec: beforeSpec, AfterSpec: afterSpec,
		BeforeRate: before.SampleRate, AfterRate: after.SampleRate,
		dir: ws.root,
	}, nil
}
