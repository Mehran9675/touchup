package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"sync"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// FrameAt decodes the video frame at t seconds. A positive maxHeight
// scales it down to that height, for thumbnails.
func (t Tools) FrameAt(ctx context.Context, path string, at float64, maxHeight int) (image.Image, error) {
	args := []string{"-ss", seconds(at), "-i", path, "-frames:v", "1"}
	if maxHeight > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=-2:'min(%d,ih)'", maxHeight))
	}
	args = append(args, "-f", "image2pipe", "-vcodec", "png", "-")
	data, err := t.output(ctx, args...)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errEmpty
	}
	return png.Decode(bytes.NewReader(data))
}

// SaveFrameAt writes the video frame at t seconds to out as PNG.
func (t Tools) SaveFrameAt(ctx context.Context, path string, at float64, out string) error {
	return t.ffmpeg(ctx, 0, nil, "-ss", seconds(at), "-i", path, "-frames:v", "1", out)
}

// Filmstrip decodes n evenly spaced thumbnails of a video, in parallel.
// Thumbnails that fail to decode are left nil.
func (t Tools) Filmstrip(ctx context.Context, path string, duration float64, n, height int) []image.Image {
	thumbs := make([]image.Image, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			at := duration * (float64(i) + 0.5) / float64(n)
			if img, err := t.FrameAt(ctx, path, at, height); err == nil {
				thumbs[i] = img
			}
		}()
	}
	wg.Wait()
	return thumbs
}

// LoadImage decodes an image file, through FFmpeg for formats Go
// can't read.
func (t Tools) LoadImage(ctx context.Context, path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err == nil {
		return img, nil
	}
	data, ferr := t.output(ctx, "-i", path, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "png", "-")
	if ferr != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}
