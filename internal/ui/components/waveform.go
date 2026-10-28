package components

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/mehran9675/touchup/internal/ui/draw"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Waveform draws an audio file's levels with a selected section. Drag
// inside the section to move it, drag its edges to resize it, or press
// elsewhere to center it there.
type Waveform struct {
	Peaks    []float32
	Duration float64
	Start    float64
	Length   float64
	// MaxLength caps resizing, in seconds.
	MaxLength float64
	// Playhead, when not negative, draws a line at that time.
	Playhead float64
	OnChange func(start, length float64)

	mode    int // 0 none, 1 move, 2 left edge, 3 right edge
	grabOff float64
}

const (
	waveNone = iota
	waveMove
	waveLeft
	waveRight
)

// NewWaveform creates a waveform.
func NewWaveform(onChange func(start, length float64)) *Waveform {
	return &Waveform{OnChange: onChange, MaxLength: 30, Playhead: -1}
}

// Layout draws the waveform across the available width.
func (w *Waveform) Layout(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(168))
	w.events(gtx, size.X)

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, w)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()

	radius := gtx.Dp(14)
	draw.RRect(gtx.Ops, image.Rect(0, 0, size.X, size.Y), radius, draw.C(string(theme.Sunken())))

	sx0, sx1 := w.x(w.Start, size.X), w.x(w.Start+w.Length, size.X)
	sel := image.Rect(sx0, 0, sx1, size.Y)
	// Section fill.
	draw.Rect(gtx.Ops, sel, draw.WithAlpha(draw.C(theme.Violet), 0.07))

	// Bars mirrored around the middle.
	bar, gap := max(1, gtx.Dp(2)), max(1, gtx.Dp(1.5))
	step := bar + gap
	mid := size.Y / 2
	maxH := float32(size.Y/2 - gtx.Dp(14))
	muted := draw.WithAlpha(draw.C(theme.Muted), 0.55)
	violet, pink := draw.C(theme.Violet), draw.C(theme.Pink)
	for x := gtx.Dp(6); x+bar < size.X-gtx.Dp(6); x += step {
		v := w.peakAt(float64(x) / float64(size.X))
		h := max(gtx.Dp(1), int(maxH*v))
		c := muted
		if x >= sx0 && x < sx1 && sx1 > sx0 {
			c = draw.Lerp(violet, pink, float32(x-sx0)/float32(sx1-sx0))
		}
		draw.RRect(gtx.Ops, image.Rect(x, mid-h, x+bar, mid+h), bar/2, c)
	}

	// Section edges with grips.
	white := draw.C("#FFFFFF")
	ew := gtx.Dp(3)
	for _, ex := range []int{sx0, sx1} {
		draw.RRect(gtx.Ops, image.Rect(ex-ew/2, gtx.Dp(6), ex-ew/2+ew, size.Y-gtx.Dp(6)), ew/2, white)
		grip := image.Rect(ex-gtx.Dp(5), mid-gtx.Dp(14), ex+gtx.Dp(5), mid+gtx.Dp(14))
		draw.RRect(gtx.Ops, grip, gtx.Dp(4), white)
		draw.Rect(gtx.Ops, image.Rect(ex-gtx.Dp(1), mid-gtx.Dp(7), ex+gtx.Dp(1), mid+gtx.Dp(7)), draw.C(theme.Violet))
	}

	if w.Playhead >= 0 {
		px := w.x(w.Playhead, size.X)
		draw.Rect(gtx.Ops, image.Rect(px, 0, px+max(1, gtx.Dp(1.5)), size.Y), white)
	}
	paintBorder(gtx, image.Rect(0, 0, size.X, size.Y), radius, draw.C(theme.Border))
	return layout.Dimensions{Size: size}
}

func (w *Waveform) peakAt(frac float64) float32 {
	if len(w.Peaks) == 0 {
		return 0
	}
	i := int(frac * float64(len(w.Peaks)))
	return w.Peaks[max(0, min(len(w.Peaks)-1, i))]
}

func (w *Waveform) x(sec float64, width int) int {
	if w.Duration <= 0 {
		return 0
	}
	return int(float64(width) * max(0, min(1, sec/w.Duration)))
}

func (w *Waveform) sec(x float32, width int) float64 {
	if width == 0 {
		return 0
	}
	return max(0, min(1, float64(x)/float64(width))) * w.Duration
}

func (w *Waveform) events(gtx layout.Context, width int) {
	edge := float32(gtx.Dp(10))
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: w, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok || w.Duration <= 0 {
			continue
		}
		at := w.sec(e.Position.X, width)
		x0, x1 := float32(w.x(w.Start, width)), float32(w.x(w.Start+w.Length, width))
		switch e.Kind {
		case pointer.Press:
			switch {
			case abs32(e.Position.X-x0) < edge:
				w.mode = waveLeft
			case abs32(e.Position.X-x1) < edge:
				w.mode = waveRight
			case e.Position.X > x0 && e.Position.X < x1:
				w.mode, w.grabOff = waveMove, at-w.Start
			default:
				w.mode, w.grabOff = waveMove, w.Length/2
				w.update(at-w.grabOff, w.Length)
			}
		case pointer.Drag:
			switch w.mode {
			case waveMove:
				w.update(at-w.grabOff, w.Length)
			case waveLeft:
				end := w.Start + w.Length
				start := max(end-w.MaxLength, min(at, end-0.5))
				w.update(start, end-start)
			case waveRight:
				length := max(0.5, min(at-w.Start, w.MaxLength))
				w.update(w.Start, length)
			}
		case pointer.Release, pointer.Cancel:
			w.mode = waveNone
		}
	}
}

func (w *Waveform) update(start, length float64) {
	length = max(0.5, min(length, w.Duration))
	start = max(0, min(start, w.Duration-length))
	w.Start, w.Length = start, length
	if w.OnChange != nil {
		w.OnChange(start, length)
	}
}
