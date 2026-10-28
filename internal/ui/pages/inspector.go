// Package pages holds TouchUp's screens and the larger panels they're
// made of. Each is created once and laid out every frame.
package pages

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Inspector is the studio's right-hand panel: engine, model and scale,
// what the output will be, and the preview and upscale actions.
type Inspector struct {
	s       *store.Store
	scroll  *scroll.Scroll
	preview *giode.ButtonElement
	run     *giode.ButtonElement
	browse  *giode.ButtonElement
	models  *giode.ButtonElement
}

// NewInspector creates the inspector.
func NewInspector(s *store.Store) *Inspector {
	in := &Inspector{s: s}
	wide := giode.Styles{Width: -1, Padding: giode.SymmetricInset(16, 12)}
	in.preview = giode.Button("Preview", theme.SecondaryStyle().With(wide)).OnClick(s.RunPreview).
		Icon(giode.Icon("scan-eye", giode.Styles{Width: 16, Height: 16}))
	in.run = giode.Button("Upscale", theme.PrimaryStyle().With(wide)).OnClick(s.Enqueue).
		Icon(giode.Icon("sparkles", giode.Styles{Width: 16, Height: 16}))
	in.browse = theme.SecondaryButton("Browse engines", "package").OnClick(func() { s.Navigate(store.PageEngines) })
	in.models = theme.SecondaryButton("Get models", "download").OnClick(func() { s.Navigate(store.PageEngines) })
	in.scroll = giode.Scroll(in.content, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}})
	return in
}

// View lays out the inspector.
func (in *Inspector) View() giode.Element {
	sess := in.s.Session
	sel := in.s.Selected(sess.Kind)
	canRun := sel.Ready() && !sess.Loading && sess.Err == ""

	verb := map[media.Kind]string{media.Image: "Upscale image", media.Video: "Upscale video", media.Audio: "Enhance audio"}[sess.Kind]
	in.run.Label(verb).Disabled(!canRun)
	actions := []giode.Element{}
	switch sess.Kind {
	case media.Video, media.Audio:
		label := "Preview frame"
		if sess.Kind == media.Audio {
			label = "Preview section"
		}
		in.preview.Label(label).Disabled(!canRun).Loading(sess.Preview.Running)
		actions = append(actions, in.preview)
	}
	actions = append(actions, in.run)

	return giode.Stack(giode.Styles{
		Width: 340, Height: -1,
		Background: theme.Surface, BorderColor: theme.Faint(), BorderWidth: 1, BorderRadius: 18,
	},
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, in.scroll),
		giode.Divider(giode.Styles{Color: theme.Faint()}),
		giode.Stack(giode.Styles{Width: -1, Gap: 10, Padding: giode.UniformInset(18)},
			giode.Stack(giode.Styles{Width: -1, Gap: 8}, actions...),
			giode.Text(in.footnote(), theme.Caption().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
		),
	)
}

func (in *Inspector) footnote() string {
	sess := in.s.Session
	switch sess.Kind {
	case media.Video:
		return "Preview upscales only the frame under the playhead."
	case media.Audio:
		return "Preview enhances only the highlighted section."
	}
	return "Images go straight to the queue: no preview needed."
}

func (in *Inspector) content() giode.Element {
	s := in.s
	sess := s.Session
	kind := sess.Kind
	sel := s.Selected(kind)
	engines := s.EnginesFor(kind)

	sections := []giode.Element{}

	// Engines.
	rows := []giode.Element{components.SectionLabel("Engine")}
	installedAny := false
	for _, m := range engines {
		installed := s.Installed(m.ID)
		installedAny = installedAny || installed
		selected := sel.Engine != nil && sel.Engine.ID == m.ID
		note := ""
		if installed && len(m.Models) > 0 && s.InstalledModels(m) == 0 {
			note = "No model installed yet"
		}
		rows = append(rows, components.EngineOption(m, selected, installed, note, func() {
			if installed {
				s.Choose(kind, m.ID, "", 0)
			} else {
				s.Navigate(store.PageEngines)
			}
		}))
	}
	if !installedAny {
		rows = append(rows, components.Banner("info", theme.Info, "No engine installed yet",
			"Install one from the Engines page to start upscaling "+kindNoun(kind)+"."), in.browse)
	}
	sections = append(sections, giode.Stack(giode.Styles{Gap: 8, Width: -1}, rows...))

	if sel.Engine != nil && !sel.Ready() {
		sections = append(sections, giode.Stack(giode.Styles{Gap: 8, Width: -1},
			components.Banner("download", theme.Info, "Choose a model to download",
				sel.Engine.Name+" is installed, but its models are separate downloads. Pick one on the Engines page."),
			in.models))
	}
	if sel.Ready() {
		// Models: the installed ones.
		var available []*engine.Model
		for i := range sel.Engine.Models {
			if mdl := &sel.Engine.Models[i]; s.ModelInstalled(sel.Engine, mdl) {
				available = append(available, mdl)
			}
		}
		if len(available) > 1 {
			pills := []giode.Element{}
			for _, mdl := range available {
				pills = append(pills, components.Pill("model:"+sel.Engine.ID, mdl.ID, mdl.Name,
					sel.Model != nil && sel.Model.ID == mdl.ID, func() { s.Choose(kind, "", mdl.ID, 0) }))
			}
			group := []giode.Element{components.SectionLabel("Model"), giode.Row(giode.Styles{Gap: 8}, pills...)}
			if sel.Model != nil && sel.Model.Description != "" {
				group = append(group, giode.Text(sel.Model.Description, theme.Caption()))
			}
			sections = append(sections, giode.Stack(giode.Styles{Gap: 10, Width: -1}, group...))
		}
		// Scales.
		if scales := sel.Engine.ScalesFor(sel.Model); len(scales) > 0 {
			pills := []giode.Element{}
			for _, sc := range scales {
				pills = append(pills, components.Pill("scale:"+sel.Engine.ID, sc, fmt.Sprintf("%dx", sc),
					sel.Scale == sc, func() { s.Choose(kind, "", "", sc) }))
			}
			sections = append(sections, giode.Stack(giode.Styles{Gap: 10, Width: -1},
				components.SectionLabel("Scale"), giode.Row(giode.Styles{Gap: 8}, pills...)))
		}
		sections = append(sections, in.output(sel))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 26, Padding: giode.UniformInset(18)}, sections...)
}

// output summarizes the file the upscale will write.
func (in *Inspector) output(sel store.Selection) giode.Element {
	s := in.s
	sess := s.Session
	info := sess.Info
	st := s.Settings
	rows := []giode.Element{components.SectionLabel("Output")}
	line := func(label, value string) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap, Gap: 12},
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(label, theme.Secondary())),
			giode.Text(value, theme.Label().With(giode.Styles{TextAlign: giode.TextAlignEnd})),
		)
	}
	switch sess.Kind {
	case media.Image, media.Video:
		if info.Width > 0 {
			rows = append(rows, line("Size", format.Resolution(info.Width*sel.Scale, info.Height*sel.Scale)))
		}
		if sess.Kind == media.Image {
			rows = append(rows, line("Format", strings.ToUpper(st.ImageFormat)))
		} else {
			codec := map[string]string{"h264": "H.264", "h265": "H.265", "prores": "ProRes 422 HQ"}[st.VideoCodec]
			rows = append(rows, line("Format", codec), line("Frames", fmt.Sprint(info.Frames)))
			if info.HasAudio {
				rows = append(rows, line("Audio", "Copied from the original"))
			}
		}
	case media.Audio:
		rows = append(rows, line("Format", strings.ToUpper(st.AudioFormat)))
		if info.SampleRate > 0 {
			rows = append(rows, line("Source", format.Rate(info.SampleRate)+" · "+format.Channels(info.Channels)))
		}
	}
	dest := "Next to the original"
	if st.OutputDir != "" {
		dest = filepath.Base(st.OutputDir)
	}
	rows = append(rows, line("Saved to", dest))
	return giode.Stack(giode.Styles{Gap: 10, Width: -1}, rows...)
}

func kindNoun(k media.Kind) string {
	switch k {
	case media.Image:
		return "images"
	case media.Video:
		return "videos"
	}
	return "audio"
}
