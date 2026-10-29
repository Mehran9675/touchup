package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mehran9675/touchup/internal/engine"
)

func TestTune(t *testing.T) {
	tools := setup(t)
	// Its time depends on its settings: more memory helps up to 2, a
	// 768 tile helps, 1024 runs out of memory, and the switch is slow.
	exe := script(t, `in="$1"; out="$2"; shift 2
delay=0.8
while [ $# -gt 0 ]; do
  case "$1" in
    --mem) case "$2" in 1) delay=0.6 ;; 2) delay=0.4 ;; 4) delay=0.5 ;; esac; shift ;;
    --tile) case "$2" in 768) delay=$(echo "$delay - 0.2" | bc) ;; 1024) echo "torch.OutOfMemoryError: out of memory"; exit 1 ;; esac; shift ;;
    --slow) delay=30 ;;
  esac
  shift
done
sleep "$delay"
cp "$in" "$out"
`)
	ch := func(vs ...string) []engine.OptionChoice {
		var out []engine.OptionChoice
		for _, v := range vs {
			out = append(out, engine.OptionChoice{Value: v, Label: v})
		}
		return out
	}
	m := &engine.Manifest{
		ID: "tunable", Name: "Tunable", Media: []string{"video"}, Input: "video",
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}"}, OutputExt: ".mp4",
		Options: []engine.Option{
			{ID: "mem", Name: "Memory", Type: engine.OptionChoiceType, Default: "0.5", Choices: ch("0.5", "1", "2", "4"), Args: []string{"--mem", "{value}"}},
			{ID: "tile", Name: "Tile", Type: engine.OptionChoiceType, Default: "512", Choices: ch("512", "768", "1024"), Args: []string{"--tile", "{value}"}},
			{ID: "slow", Name: "Slow", Type: engine.OptionToggle, Default: "false", Args: []string{"--slow"}},
		},
		Tune: &engine.Tuning{Seconds: 1, Steps: []engine.TuneStep{
			{Option: "mem", Values: []string{"1", "2", "4"}},
			{Option: "tile", Values: []string{"768", "1024"}},
			{Option: "slow", Values: []string{"true"}},
		}},
	}
	o := options(t, tools, m)
	o.EngineOptions = m.OptionValues(nil)
	video := t.TempDir() + "/in.mp4"
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=10", "-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)

	start := time.Now()
	res, err := Tune(context.Background(), o, video, 1, noReport)
	if err != nil {
		t.Fatal(err)
	}
	if res.Best["mem"] != "2" || res.Best["tile"] != "768" || res.Best["slow"] != "false" {
		t.Errorf("kept %v, want mem 2, tile 768, slow off", res.Best)
	}
	if took := time.Since(start); took > 15*time.Second {
		t.Errorf("took %v: the slow trial wasn't stopped", took)
	}
	report := res.Describe(m)
	for _, want := range []string{"Memory: 0.5 → 2", "Tile: 512 → 768", "✗ Tile: 1024: Tunable failed", "✗ Slow: On: stopped"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	t.Log("\n" + report)
}
