package engine

import (
	"math"
	"testing"
)

func TestProgressSteps(t *testing.T) {
	p := newProgress(&Manifest{ProgressSteps: []string{`Chunk (\d+)/(\d+)`, `Upscaling batch (\d+)/(\d+)`}})
	for _, c := range []struct {
		line string
		want float64
		ok   bool
	}{
		{"Loading model", 0, false},
		{"Chunk 1/4: 42 new + 0 context frames", 0, true},
		{"Upscaling batch 1/2", 0, true},
		{"Upscaling batch 2/2", 0.125, true},
		{"VRAM at 85%", 0, false}, // steps replace percentages
		{"Chunk 2/4: 42 new + 3 context frames", 0.25, true},
		{"Upscaling batch 2/2", 0.375, true},
	} {
		got, ok := p.line(c.line)
		if ok != c.ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%q: got %v, %v; want %v, %v", c.line, got, ok, c.want, c.ok)
		}
	}

	// Inner steps alone, as for a single image, fill the bar too.
	p = newProgress(&Manifest{ProgressSteps: []string{`Chunk (\d+)/(\d+)`, `Upscaling batch (\d+)/(\d+)`}})
	if got, _ := p.line("Upscaling batch 3/4"); got != 0.5 {
		t.Errorf("inner step alone: got %v, want 0.5", got)
	}
}

func TestProgressPercent(t *testing.T) {
	p := newProgress(&Manifest{})
	if got, ok := p.line("tile 3/9 37.5%"); !ok || got != 0.375 {
		t.Errorf("got %v, %v", got, ok)
	}
}
