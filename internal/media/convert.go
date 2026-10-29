package media

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ConvertImage writes the image at in to out in the format named by
// out's extension, copying when the formats already match.
func (t Tools) ConvertImage(ctx context.Context, in, out string) error {
	return t.ConvertImageSized(ctx, in, out, 0, 0)
}

// ConvertImageSized is ConvertImage that also resizes the image to w×h
// with Lanczos filtering, unless w or h is 0.
func (t Tools) ConvertImageSized(ctx context.Context, in, out string, w, h int) error {
	if sameExt(in, out) && (w <= 0 || h <= 0) {
		return copyFile(in, out)
	}
	args := []string{"-i", in}
	if w > 0 && h > 0 {
		args = append(args, "-vf", scaleFilter(w, h))
	}
	switch strings.ToLower(filepath.Ext(out)) {
	case ".jpg", ".jpeg":
		args = append(args, "-q:v", "1")
	case ".webp":
		args = append(args, "-lossless", "0", "-quality", "95")
	}
	return t.ffmpeg(ctx, 0, nil, append(args, out)...)
}

// ConvertAudio writes the audio at in to out in the format named by
// out's extension.
func (t Tools) ConvertAudio(ctx context.Context, in, out string) error {
	if sameExt(in, out) {
		return copyFile(in, out)
	}
	args := []string{"-i", in, "-vn"}
	switch strings.ToLower(filepath.Ext(out)) {
	case ".mp3":
		args = append(args, "-c:a", "libmp3lame", "-b:a", "320k")
	case ".flac":
		args = append(args, "-c:a", "flac", "-compression_level", "8")
	case ".wav":
		args = append(args, "-c:a", "pcm_s24le")
	}
	return t.ffmpeg(ctx, 0, nil, append(args, out)...)
}

func sameExt(a, b string) bool {
	ea, eb := strings.ToLower(filepath.Ext(a)), strings.ToLower(filepath.Ext(b))
	return ea == eb || (ea == ".jpeg" && eb == ".jpg") || (ea == ".jpg" && eb == ".jpeg")
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
