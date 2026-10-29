package store

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/mehran9675/giode"
)

// ErrorReport is the whole of an error as plain text to copy into a bug
// report: what failed, why, the computer and setup it ran on, and
// everything the engine or installer printed that TouchUp kept.
func (s *Store) ErrorReport(title, summary, log string) string {
	var b strings.Builder
	b.WriteString(title + "\n")
	if summary != "" {
		b.WriteString(summary + "\n")
	}
	version := giode.AppVersion
	if version == "" {
		version = "development build"
	}
	b.WriteString("\nTouchUp " + version + " on " + runtime.GOOS + "/" + runtime.GOARCH + "\n")
	gpu := "automatic"
	if s.Settings.GPU >= 0 {
		gpu = strconv.Itoa(s.Settings.GPU)
	}
	b.WriteString("GPU setting: " + gpu + "\n")
	if s.Python != "" {
		b.WriteString("Python: " + s.Python + "\n")
	}
	if log != "" {
		b.WriteString("\nOutput:\n" + log + "\n")
	}
	return b.String()
}
