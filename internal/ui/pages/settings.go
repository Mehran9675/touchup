package pages

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/slider"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/mehran9675/giode/styles/properties"

	"github.com/mehran9675/touchup/internal/paths"
	"github.com/mehran9675/touchup/internal/settings"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

var (
	imageFormats = []string{"png", "jpg", "webp"}
	audioFormats = []string{"wav", "flac", "mp3"}
	videoCodecs  = []string{"h264", "h265", "prores"}
	gpus         = []int{-1, 0, 1, 2}
)

// CRF range offered by the quality slider; lower is better.
const crfBest, crfSmallest = 10, 28

// Settings is the preferences page.
type Settings struct {
	s      *store.Store
	scroll *scroll.Scroll

	mode, image, audio, codec, gpu *segmented.Control
	quality                        *slider.Slider
	keepWork, sysmem               *toggle.Switch
	output                         *input.Input
	tools                          map[string]*input.Input
	showData                       *giode.ButtonElement
}

// NewSettings creates the page.
func NewSettings(s *store.Store) *Settings {
	st := s.Settings
	p := &Settings{s: s, tools: map[string]*input.Input{}}
	update := s.UpdateSettings

	p.mode = theme.Segmented("Dark", "Light").OnChange(func(i int) {
		update(func(c *settings.Settings) { c.LightMode = i == 1 })
	})
	p.mode.SetSelected(boolIndex(st.LightMode))
	p.image = theme.Segmented("PNG", "JPEG", "WebP").OnChange(func(i int) {
		update(func(c *settings.Settings) { c.ImageFormat = imageFormats[i] })
	})
	p.image.SetSelected(max(0, slices.Index(imageFormats, st.ImageFormat)))
	p.audio = theme.Segmented("WAV", "FLAC", "MP3").OnChange(func(i int) {
		update(func(c *settings.Settings) { c.AudioFormat = audioFormats[i] })
	})
	p.audio.SetSelected(max(0, slices.Index(audioFormats, st.AudioFormat)))
	p.codec = theme.Segmented("H.264", "H.265", "ProRes").OnChange(func(i int) {
		update(func(c *settings.Settings) { c.VideoCodec = videoCodecs[i] })
	})
	p.codec.SetSelected(max(0, slices.Index(videoCodecs, st.VideoCodec)))
	p.gpu = theme.Segmented("Auto", "GPU 0", "GPU 1", "GPU 2").OnChange(func(i int) {
		update(func(c *settings.Settings) { c.GPU = gpus[i] })
	})
	p.gpu.SetSelected(max(0, slices.Index(gpus, st.GPU)))

	p.quality = giode.Input.Slider(giode.Styles{Color: theme.Primary, Background: theme.Raised(), BorderColor: "#FFFFFF"}).
		OnChange(func(v float32) {
			crf := crfSmallest - int(v*float32(crfSmallest-crfBest)+0.5)
			update(func(c *settings.Settings) { c.VideoQuality = crf })
		})
	p.quality.SetValue(float32(crfSmallest-st.VideoQuality) / float32(crfSmallest-crfBest))

	p.keepWork = giode.Input.Switch("").OnChange(func(on bool) {
		update(func(c *settings.Settings) { c.KeepWork = on })
	})
	p.keepWork.SetChecked(st.KeepWork)
	p.sysmem = giode.Input.Switch("").OnChange(func(on bool) {
		update(func(c *settings.Settings) { c.AllowSystemMemory = on })
	})
	p.sysmem.SetChecked(st.AllowSystemMemory)

	p.output = pathField("Next to the original file", st.OutputDir, func(v string) {
		update(func(c *settings.Settings) { c.OutputDir = v })
	})
	toolFields := map[string]func(*settings.Settings) *string{
		"ffmpeg":  func(c *settings.Settings) *string { return &c.FFmpeg },
		"ffprobe": func(c *settings.Settings) *string { return &c.FFprobe },
		"ffplay":  func(c *settings.Settings) *string { return &c.FFplay },
		"python":  func(c *settings.Settings) *string { return &c.Python },
	}
	for name, field := range toolFields {
		p.tools[name] = pathField("Found automatically", *field(&s.Settings), func(v string) {
			update(func(c *settings.Settings) { *field(c) = v })
		})
	}
	p.showData = theme.SecondaryButton("Show folder", "folder-open").OnClick(func() { s.Reveal(paths.Engines()) })
	p.scroll = giode.Scroll(p.content, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}})
	return p
}

// pathField is a text field that saves when the user presses Enter or
// leaves it.
func pathField(placeholder, value string, save func(string)) *input.Input {
	f := theme.Field(placeholder)
	f.SetText(value)
	commit := func() { save(strings.TrimSpace(f.Text())) }
	f.Submit(func(string) { commit() })
	f.OnBlur(commit)
	return f.Styles(giode.Styles{
		Background: theme.Raised(), Color: theme.Text, BorderColor: theme.Border, BorderWidth: 1,
		BorderRadius: 10, FontFamily: theme.Mono, FontSize: 12, Padding: giode.SymmetricInset(12, 9), Width: 340,
	})
}

// View lays out the page.
func (p *Settings) View() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Top: 28, Right: 32, Bottom: 16, Left: 32}},
			components.PageHeader("Settings", "Where results go, how they're encoded, and the tools TouchUp drives.")),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, p.scroll),
	)
}

func (p *Settings) content() giode.Element {
	s := p.s
	st := s.Settings
	seg := func(c *segmented.Control, width int) giode.Element {
		return giode.Box(giode.Styles{Width: properties.Width(width)}, c)
	}
	section := func(title string, rows ...giode.Element) giode.Element {
		children := []giode.Element{giode.Text(title, theme.Heading())}
		for i, r := range rows {
			if i > 0 {
				children = append(children, giode.Divider(giode.Styles{Color: theme.Faint()}))
			}
			children = append(children, r)
		}
		return components.Card(giode.Styles{Width: -1, Gap: 12}, children...)
	}

	tool := func(name, label, found, missing string) giode.Element {
		var badge giode.Element
		switch {
		case found != "":
			badge = components.StatusBadge("Found", theme.Success)
		case name == "python":
			badge = components.StatusBadge("Automatic", theme.Info)
		default:
			badge = components.StatusBadge("Missing", theme.Warning)
		}
		desc := missing
		if found != "" {
			desc = found
		}
		return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 16, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(0, 6)},
			giode.Stack(giode.Styles{Gap: 4, FlexGrow: 1},
				giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter}, giode.Text(label, theme.Label()), badge),
				giode.Text(desc, theme.Code().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
			),
			p.tools[name],
		)
	}

	return giode.Stack(giode.Styles{Width: -1, MaxWidth: 920, Gap: 18, Padding: giode.Inset{Right: 32, Bottom: 32, Left: 32}},
		section("Appearance",
			components.SettingRow("Theme", "TouchUp is designed dark-first; light suits bright rooms.", seg(p.mode, 220)),
		),
		section("Output",
			components.SettingRow("Save results to", "Leave empty to save next to each original.", p.output),
			components.SettingRow("Image format", "PNG keeps every detail; WebP and JPEG are smaller.", seg(p.image, 260)),
			components.SettingRow("Video codec", "H.265 is smaller; ProRes is for editing.", seg(p.codec, 260)),
			components.SettingRow("Video quality", fmt.Sprintf("CRF %d · %s", st.VideoQuality, qualityWord(st.VideoQuality)),
				giode.Box(giode.Styles{Width: 260}, p.quality)),
			components.SettingRow("Audio format", "WAV and FLAC are lossless; MP3 is encoded at 320 kbps.", seg(p.audio, 260)),
		),
		section("Hardware",
			components.SettingRow("GPU", "Which graphics card engines use, when there's more than one.", seg(p.gpu, 340)),
			components.SettingRow("Use system memory when the GPU is full",
				"On, a job that doesn't fit in GPU memory carries on in system memory, often ten times slower or worse. "+
					"Off, it stops with \"out of memory\" so you can pick smaller tiles or batches. Applies to Python engines.",
				p.sysmem),
		),
		section("Tools",
			tool("ffmpeg", "FFmpeg", s.Tools.FFmpeg, "Needed to read and write video and audio. Install it from ffmpeg.org."),
			tool("ffprobe", "FFprobe", s.Tools.FFprobe, "Comes with FFmpeg; reads file details."),
			tool("ffplay", "FFplay", s.Tools.FFplay, "Comes with most FFmpeg builds; plays audio previews."),
			tool("python", "Python", s.Python, s.PythonNote),
		),
		section("Storage",
			components.SettingRow("Engines folder", paths.Engines(), p.showData),
			components.SettingRow("Keep work files", "Keep extracted frames and clips after each job, for debugging.", p.keepWork),
		),
	)
}

func qualityWord(crf int) string {
	switch {
	case crf <= 14:
		return "near lossless, large files"
	case crf <= 18:
		return "visually lossless"
	case crf <= 23:
		return "balanced"
	}
	return "small files"
}

func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}
