package components

import (
	"gioui.org/layout"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"
)

// Grid lays cells out in as many equal columns as fit at least minWidth
// dp each. cell builds the i-th of n cells at the given width in dp.
func Grid(n, minWidth, gap int, cell func(i, width int) giode.Element) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		avail := int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
		cols := max(1, (avail+gap)/(minWidth+gap))
		width := (avail - gap*(cols-1)) / cols
		cells := make([]giode.Element, n)
		for i := range n {
			cells[i] = cell(i, width)
		}
		return giode.Row(giode.Styles{Width: -1, Gap: properties.Gap(gap)}, cells...).Layout(gtx)
	})
}
