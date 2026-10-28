package media

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mehran9675/touchup/internal/proc"
)

// Info describes a media file.
type Info struct {
	Duration   float64 // seconds; 0 for images
	Width      int
	Height     int
	FPS        float64
	Frames     int
	VideoCodec string
	HasAudio   bool
	AudioCodec string
	SampleRate int
	Channels   int
	Bitrate    int // bits per second, whole file
	Size       int64
}

type probeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
		NbFrames     string `json:"nb_frames"`
		SampleRate   string `json:"sample_rate"`
		Channels     int    `json:"channels"`
		Duration     string `json:"duration"`
		Disposition  struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
		Size     string `json:"size"`
	} `json:"format"`
}

// Probe reads a file's streams with FFprobe.
func (t Tools) Probe(ctx context.Context, path string) (Info, error) {
	if t.FFprobe == "" {
		return Info{}, ErrNoFFmpeg
	}
	cmd := proc.Command(ctx, t.FFprobe, "-v", "error", "-print_format", "json",
		"-show_streams", "-show_format", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return Info{}, toolError("ffprobe", err, stderr.String())
	}
	var p probeOutput
	if err := json.Unmarshal(data, &p); err != nil {
		return Info{}, err
	}
	var info Info
	info.Duration = parseFloat(p.Format.Duration)
	info.Bitrate = int(parseFloat(p.Format.BitRate))
	info.Size = int64(parseFloat(p.Format.Size))
	for _, s := range p.Streams {
		switch s.CodecType {
		case "video":
			if info.Width != 0 || s.Disposition.AttachedPic == 1 {
				continue
			}
			info.Width, info.Height = s.Width, s.Height
			info.VideoCodec = s.CodecName
			info.FPS = parseRate(s.AvgFrameRate)
			if info.FPS == 0 {
				info.FPS = parseRate(s.RFrameRate)
			}
			info.Frames, _ = strconv.Atoi(s.NbFrames)
		case "audio":
			if info.HasAudio {
				continue
			}
			info.HasAudio = true
			info.AudioCodec = s.CodecName
			info.SampleRate, _ = strconv.Atoi(s.SampleRate)
			info.Channels = s.Channels
		}
	}
	if info.Frames == 0 && info.FPS > 0 {
		info.Frames = int(info.Duration*info.FPS + 0.5)
	}
	return info, nil
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseRate parses FFprobe's "30000/1001" frame rates.
func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return parseFloat(s)
	}
	d := parseFloat(den)
	if d == 0 {
		return 0
	}
	return parseFloat(num) / d
}
