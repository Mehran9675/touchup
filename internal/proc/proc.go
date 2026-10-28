// Package proc starts child processes the way TouchUp needs them:
// without a console window on Windows, and with their output split into
// lines on both \n and \r so progress meters can be followed.
package proc

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
)

// Command builds an exec.Cmd that doesn't open a console window.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	return cmd
}

// Lines calls fn for every line read from r, treating a lone \r as a
// line end too, since progress meters redraw with it.
func Lines(r io.Reader, fn func(line string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sc.Split(scanLines)
	for sc.Scan() {
		fn(sc.Text())
	}
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, bytes.TrimSpace(data[:i]), nil
	}
	if atEOF {
		return len(data), bytes.TrimSpace(data), nil
	}
	return 0, nil, nil
}
