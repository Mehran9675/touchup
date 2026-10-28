package store

import (
	"fmt"
	"slices"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/settings"
)

// Selection is the engine, model and scale chosen for a kind of media,
// resolved against what's installed.
type Selection struct {
	Engine *engine.Manifest // nil when no engine for the kind is installed
	Model  *engine.Model
	Scale  int
}

// Label describes the selection: "Real-ESRGAN · General · 4x".
func (sel Selection) Label() string {
	if sel.Engine == nil {
		return ""
	}
	l := sel.Engine.Name
	if sel.Model != nil {
		l += " · " + sel.Model.Name
	}
	if sel.Scale > 1 {
		l += fmt.Sprintf(" · %dx", sel.Scale)
	}
	return l
}

// EnginesFor lists the engines that handle kind, installed ones first.
func (s *Store) EnginesFor(kind media.Kind) []*engine.Manifest {
	list := s.Registry.For(kind.String())
	slices.SortStableFunc(list, func(a, b *engine.Manifest) int {
		ia, ib := s.Installed(a.ID), s.Installed(b.ID)
		switch {
		case ia && !ib:
			return -1
		case ib && !ia:
			return 1
		}
		return 0
	})
	return list
}

// Selected returns the selection for kind: the remembered one when its
// engine is still installed, otherwise the first installed engine.
func (s *Store) Selected(kind media.Kind) Selection {
	c := s.Settings.Choices[kind.String()]
	m := s.Registry.Get(c.Engine)
	if m == nil || !s.Installed(m.ID) || !m.Handles(kind.String()) {
		m = nil
		for _, e := range s.EnginesFor(kind) {
			if s.Installed(e.ID) {
				m = e
				break
			}
		}
		if m == nil {
			return Selection{}
		}
		c = settings.Choice{Engine: m.ID}
	}
	model := m.Model(c.Model)
	scale := c.Scale
	if scales := m.ScalesFor(model); !slices.Contains(scales, scale) {
		scale = m.PreferredScale(model)
	}
	if len(m.ScalesFor(model)) == 0 {
		scale = 1
	}
	return Selection{Engine: m, Model: model, Scale: scale}
}

// Choose remembers an engine, model and scale for kind. Empty values
// keep the current ones; a new engine resets the model and scale.
func (s *Store) Choose(kind media.Kind, engineID, modelID string, scale int) {
	cur := s.Selected(kind)
	c := settings.Choice{}
	if cur.Engine != nil {
		c.Engine = cur.Engine.ID
		if cur.Model != nil {
			c.Model = cur.Model.ID
		}
		c.Scale = cur.Scale
	}
	if engineID != "" && engineID != c.Engine {
		c = settings.Choice{Engine: engineID}
	}
	if modelID != "" {
		c.Model = modelID
	}
	if scale > 0 {
		c.Scale = scale
	}
	if s.Settings.Choices == nil {
		s.Settings.Choices = map[string]settings.Choice{}
	}
	s.Settings.Choices[kind.String()] = c
	s.saveSettings()
}

// options builds pipeline options for kind's selection.
func (s *Store) options(kind media.Kind) (pipeline.Options, Selection) {
	sel := s.Selected(kind)
	return pipeline.Options{
		Engine: sel.Engine, Model: sel.Model, Scale: sel.Scale,
		Settings: s.Settings, Tools: s.Tools,
	}, sel
}
