package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mehran9675/touchup/internal/proc"
)

// Job is one run of an engine.
type Job struct {
	Model *Model
	Scale int
	GPU   int // -1 lets the engine choose

	// Input is a file, or a folder for batch runs. Output is the file
	// the engine should write, or a folder for batch runs.
	Input, Output string
	// InputDir and OutputDir are the folders holding Input and Output.
	InputDir, OutputDir string
	// InWidth and InHeight are the input's size in pixels, for engines
	// that take a target resolution instead of a scale.
	InWidth, InHeight int
	// OutputExt overrides the extension of the file the engine writes.
	OutputExt string
	// PathDirs are put in front of PATH for the engine, so engines that
	// call FFmpeg themselves find the one TouchUp uses.
	PathDirs []string

	// Progress receives the engine's own percentage, when it prints one.
	Progress func(frac float64)
	// Log receives each line the engine prints.
	Log func(line string)
}

var defaultProgress = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// Run runs the engine and returns the file it produced: Output when the
// engine wrote it, otherwise the newest file with the engine's output
// extension in OutputDir (for engines that pick their own names). Batch
// runs return OutputDir.
func (m *Manifest) Run(ctx context.Context, j Job) (string, error) {
	exe := m.ExecutablePath()
	if exe == "" {
		return "", fmt.Errorf("%s is not installed", m.Name)
	}
	if j.Model != nil && !m.ModelInstalled(j.Model) {
		return "", fmt.Errorf("the %s model of %s is not installed", j.Model.Name, m.Name)
	}
	if err := installScripts(m); err != nil {
		return "", fmt.Errorf("updating %s's scripts: %w", m.Name, err)
	}
	args := m.Command(j)
	cmd := proc.Command(ctx, exe, args...)
	cmd.Dir = m.workingDir(exe)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1", "PYTHONIOENCODING=utf-8")
	if len(j.PathDirs) > 0 {
		dirs := append(slices.Clone(j.PathDirs), os.Getenv("PATH"))
		cmd.Env = append(cmd.Env, "PATH="+strings.Join(dirs, string(os.PathListSeparator)))
	}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw

	re := defaultProgress
	if m.Progress != "" {
		if custom, err := regexp.Compile(m.Progress); err == nil {
			re = custom
		}
	}
	tail := &tailBuffer{max: 400}
	done := make(chan struct{})
	go func() {
		defer close(done)
		proc.Lines(pr, func(raw string) {
			line := strings.TrimSpace(raw)
			if line == "" {
				return
			}
			tail.add(raw)
			if j.Log != nil {
				j.Log(line)
			}
			if j.Progress == nil {
				return
			}
			if mm := re.FindStringSubmatch(line); len(mm) > 1 {
				if v, err := strconv.ParseFloat(mm[1], 64); err == nil {
					j.Progress(min(1, max(0, v/100)))
				}
			}
		})
	}()
	started := time.Now()
	err := cmd.Run()
	pw.Close()
	<-done
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", &RunError{Engine: m.Name, Summary: summarize(tail.lines), Log: tail.String()}
	}
	if _, err := os.Stat(j.Output); err == nil {
		return j.Output, nil
	}
	ext := m.OutputExt
	if j.OutputExt != "" {
		ext = j.OutputExt
	}
	if found := newestOutput(j.OutputDir, ext, started); found != "" {
		return found, nil
	}
	return "", &RunError{Engine: m.Name, Summary: "finished without writing a " + ext + " file", Log: tail.String()}
}

// Command returns the engine's arguments with the placeholders filled in.
func (m *Manifest) Command(j Job) []string {
	vars := map[string]string{
		"{input}":     j.Input,
		"{output}":    j.Output,
		"{inputDir}":  j.InputDir,
		"{outputDir}": j.OutputDir,
		"{scale}":     strconv.Itoa(max(1, j.Scale)),
		"{gpu}":       strconv.Itoa(j.GPU),
		"{engineDir}": m.Dir(),
		"{model}":     "",
		"{modelDir}":  m.ModelsDir(),
		"{modelPath}": m.ModelPath(j.Model),
		"{inWidth}":   strconv.Itoa(j.InWidth),
		"{inHeight}":  strconv.Itoa(j.InHeight),
		"{outWidth}":  strconv.Itoa(j.InWidth * max(1, j.Scale)),
		"{outHeight}": strconv.Itoa(j.InHeight * max(1, j.Scale)),
		"{outShort}":  strconv.Itoa(min(j.InWidth, j.InHeight) * max(1, j.Scale)),
	}
	if j.Model != nil {
		vars["{model}"] = j.Model.ID
	}
	expand := func(a string) string {
		for k, v := range vars {
			a = strings.ReplaceAll(a, k, v)
		}
		return a
	}
	var out []string
	for _, a := range m.Args {
		if a == "{modelArgs}" {
			if j.Model != nil {
				for _, ma := range j.Model.Args {
					out = append(out, expand(ma))
				}
			}
			continue
		}
		out = append(out, expand(a))
	}
	if j.GPU >= 0 {
		for _, a := range m.GPUArgs {
			out = append(out, expand(a))
		}
	}
	return out
}

// workingDir is where the engine runs: its install folder, where ncnn
// engines look for their models, or the executable's folder.
func (m *Manifest) workingDir(exe string) string {
	if m.Install.Type == "archive" {
		return m.Dir()
	}
	return filepath.Dir(exe)
}

// newestOutput finds the newest file with ext written under dir since
// started.
func newestOutput(dir, ext string, started time.Time) string {
	var best string
	var bestTime time.Time
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ext) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(started.Add(-2*time.Second)) {
			return nil
		}
		if best == "" || info.ModTime().After(bestTime) {
			best, bestTime = p, info.ModTime()
		}
		return nil
	})
	return best
}

// RunError is an engine or install step that failed: a one-line Summary
// to show, and the end of what it printed, for a details view.
type RunError struct {
	Engine  string
	Summary string
	Log     string
}

func (e *RunError) Error() string { return e.Engine + " failed: " + e.Summary }

// summarize picks the line that says why a run failed: the last "Error:"
// line (TouchUp's runner scripts end with one), else the last line that
// isn't part of a traceback's code listing.
func summarize(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if s, ok := strings.CutPrefix(strings.TrimSpace(lines[i]), "Error: "); ok {
			return s
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" || strings.Trim(l, "^~ ") == "" || strings.HasPrefix(l, "File \"") {
			continue
		}
		return l
	}
	return "it exited without saying why"
}

// LogOf returns the engine output behind err, if it's a RunError.
func LogOf(err error) string {
	var re *RunError
	if errors.As(err, &re) {
		return re.Log
	}
	return ""
}
