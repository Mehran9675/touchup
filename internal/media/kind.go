// Package media inspects and transforms media files with FFmpeg:
// probing, frame and clip extraction, waveforms, spectrograms, frame
// splitting and joining, format conversion and playback.
package media

import (
	"path/filepath"
	"strings"
)

// Kind is the type of a media file.
type Kind int

const (
	Unknown Kind = iota
	Image
	Video
	Audio
)

// String returns the kind's lowercase name, as used in engine manifests.
func (k Kind) String() string {
	switch k {
	case Image:
		return "image"
	case Video:
		return "video"
	case Audio:
		return "audio"
	}
	return "unknown"
}

// Title returns the kind's display name.
func (k Kind) Title() string {
	switch k {
	case Image:
		return "Image"
	case Video:
		return "Video"
	case Audio:
		return "Audio"
	}
	return "File"
}

var extensions = map[Kind][]string{
	Image: {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff", ".gif"},
	Video: {".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v", ".wmv", ".flv", ".mpg", ".mpeg", ".ts"},
	Audio: {".wav", ".mp3", ".flac", ".m4a", ".aac", ".ogg", ".opus", ".wma", ".aif", ".aiff"},
}

// Detect returns the kind of the file at path from its extension.
func Detect(path string) Kind {
	ext := strings.ToLower(filepath.Ext(path))
	for k, exts := range extensions {
		for _, e := range exts {
			if e == ext {
				return k
			}
		}
	}
	return Unknown
}

// Extensions lists every supported extension, without dots, for file
// pickers.
func Extensions() []string {
	var all []string
	for _, k := range []Kind{Image, Video, Audio} {
		for _, e := range extensions[k] {
			all = append(all, strings.TrimPrefix(e, "."))
		}
	}
	return all
}
