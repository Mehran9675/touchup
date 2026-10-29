package store

import (
	"testing"
	"time"
)

func TestPace(t *testing.T) {
	var p Pace
	p.add(0) // nothing reported yet
	p.add(0.02)
	start := p.at
	if _, ok := p.Left(start.Add(30*time.Second), 0.1); ok {
		t.Error("guessed before a minute had passed")
	}
	if _, ok := p.Left(start.Add(5*time.Minute), 0.022); ok {
		t.Error("guessed from too little progress")
	}
	// 10% of the work in 10 minutes since the first report: the 88%
	// left takes 88 minutes.
	left, ok := p.Left(start.Add(10*time.Minute), 0.12)
	if !ok || left != 88*time.Minute {
		t.Errorf("got %v, %v; want 88m", left, ok)
	}
}
