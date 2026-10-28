package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
)

// Spectrogram renders a file's frequency content as an image, frequency
// rising upward on a linear scale. The audio is resampled to rate first
// so two files with different sample rates line up: a 22 kHz source
// shows the empty band an upscaler is meant to fill.
func (t Tools) Spectrogram(ctx context.Context, path string, rate, width, height int) (image.Image, error) {
	filter := fmt.Sprintf("aresample=%d,aformat=channel_layouts=mono,"+
		"showspectrumpic=s=%dx%d:legend=0:color=magma:scale=log:fscale=lin:gain=2", rate, width, height)
	data, err := t.output(ctx, "-i", path, "-lavfi", filter, "-f", "image2pipe", "-vcodec", "png", "-")
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}
