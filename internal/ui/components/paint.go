package components

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// Painting helpers shared by the custom-drawn components.

// paintBorder strokes a rounded rectangle's outline 1dp wide.
func paintBorder(gtx layout.Context, r image.Rectangle, radius int, c color.NRGBA) {
	w := float32(max(1, gtx.Dp(1)))
	rr := clip.UniformRRect(r, radius)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: w}.Op())
}

// fillPath fills a closed path.
func fillPath(gtx layout.Context, spec clip.PathSpec, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: spec}.Op())
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
