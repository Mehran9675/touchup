package components

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// outputLine shows the latest line an engine printed, or nothing.
func outputLine(line string) giode.Element {
	if line == "" {
		return giode.Box(giode.Styles{})
	}
	return giode.Text(line, theme.Code().With(giode.Styles{MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))
}
