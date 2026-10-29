package store

import (
	"strings"
	"time"
)

// liveLogLines is how much of a running engine's output is kept.
const liveLogLines = 400

// LiveLog is the end of what a running engine has printed, so it can be
// copied before the run ends, and when it last printed something.
type LiveLog struct {
	lines []string
	Last  time.Time
}

func (l *LiveLog) add(line string) {
	l.lines = append(l.lines, line)
	if len(l.lines) > liveLogLines {
		l.lines = l.lines[len(l.lines)-liveLogLines:]
	}
	l.Last = time.Now()
}

func (l *LiveLog) reset() { *l = LiveLog{Last: time.Now()} }

func (l *LiveLog) String() string { return strings.Join(l.lines, "\n") }
