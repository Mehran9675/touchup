package engine

import (
	"regexp"
	"strconv"
)

// progress reads an engine's progress from its output lines.
type progress struct {
	percent *regexp.Regexp
	// steps are nested counters, outermost first, each matching "n" and
	// "total" in its two groups: say chunks of a video, then batches of
	// a chunk. A line for a step means its nth part has started.
	steps       []*regexp.Regexp
	done, total []float64
}

func newProgress(m *Manifest) *progress {
	p := &progress{percent: defaultProgress}
	if m.Progress != "" {
		if re, err := regexp.Compile(m.Progress); err == nil {
			p.percent = re
		}
	}
	for _, s := range m.ProgressSteps {
		if re, err := regexp.Compile(s); err == nil && re.NumSubexp() >= 2 {
			p.steps = append(p.steps, re)
		}
	}
	p.done = make([]float64, len(p.steps))
	p.total = make([]float64, len(p.steps))
	return p
}

// line returns the progress a line reports, in [0, 1].
func (p *progress) line(s string) (float64, bool) {
	if len(p.steps) == 0 {
		mm := p.percent.FindStringSubmatch(s)
		if len(mm) < 2 {
			return 0, false
		}
		v, err := strconv.ParseFloat(mm[1], 64)
		if err != nil {
			return 0, false
		}
		return min(1, max(0, v/100)), true
	}
	for i, re := range p.steps {
		mm := re.FindStringSubmatch(s)
		if len(mm) < 3 {
			continue
		}
		n, err1 := strconv.Atoi(mm[1])
		total, err2 := strconv.Atoi(mm[2])
		if err1 != nil || err2 != nil || total <= 0 {
			return 0, false
		}
		p.done[i], p.total[i] = float64(max(0, n-1)), float64(total)
		for j := i + 1; j < len(p.steps); j++ {
			p.done[j], p.total[j] = 0, 0
		}
		return p.frac(), true
	}
	return 0, false
}

// frac adds up the steps: the parts of each level that are done, each
// worth its share of the level above. A level not seen yet counts as one
// part, so an engine that prints only inner steps still fills the bar.
func (p *progress) frac() float64 {
	frac, share := 0.0, 1.0
	for i := range p.steps {
		if p.total[i] == 0 {
			continue
		}
		frac += share * p.done[i] / p.total[i]
		share /= p.total[i]
	}
	return min(1, frac)
}
