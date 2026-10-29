package pipeline

import (
	"context"
	"os"
	"path/filepath"
)

// Image upscales the image at in and writes it to out.
func Image(ctx context.Context, o Options, in, out string, report Reporter) error {
	if err := o.check(); err != nil {
		return err
	}
	ws, err := newWorkspace("image", o.Settings.KeepWork)
	if err != nil {
		return err
	}
	defer ws.close()

	report(Update{Stage: "Preparing", Frac: 0.02})
	src := filepath.Join(ws.in, "source.png")
	if err := o.Tools.ConvertImage(ctx, in, src); err != nil {
		return err
	}
	info, err := o.Tools.Probe(ctx, src)
	if err != nil {
		return err
	}
	report(Update{Stage: "Upscaling", Frac: 0.05})
	j := o.job(src, filepath.Join(ws.out, "source"+o.imageExt()), ws.in, ws.out, report, "Upscaling", 0.05, 0.95)
	j.OutputExt = o.imageExt()
	result, err := o.Engine.Run(ctx, o.sized(j, info.Width, info.Height))
	if err != nil {
		return err
	}
	report(Update{Stage: "Saving", Frac: 0.96})
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if sz := o.size(info.Width, info.Height); sz.Exact {
		return o.Tools.ConvertImageSized(ctx, result, out, sz.Width, sz.Height)
	}
	return o.Tools.ConvertImage(ctx, result, out)
}
