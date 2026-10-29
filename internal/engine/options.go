package engine

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Option is an engine setting the user can change, such as a tile size
// or a batch size. Its value becomes arguments: Args with {value}
// replaced, or for a toggle Args when on and ArgsOff when off. Options
// whose ID starts with "touchup." aren't passed to the engine; TouchUp
// reads them itself.
type Option struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        string         `json:"type"` // "choice", "number" or "toggle"
	Default     string         `json:"default"`
	Choices     []OptionChoice `json:"choices,omitempty"`
	// Min and Max bound a number, when Max is above Min.
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Args    []string `json:"args,omitempty"`
	ArgsOff []string `json:"argsOff,omitempty"`
}

// OptionChoice is one value of a choice option.
type OptionChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Option types.
const (
	OptionChoiceType = "choice"
	OptionNumber     = "number"
	OptionToggle     = "toggle"
)

// OptionTileImage is TouchUp's own option for engines that can't split
// pictures themselves: the largest output, in pixels a side, made in one
// run. Bigger images are upscaled in overlapping tiles and joined.
const OptionTileImage = "touchup.tileImage"

// Valid reports whether v is a value the option takes.
func (o *Option) Valid(v string) bool {
	switch o.Type {
	case OptionToggle:
		return v == "true" || v == "false"
	case OptionChoiceType:
		return slices.ContainsFunc(o.Choices, func(c OptionChoice) bool { return c.Value == v })
	case OptionNumber:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
		return o.Max <= o.Min || (f >= o.Min && f <= o.Max)
	}
	return false
}

// Label is how a value reads: a choice's label, "On" or "Off", or the
// number.
func (o *Option) Label(v string) string {
	switch o.Type {
	case OptionToggle:
		if v == "true" {
			return "On"
		}
		return "Off"
	case OptionChoiceType:
		for _, c := range o.Choices {
			if c.Value == v {
				return c.Label
			}
		}
	}
	return v
}

// OptionByID returns the option with id, or nil.
func (m *Manifest) OptionByID(id string) *Option {
	for i := range m.Options {
		if m.Options[i].ID == id {
			return &m.Options[i]
		}
	}
	return nil
}

// OptionValues returns every option's value: the user's where it's
// valid, otherwise the default.
func (m *Manifest) OptionValues(user map[string]string) map[string]string {
	values := map[string]string{}
	for i := range m.Options {
		o := &m.Options[i]
		values[o.ID] = o.Default
		if v, ok := user[o.ID]; ok && o.Valid(v) {
			values[o.ID] = v
		}
	}
	return values
}

// optionArgs returns the arguments the options' values make, in the
// options' order, with placeholders filled in by expand.
func (m *Manifest) optionArgs(values map[string]string, expand func(string) string) []string {
	var out []string
	for i := range m.Options {
		o := &m.Options[i]
		if strings.HasPrefix(o.ID, "touchup.") {
			continue
		}
		v, ok := values[o.ID]
		if !ok || !o.Valid(v) {
			v = o.Default
		}
		args := o.Args
		if o.Type == OptionToggle && v != "true" {
			args = o.ArgsOff
		}
		for _, a := range args {
			out = append(out, expand(strings.ReplaceAll(a, "{value}", v)))
		}
	}
	return out
}

// validateOptions lists what's wrong with the manifest's options.
func (m *Manifest) validateOptions() []string {
	var problems []string
	seen := map[string]bool{}
	for i := range m.Options {
		o := &m.Options[i]
		switch {
		case o.ID == "" || seen[o.ID]:
			problems = append(problems, fmt.Sprintf("option %q needs a unique id", o.Name))
		case o.Type != OptionChoiceType && o.Type != OptionNumber && o.Type != OptionToggle:
			problems = append(problems, fmt.Sprintf("option %q has an unknown type %q", o.ID, o.Type))
		case o.Type == OptionChoiceType && len(o.Choices) == 0:
			problems = append(problems, fmt.Sprintf("option %q has no choices", o.ID))
		case !o.Valid(o.Default):
			problems = append(problems, fmt.Sprintf("option %q has an invalid default %q", o.ID, o.Default))
		}
		seen[o.ID] = true
	}
	return problems
}

// Tuning is what TouchUp tries when finding an engine's fastest
// settings: a clip of Seconds from the user's video, upscaled with each
// step's values in turn.
type Tuning struct {
	Seconds float64    `json:"seconds"`
	Steps   []TuneStep `json:"steps"`
}

// TuneStep is an option and the values to try for it, from the
// cheapest to the most demanding: the tuner keeps going while each is
// faster than the last and stops at the first that's slower or fails.
type TuneStep struct {
	Option string   `json:"option"`
	Values []string `json:"values"`
}

func (m *Manifest) validateTuning() []string {
	if m.Tune == nil {
		return nil
	}
	var problems []string
	if m.Tune.Seconds <= 0 {
		problems = append(problems, "tune needs a clip length in seconds")
	}
	for _, st := range m.Tune.Steps {
		o := m.OptionByID(st.Option)
		if o == nil {
			problems = append(problems, fmt.Sprintf("tune step names an unknown option %q", st.Option))
			continue
		}
		for _, v := range st.Values {
			if !o.Valid(v) {
				problems = append(problems, fmt.Sprintf("tune step for %q has an invalid value %q", st.Option, v))
			}
		}
	}
	return problems
}
