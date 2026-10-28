// Package shell is TouchUp's window layout: the sidebar, the current
// page and the toasts above them.
package shell

import (
	"github.com/mehran9675/giode"

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
}

// New builds every page once.
func New(s *store.Store, dragging func() bool) *Shell {
	return &Shell{
		s:        s,
		studio:   pages.NewStudio(s, dragging),
		engines:  pages.NewEngines(s),
		queue:    pages.NewQueue(s),
		settings: pages.NewSettings(s),
	}
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
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: theme.Background},
		giode.Row(giode.Styles{Width: -1, Height: -1, Wrap: giode.WrapNoWrap},
			Sidebar(sh.s),
			giode.Box(giode.Styles{Width: 1, Height: -1, Background: theme.Faint()}),
			giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, page),
		),
		// Overlays last, so they paint above everything.
		sh.engines.Overlay(),
		sh.s.Toasts,
	)
}
