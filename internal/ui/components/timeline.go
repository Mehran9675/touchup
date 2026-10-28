package components

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/mehran9675/touchup/internal/ui/draw"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Timeline is a video scrubber over a filmstrip of thumbnails. Dragging
// moves the playhead; OnSeek gets the time while dragging (throttled)
// and once more, final, on release.
type Timeline struct {
	Duration float64
	Time     float64
	Film     []image.Image
	// Marker, when not negative, marks the time of the last preview.
	Marker float64
	OnSeek func(t float64, final bool)

	dragging bool
	hoverX   float32
	hovering bool
	lastEmit time.Time
}

// NewTimeline creates a timeline.
func NewTimeline(onSeek func(t float64, final bool)) *Timeline {
	return &Timeline{OnSeek: onSeek, Marker: -1}
}

// Layout draws the timeline across the available width.
func (t *Timeline) Layout(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(72))
	pad := gtx.Dp(8) // room for the playhead knob above and below
	strip := image.Rect(0, pad, size.X, size.Y-pad)
	t.events(gtx, size.X)

	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, t)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()

	radius := gtx.Dp(10)
	draw.RRect(gtx.Ops, strip, radius, draw.C(string(theme.Sunken())))
	// Filmstrip, clipped to the rounded strip.
	rr := clip.UniformRRect(strip, radius).Push(gtx.Ops)
	if n := len(t.Film); n > 0 {
		for i, img := range t.Film {
			cell := image.Rect(strip.Min.X+i*strip.Dx()/n, strip.Min.Y, strip.Min.X+(i+1)*strip.Dx()/n, strip.Max.Y)
			draw.Cover(gtx.Ops, img, cell)
		}
	}
	// Dim what's after the playhead slightly to show position at a glance.
	x := t.xFor(t.Time, size.X)
	draw.Rect(gtx.Ops, image.Rect(x, strip.Min.Y, strip.Max.X, strip.Max.Y), draw.C("#00000040"))
	if t.hovering && !t.dragging {
		hx := int(t.hoverX)
		draw.Rect(gtx.Ops, image.Rect(hx, strip.Min.Y, hx+gtx.Dp(1), strip.Max.Y), draw.C("#FFFFFF80"))
	}
	rr.Pop()
	// Border.
	paintBorder(gtx, strip, radius, draw.C(theme.Border))

	// Preview marker: a small diamond under the strip.
	if t.Marker >= 0 && t.Duration > 0 {
		mx := float32(t.xFor(t.Marker, size.X))
		my := float32(strip.Max.Y + pad/2)
		d := float32(gtx.Dp(4))
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(f32.Pt(mx, my-d))
		p.LineTo(f32.Pt(mx+d, my))
		p.LineTo(f32.Pt(mx, my+d))
		p.LineTo(f32.Pt(mx-d, my))
		p.Close()
		fillPath(gtx, p.End(), draw.C(theme.Pink))
	}

	// Playhead.
	white := draw.C("#FFFFFF")
	w := gtx.Dp(3)
	draw.RRect(gtx.Ops, image.Rect(x-w/2, gtx.Dp(2), x-w/2+w, size.Y-gtx.Dp(2)), w/2, white)
	draw.Circle(gtx.Ops, image.Pt(x, gtx.Dp(5)), gtx.Dp(5), white)
	draw.Circle(gtx.Ops, image.Pt(x, gtx.Dp(5)), gtx.Dp(3), draw.C(theme.Violet))
	return layout.Dimensions{Size: size}
}

func (t *Timeline) xFor(sec float64, width int) int {
	if t.Duration <= 0 {
		return 0
	}
	return int(float64(width) * max(0, min(1, sec/t.Duration)))
}

func (t *Timeline) timeAt(x float32, width int) float64 {
	if width == 0 {
		return 0
	}
	return max(0, min(1, float64(x)/float64(width))) * t.Duration
}

func (t *Timeline) events(gtx layout.Context, width int) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: t,
			Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter, pointer.Move:
			t.hovering, t.hoverX = true, e.Position.X
		case pointer.Leave:
			t.hovering = false
		case pointer.Press:
			t.dragging = true
			t.seek(t.timeAt(e.Position.X, width), false)
		case pointer.Drag:
			t.hoverX = e.Position.X
			t.seek(t.timeAt(e.Position.X, width), false)
		case pointer.Release:
			if t.dragging {
				t.dragging = false
				t.seek(t.timeAt(e.Position.X, width), true)
			}
		case pointer.Cancel:
			t.dragging = false
		}
	}
}

// seek moves the playhead and tells the owner, at most every 120ms
// while dragging.
func (t *Timeline) seek(sec float64, final bool) {
	t.Time = sec
	if t.OnSeek == nil {
		return
	}
	if final || time.Since(t.lastEmit) > 120*time.Millisecond {
		t.lastEmit = time.Now()
		t.OnSeek(sec, final)
	}
}
