package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mehran9675/touchup/internal/settings"
)

// OutputPath returns where the result of upscaling input goes: next to
// it or in the configured folder, named "<name> (TouchUp 4x).<ext>", with
// a number added when that file exists or taken reports the name as
// reserved (by a job still in the queue).
func OutputPath(input string, s settings.Settings, size string, ext string, taken func(string) bool) string {
	dir := s.OutputDir
	if dir == "" {
		dir = filepath.Dir(input)
	}
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	tag := "TouchUp"
	if size != "" {
		tag += " " + size
	}
	candidate := filepath.Join(dir, fmt.Sprintf("%s (%s)%s", base, tag, ext))
	for i := 2; exists(candidate) || taken != nil && taken(candidate); i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%s %d)%s", base, tag, i, ext))
	}
	return candidate
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
