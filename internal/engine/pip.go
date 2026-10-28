package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mehran9675/touchup/internal/proc"
)

// FindPython returns the interpreter to create environments with: the
// configured one, or python3/python on PATH.
func FindPython(configured string) string {
	if configured != "" {
		return configured
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func installPip(ctx context.Context, m *Manifest, python string, report Report) error {
	python = FindPython(python)
	if python == "" {
		return errors.New("Python was not found: install Python 3 or set its path in Settings")
	}
	venv := filepath.Join(m.Dir(), "venv")
	report(Status{Stage: "Creating Python environment", Frac: -1})
	if err := runLogged(ctx, report, "Creating Python environment", python, "-m", "venv", venv); err != nil {
		return err
	}
	vpy := filepath.Join(m.venvBin(), "python")
	if runtime.GOOS == "windows" {
		vpy += ".exe"
	}
	report(Status{Stage: "Updating pip", Frac: -1})
	if err := runLogged(ctx, report, "Updating pip", vpy, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
		return err
	}
	args := append([]string{"-m", "pip", "install", "--progress-bar", "off"}, m.Install.Packages...)
	return runLogged(ctx, report, "Installing packages", vpy, args...)
}

// runLogged runs a command, passing each output line to report, and
// returns the tail of its output on failure.
func runLogged(ctx context.Context, report Report, stage, name string, args ...string) error {
	cmd := proc.Command(ctx, name, args...)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	tail := &tailBuffer{max: 12}
	done := make(chan struct{})
	go func() {
		proc.Lines(pr, func(line string) {
			if line == "" {
				return
			}
			tail.add(line)
			report(Status{Stage: stage, Frac: -1, Line: line})
		})
		close(done)
	}()
	err := cmd.Run()
	pw.Close()
	<-done
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s failed: %s", strings.ToLower(stage), tail.String())
	}
	return nil
}

// tailBuffer keeps the last lines of a command's output.
type tailBuffer struct {
	lines []string
	max   int
}

func (t *tailBuffer) add(line string) {
	t.lines = append(t.lines, line)
	if len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

func (t *tailBuffer) String() string {
	var b bytes.Buffer
	for i, l := range t.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	return b.String()
}
