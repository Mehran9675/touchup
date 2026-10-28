package pages

import (
	"context"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Engines is the engine library: install, remove and add engines.
type Engines struct {
	s      *store.Store
	filter *segmented.Control
	scroll *scroll.Scroll
	add    *giode.ButtonElement
	imp    *giode.ButtonElement
	form   *components.CustomEngineForm
	formOn bool
	dialog *dialog.Dialog
}

// NewEngines creates the page.
func NewEngines(s *store.Store) *Engines {
	e := &Engines{s: s, filter: theme.Segmented("All", "Image & video", "Audio", "Installed")}
	e.scroll = giode.Scroll(e.grid, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}})
	e.form = components.NewCustomEngineForm()
	e.form.OnSave = func(m *engine.Manifest) error {
		if err := s.AddCustomEngine(m); err != nil {
			return err
		}
		e.formOn = false
		return nil
	}
	e.form.OnCancel = func() { e.formOn = false }
	e.dialog = giode.Dialog(&e.formOn, func() giode.Element { return e.form }, giode.Styles{
		Background: theme.Surface, BorderRadius: 20, Padding: giode.UniformInset(26),
	})
	e.add = theme.PrimaryButton("Add custom engine", "plus").OnClick(func() {
		e.form.Reset()
		e.formOn = true
	})
	e.imp = theme.SecondaryButton("Import manifest", "file-code").OnClick(s.ImportManifest)
	return e
}

// View lays out the page.
func (e *Engines) View() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Stack(giode.Styles{Width: -1, Gap: 18, Padding: giode.Inset{Top: 28, Right: 32, Bottom: 8, Left: 32}},
			components.PageHeader("Engines",
				"Open-source models that run on this computer. Install what you need, or bring your own.",
				e.imp, e.add),
			giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{Width: 540}, e.filter),
			),
		),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, e.scroll),
	)
}

// Overlay is the custom engine dialog, laid out above the whole window.
func (e *Engines) Overlay() giode.Element { return e.dialog }

func (e *Engines) grid() giode.Element {
	s := e.s
	var shown []*engine.Manifest
	for _, m := range s.Registry.All() {
		installed := s.Installed(m.ID)
		switch e.filter.Selected() {
		case 1:
			if !m.Handles("image") && !m.Handles("video") {
				continue
			}
		case 2:
			if !m.Handles("audio") {
				continue
			}
		case 3:
			if !installed {
				continue
			}
		}
		shown = append(shown, m)
	}
	if len(shown) == 0 {
		return giode.Box(giode.Styles{Width: -1, Height: 420},
			components.EmptyState("package", "No engines installed",
				"Install one of the built-in engines or add your own to get started."))
	}
	return giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Top: 12, Right: 32, Bottom: 32, Left: 32}},
		components.Grid(len(shown), 320, 18, func(i, width int) giode.Element {
			return e.card(shown[i], s.Installed(shown[i].ID), width)
		}))
}

func (e *Engines) card(m *engine.Manifest, installed bool, width int) giode.Element {
	s := e.s
	in := s.Installs[m.ID]
	p := components.EngineCardProps{Engine: m, Installed: installed, Width: width}
	var actions []giode.Element
	homepage := func() giode.Element {
		return components.Action("engine-home", m.ID, func() *giode.ButtonElement {
			return theme.GhostButton("Website", "external-link")
		}, func() { go s.App.OpenURL(context.Background(), m.Homepage) })
	}
	spacer := giode.Box(giode.Styles{FlexGrow: 1})
	switch {
	case in.Running():
		p.Installing, p.Stage, p.Frac, p.Line = true, in.Status.Stage, in.Status.Frac, in.Status.Line
		actions = append(actions, spacer, components.Action("engine-cancel", m.ID, func() *giode.ButtonElement {
			return theme.GhostButton("Cancel", "x")
		}, func() { s.CancelInstall(m.ID) }))
	case in != nil && in.Err != "":
		p.Err = in.Err
		actions = append(actions,
			components.Action("engine-retry", m.ID, func() *giode.ButtonElement {
				return theme.SecondaryButton("Try again", "refresh-cw")
			}, func() { s.InstallEngine(m) }),
			spacer,
			components.Action("engine-dismiss", m.ID, func() *giode.ButtonElement {
				return theme.GhostButton("Dismiss", "")
			}, func() { s.DismissInstallError(m.ID) }))
	case installed:
		label := "Uninstall"
		if m.Install.Type == "none" {
			label = "Remove"
		}
		if m.Homepage != "" {
			actions = append(actions, homepage())
		}
		actions = append(actions, spacer)
		if m.Install.Type != "none" {
			actions = append(actions, components.Action("engine-uninstall", m.ID, func() *giode.ButtonElement {
				return theme.GhostButton(label, "trash")
			}, func() { s.UninstallEngine(m) }))
		}
		if m.Custom {
			actions = append(actions, components.Action("engine-remove", m.ID, func() *giode.ButtonElement {
				return theme.DangerButton("Delete", "trash")
			}, func() { s.RemoveCustomEngine(m) }))
		}
	case !m.Supported():
		actions = append(actions, giode.Text("Not available for this system", theme.Caption()))
	default:
		verb := "Install"
		if m.Install.Type == "none" {
			verb = "Check again"
		}
		actions = append(actions, components.Action("engine-install", m.ID, func() *giode.ButtonElement {
			return theme.PrimaryButton(verb, "download")
		}, func() { s.InstallEngine(m) }))
		if m.Homepage != "" {
			actions = append(actions, homepage())
		}
		actions = append(actions, spacer)
		if m.Custom {
			actions = append(actions, components.Action("engine-remove", m.ID, func() *giode.ButtonElement {
				return theme.DangerButton("Delete", "trash")
			}, func() { s.RemoveCustomEngine(m) }))
		}
	}
	p.Actions = actions
	return components.EngineCard(p)
}
