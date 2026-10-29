// Package format turns numbers into the short strings the UI shows.
package format

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// Clock formats seconds as m:ss.cc, or h:mm:ss for long media.
func Clock(sec float64) string {
	sec = max(0, sec)
	if sec >= 3600 {
		h := int(sec) / 3600
		return fmt.Sprintf("%d:%02d:%02d", h, int(sec)%3600/60, int(sec)%60)
	}
	m := int(sec) / 60
	s := sec - float64(m*60)
	return fmt.Sprintf("%d:%05.2f", m, s)
}

// Duration formats seconds compactly: "42s", "3m 05s", "1h 02m".
func Duration(sec float64) string {
	d := time.Duration(sec * float64(time.Second)).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
}

// Bytes formats a size: "12.4 MB".
func Bytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n)
	i := -1
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// Rate formats a sample rate: "44.1 kHz".
func Rate(hz int) string {
	if hz <= 0 {
		return "—"
	}
	k := float64(hz) / 1000
	if k == math.Trunc(k) {
		return fmt.Sprintf("%d kHz", int(k))
	}
	return fmt.Sprintf("%.1f kHz", k)
}

// Resolution formats a size: "1920 × 1080".
func Resolution(w, h int) string { return fmt.Sprintf("%d × %d", w, h) }

// FPS formats a frame rate: "23.98 fps", "30 fps".
func FPS(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%d fps", int(f))
	}
	return fmt.Sprintf("%.2f fps", f)
}

// Channels names a channel count.
func Channels(n int) string {
	switch n {
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	}
	return fmt.Sprintf("%d ch", n)
}

// Elapsed formats how long something took or has been running.
func Elapsed(d time.Duration) string { return Duration(d.Seconds()) }

// Remaining formats an estimate of the time left, to the minute: "1 min",
// "45 min", "3h 20m".
func Remaining(d time.Duration) string {
	m := max(1, int(d.Round(time.Minute).Minutes()))
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%dh %02dm", m/60, m%60)
}

// Count formats a count with thousands separators: "35,787".
func Count(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
