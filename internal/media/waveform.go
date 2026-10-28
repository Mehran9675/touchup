package media

import (
	"context"
	"encoding/binary"
	"math"
)

// waveformRate is the sample rate audio is decoded at for the waveform:
// enough for the peaks, cheap for hour-long files.
const waveformRate = 4000

// Waveform returns n peak levels in [0, 1] across the whole file.
func (t Tools) Waveform(ctx context.Context, path string, n int) ([]float32, error) {
	data, err := t.output(ctx, "-i", path, "-vn", "-ac", "1", "-ar", itoa(waveformRate),
		"-f", "s16le", "-acodec", "pcm_s16le", "-")
	if err != nil {
		return nil, err
	}
	samples := len(data) / 2
	if samples == 0 || n <= 0 {
		return nil, errEmpty
	}
	peaks := make([]float32, n)
	var top float32
	for i := range n {
		from, to := i*samples/n, (i+1)*samples/n
		var peak, sum float64
		for s := from; s < to; s++ {
			v := math.Abs(float64(int16(binary.LittleEndian.Uint16(data[s*2:]))) / 32768)
			peak = max(peak, v)
			sum += v * v
		}
		// Blend peak and RMS: pure peaks look like a solid block on
		// loud masters.
		rms := 0.0
		if to > from {
			rms = math.Sqrt(sum / float64(to-from))
		}
		peaks[i] = float32(0.35*peak + 0.65*rms*1.8)
		top = max(top, peaks[i])
	}
	if top > 0 {
		for i := range peaks {
			peaks[i] = min(1, peaks[i]/top)
		}
	}
	return peaks, nil
}
