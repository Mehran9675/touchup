package store

import "time"

// Pace estimates how long a run has left from how fast its progress has
// moved since the engine first reported some. Progress before that (a
// stage's fixed starting point) says nothing about speed.
type Pace struct {
	at   time.Time
	frac float64
}

func (p *Pace) reset() { *p = Pace{} }

func (p *Pace) add(frac float64) {
	if p.at.IsZero() && frac > 0 {
		p.at, p.frac = time.Now(), frac
	}
}

// Left returns the time still needed at frac, once progress has moved
// since the first report and a minute has passed.
func (p *Pace) Left(now time.Time, frac float64) (time.Duration, bool) {
	done, took := frac-p.frac, now.Sub(p.at)
	if p.at.IsZero() || done <= 0 || took < time.Minute || frac >= 1 {
		return 0, false
	}
	return time.Duration(float64(took) * (1 - frac) / done).Round(time.Minute), true
}

// TimeLeft estimates how long the preview has left.
func (p Preview) TimeLeft(now time.Time) (time.Duration, bool) {
	return p.Pace.Left(now, p.Update.Frac)
}

// TimeLeft estimates how long the job has left.
func (j *Job) TimeLeft(now time.Time) (time.Duration, bool) { return j.Pace.Left(now, j.Update.Frac) }
