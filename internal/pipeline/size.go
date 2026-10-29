package pipeline

import (
	"math"
	"strconv"

	"github.com/mehran9675/touchup/internal/engine"
)

// Size is how an input of a given size gets upscaled: the scale the
// engine runs at and the size of the result.
type Size struct {
	Scale         int
	Width, Height int
	// Exact is true when the engine's output has to be resized to Width
	// and Height, because it upscales by a fixed factor that doesn't
	// land on a target resolution.
	Exact bool
}

// PlanSize works out the Size for a w×h input. Without a target the
// engine's scale decides the size. A target is the short side of the
// result in pixels, such as 2160 for 4K: the engine runs at the smallest
// scale that reaches it, and the result is resized to fit.
func PlanSize(m *engine.Manifest, model *engine.Model, scale, target, w, h int) Size {
	scale = max(1, scale)
	short := min(w, h)
	if target <= 0 || short <= 0 {
		return Size{Scale: scale, Width: w * scale, Height: h * scale}
	}
	f := float64(target) / float64(short)
	sz := Size{Scale: m.ScaleFor(model, f), Exact: true}
	// The short side is exactly the target; the long side keeps the
	// aspect ratio, rounded to an even number for video encoders.
	even := func(v float64) int { return max(2, int(math.Round(v/2))*2) }
	if w <= h {
		sz.Width, sz.Height = target, even(float64(h)*f)
	} else {
		sz.Width, sz.Height = even(float64(w)*f), target
	}
	return sz
}

// size plans a w×h input with o's engine, model, scale and target.
func (o Options) size(w, h int) Size {
	return PlanSize(o.Engine, o.Model, o.Scale, o.Target, w, h)
}

// sized sets a w×h input's sizes on j: the scale to run at, and the
// target size for engines that are told one.
func (o Options) sized(j engine.Job, w, h int) engine.Job {
	sz := o.size(w, h)
	j.InWidth, j.InHeight = w, h
	j.Scale = sz.Scale
	if sz.Exact {
		j.OutWidth, j.OutHeight = sz.Width, sz.Height
	}
	return j
}

// Targets are the resolutions offered besides scales, as short sides.
var Targets = []int{720, 1080, 1440, 2160}

// TargetLabel names a target resolution: "1080p", or "4K" for 2160.
func TargetLabel(t int) string {
	if t == 2160 {
		return "4K"
	}
	return strconv.Itoa(t) + "p"
}
