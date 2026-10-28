package media

import (
	"context"
	"strconv"
)

// AudioClip decodes length seconds of audio starting at start into a
// 16-bit WAV file at out, keeping the source's channels, and its sample
// rate unless rate is set. A zero length takes the whole file.
func (t Tools) AudioClip(ctx context.Context, path string, start, length float64, rate int, out string, onProgress Progress) error {
	args := []string{}
	if start > 0 {
		args = append(args, "-ss", seconds(start))
	}
	if length > 0 {
		args = append(args, "-t", seconds(length))
	}
	args = append(args, "-i", path, "-vn", "-map", "0:a:0", "-acodec", "pcm_s16le")
	if rate > 0 {
		args = append(args, "-ar", itoa(rate))
	}
	args = append(args, out)
	return t.ffmpeg(ctx, length, onProgress, args...)
}

// VideoClip re-encodes length seconds of video around start into a
// near-lossless clip at out, for previewing engines that take whole
// videos.
func (t Tools) VideoClip(ctx context.Context, path string, start, length float64, out string) error {
	return t.ffmpeg(ctx, 0, nil, "-ss", seconds(start), "-t", seconds(length), "-i", path,
		"-map", "0:v:0", "-an", "-c:v", "libx264", "-crf", "8", "-preset", "veryfast",
		"-pix_fmt", "yuv420p", out)
}

func itoa(v int) string { return strconv.Itoa(v) }
