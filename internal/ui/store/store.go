// Package store holds TouchUp's application state and the actions that
// change it. Everything here runs on the UI goroutine; background work
// reports back through App.Post.
package store

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/settings"
)

// Page is a top-level screen.
type Page int

const (
	PageStudio Page = iota
	PageEngines
	PageQueue
	PageSettings
)

// Store is the application state.
type Store struct {
	App    *giode.App
	Toasts *giode.ToasterElement

	Registry *engine.Registry
	Settings settings.Settings
	Tools    media.Tools
	Python   string // the interpreter found for Python engines

	Page    Page
	Session *Session
	Jobs    []*Job

	// Installs tracks engines being installed, by ID.
	Installs map[string]*Install

	Player  *media.Player
	Playing string // the file playing, or ""

	installed map[string]bool
	nextID    int
}

// New loads the settings and engines.
func New(toasts *giode.ToasterElement) *Store {
	reg, errs := engine.Load()
	s := &Store{
		Toasts:    toasts,
		Registry:  reg,
		Settings:  settings.Load(),
		Installs:  map[string]*Install{},
		Player:    &media.Player{},
		installed: map[string]bool{},
	}
	s.locateTools()
	s.refreshInstalled()
	for _, err := range errs {
		toasts.Show(giode.ToastWarning, "Skipped an engine manifest: "+err.Error())
	}
	return s
}

// Attach connects the store to the running app, for posting results
// back from background work.
func (s *Store) Attach(app *giode.App) {
	s.App = app
	s.Player.OnChange = func(playing string) {
		app.Post(func() { s.Playing = playing })
	}
}

// Navigate switches pages.
func (s *Store) Navigate(p Page) { s.Page = p }

// Installed reports whether engine id is ready to run.
func (s *Store) Installed(id string) bool { return s.installed[id] }

// refreshInstalled checks which engines are installed. It touches the
// disk, so it runs when something changed rather than every frame.
func (s *Store) refreshInstalled() {
	for _, m := range s.Registry.All() {
		s.installed[m.ID] = m.Installed()
	}
}

func (s *Store) locateTools() {
	st := s.Settings
	s.Tools = media.Locate(st.FFmpeg, st.FFprobe, st.FFplay)
	s.Python = engine.FindPython(st.Python)
}

func (s *Store) id() int {
	s.nextID++
	return s.nextID
}

// post runs fn on the UI goroutine.
func (s *Store) post(fn func()) { s.App.Post(fn) }

// fail shows an error toast; safe from any goroutine.
func (s *Store) fail(err error) {
	if err != nil {
		s.Toasts.Show(giode.ToastError, err.Error())
	}
}
