package components

import (
	"fmt"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// EngineCardProps is what an EngineCard shows.
type EngineCardProps struct {
	Engine    *engine.Manifest
	Width     int // dp
	Installed bool
	// Installing is true while an install runs; Stage, Frac and Line
	// describe it. Err is a failed install's message.
	Installing bool
	Stage      string
	Frac       float64
	Line       string
	Err        string
	// Actions are the card's buttons, built by the page.
	Actions []giode.Element
}

// EngineCard presents an engine in the library.
func EngineCard(p EngineCardProps) giode.Element {
	m := p.Engine
	icon, color := MediaIcon(media.Image)
	if m.Handles("audio") {
		icon, color = MediaIcon(media.Audio)
	} else if m.Input == "video" {
		icon, color = MediaIcon(media.Video)
	}

	var badge giode.Element = giode.Box(giode.Styles{})
	switch {
	case p.Installed:
		badge = StatusBadge("Installed", theme.Success)
	case m.Custom:
		badge = StatusBadge("Custom", theme.Info)
	}

	tags := []giode.Element{}
	for _, k := range m.Media {
		tags = append(tags, Tag(strings.ToUpper(k[:1])+k[1:], color))
	}
	for _, t := range m.Tags {
		tags = append(tags, Tag(t, theme.Muted))
	}

	facts := []giode.Element{}
	if m.Requirements != "" {
		facts = append(facts, fact("cpu", m.Requirements))
	}
	detail := []string{}
	if m.Install.Size != "" {
		detail = append(detail, m.Install.Size)
	}
	if n := len(m.Models); n > 0 {
		detail = append(detail, fmt.Sprintf("%d model%s", n, plural(n)))
	}
	if len(m.Scales) > 0 {
		s := make([]string, len(m.Scales))
		for i, v := range m.Scales {
			s[i] = fmt.Sprintf("%dx", v)
		}
		detail = append(detail, strings.Join(s, " · "))
	}
	if m.License != "" {
		detail = append(detail, m.License)
	}
	if len(detail) > 0 {
		facts = append(facts, fact("package", strings.Join(detail, "  ·  ")))
	}

	children := []giode.Element{
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			IconBubble(icon, color, 40),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(m.Name, theme.Heading()),
				giode.Text(m.Vendor, theme.Caption().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
			),
			badge,
		),
		giode.Box(giode.Styles{MinHeight: 57},
			giode.Text(m.Description, theme.Secondary().With(giode.Styles{MaxLines: 3, TextOverflow: giode.TextOverflowEllipsis}))),
		giode.Row(giode.Styles{Gap: 6}, tags...),
		giode.Stack(giode.Styles{Gap: 6}, facts...),
		giode.Divider(giode.Styles{Color: theme.Faint()}),
	}

	switch {
	case p.Installing:
		children = append(children, giode.Stack(giode.Styles{Gap: 8, Width: -1},
			giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(p.Stage+"…", theme.Label())),
				giode.Text(percent(p.Frac), theme.Caption()),
			),
			ProgressBar(p.Frac, 6),
			outputLine(p.Line),
		))
	case p.Err != "":
		children = append(children, giode.Text(p.Err, theme.Code().With(giode.Styles{Color: theme.Danger, MaxLines: 5, TextOverflow: giode.TextOverflowEllipsis})))
	}
	if len(p.Actions) > 0 {
		children = append(children, giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, p.Actions...))
	}
	return Card(giode.Styles{Width: properties.Width(p.Width), Gap: 14}, children...)
}

func fact(icon, text string) giode.Element {
	return giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		theme.Icon(icon, 14, theme.Muted),
		giode.Text(text, theme.Caption().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
	)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func percent(f float64) string {
	if f < 0 {
		return ""
	}
	return fmt.Sprintf("%d%%", int(f*100))
}
