package store

import (
	"maps"

	"github.com/mehran9675/touchup/internal/engine"
)

// EngineOptions returns the values of m's options: the user's, and the
// defaults for the rest.
func (s *Store) EngineOptions(m *engine.Manifest) map[string]string {
	if m == nil {
		return nil
	}
	return m.OptionValues(s.Settings.EngineOptions[m.ID])
}

// SetEngineOption saves a value for one of m's options. The default
// value isn't stored, so a changed default reaches users who never
// touched the option.
func (s *Store) SetEngineOption(m *engine.Manifest, id, value string) {
	o := m.OptionByID(id)
	if o == nil || !o.Valid(value) {
		return
	}
	values := maps.Clone(s.Settings.EngineOptions[m.ID])
	if values == nil {
		values = map[string]string{}
	}
	if value == o.Default {
		delete(values, id)
	} else {
		values[id] = value
	}
	s.setEngineOptions(m, values)
}

// ResetEngineOptions puts all of m's options back to their defaults.
func (s *Store) ResetEngineOptions(m *engine.Manifest) { s.setEngineOptions(m, nil) }

// EngineOptionsChanged reports whether any of m's options differs from
// its default.
func (s *Store) EngineOptionsChanged(m *engine.Manifest) bool {
	return len(s.Settings.EngineOptions[m.ID]) > 0
}

func (s *Store) setEngineOptions(m *engine.Manifest, values map[string]string) {
	all := maps.Clone(s.Settings.EngineOptions)
	if all == nil {
		all = map[string]map[string]string{}
	}
	if len(values) == 0 {
		delete(all, m.ID)
	} else {
		all[m.ID] = values
	}
	s.Settings.EngineOptions = all
	s.saveSettings()
}
