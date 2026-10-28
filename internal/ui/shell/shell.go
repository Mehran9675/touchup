// Package shell is TouchUp's window layout: the sidebar, the current
// page and the toasts above them.
package shell

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/titlebar"

	"github.com/mehran9675/touchup/internal/ui/pages"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Shell is the root of the UI.
type Shell struct {
	s        *store.Store
	studio   *pages.Studio
	engines  *pages.Engines
	queue    *pages.Queue
	settings *pages.Settings
	bar      *titlebar.TitleBar // nil with the system frame
}

// New builds every page once, and the title bar when the window is
// frameless.
func New(s *store.Store, dragging func() bool) *Shell {
	sh := &Shell{
		s:        s,
		studio:   pages.NewStudio(s, dragging),
		engines:  pages.NewEngines(s),
		queue:    pages.NewQueue(s),
		settings: pages.NewSettings(s),
	}
	if CustomFrame() {
		sh.bar = newTitleBar(s)
	}
	return sh
}

// View lays out the window.
func (sh *Shell) View() giode.Element {
	var page giode.Element
	switch sh.s.Page {
	case store.PageEngines:
		page = sh.engines.View()
	case store.PageQueue:
		page = sh.queue.View()
	case store.PageSettings:
		page = sh.settings.View()
	default:
		page = sh.studio.View()
	}
	var bar giode.Element = giode.Box(giode.Styles{Display: giode.DisplayNone})
	if sh.bar != nil {
		bar = sh.bar.Title(title(sh.s))
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: theme.Background},
		bar,
		giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap},
			Sidebar(sh.s),
			giode.Box(giode.Styles{Width: 1, Height: -1, Background: theme.Faint()}),
			giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, page),
		),
		// Overlays last, so they paint above everything.
		sh.engines.Overlay(),
		sh.s.Toasts,
	)
}
