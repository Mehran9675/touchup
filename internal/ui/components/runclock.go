package components

import (
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// quietAfter is how long an engine can go without printing before
// RunClock says so.
const quietAfter = 20 * time.Second

// RunClock says how long a run has gone on, and how long since the
// engine last printed anything once it has been quiet for a while, so a
// slow run can be told from a stuck one. It ticks every second.
func RunClock(started, lastOutput time.Time) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		text := "Running for " + format.Elapsed(gtx.Now.Sub(started))
		style := theme.Caption()
		if quiet := gtx.Now.Sub(lastOutput); !lastOutput.IsZero() && quiet >= quietAfter {
			text += " · no output for " + format.Elapsed(quiet)
			style = style.With(giode.Styles{Color: theme.Warning})
		}
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second)})
		return giode.Text(text, style).Layout(gtx)
	})
}
