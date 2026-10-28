package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mehran9675/touchup/internal/proc"
)

// Progress receives a fraction in [0, 1].
type Progress func(frac float64)

// ffmpeg runs FFmpeg with args, reporting progress against total
// seconds of output when onProgress is set.
func (t Tools) ffmpeg(ctx context.Context, total float64, onProgress Progress, args ...string) error {
	if t.FFmpeg == "" {
		return ErrNoFFmpeg
	}
	full := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y"}, args...)
	if onProgress != nil {
		full = append([]string{"-progress", "pipe:1", "-nostats"}, full...)
	}
	cmd := proc.Command(ctx, t.FFmpeg, full...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	proc.Lines(out, func(line string) {
		if onProgress == nil || total <= 0 {
			return
		}
		if v, ok := strings.CutPrefix(line, "out_time_us="); ok {
			if us, err := strconv.ParseFloat(v, 64); err == nil {
				onProgress(clamp01(us / 1e6 / total))
			}
		}
	})
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return toolError("ffmpeg", err, stderr.String())
	}
	return nil
}

// output runs FFmpeg and returns what it wrote to stdout.
func (t Tools) output(ctx context.Context, args ...string) ([]byte, error) {
	if t.FFmpeg == "" {
		return nil, ErrNoFFmpeg
	}
	full := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, args...)
	cmd := proc.Command(ctx, t.FFmpeg, full...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, toolError("ffmpeg", err, stderr.String())
	}
	return data, nil
}

func toolError(tool string, err error, stderr string) error {
	msg := strings.TrimSpace(stderr)
	if i := strings.LastIndex(msg, "\n"); i >= 0 {
		msg = strings.TrimSpace(msg[i+1:])
	}
	if msg == "" {
		return fmt.Errorf("%s: %w", tool, err)
	}
	return fmt.Errorf("%s: %s", tool, msg)
}

func clamp01(v float64) float64 {
	return max(0, min(1, v))
}

// seconds formats a time for FFmpeg's -ss and -t options.
func seconds(v float64) string {
	return strconv.FormatFloat(max(0, v), 'f', 3, 64)
}

var errEmpty = errors.New("ffmpeg produced no output")
