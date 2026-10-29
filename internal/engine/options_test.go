package engine

import (
	"slices"
	"testing"
)

func TestOptionArgs(t *testing.T) {
	m := &Manifest{
		ID: "opts", Name: "Opts", Media: []string{"image"}, Input: "image",
		Install: InstallSpec{Type: "none"}, Executable: map[string]string{"*": "x"},
		Args: []string{"-i", "{input}"}, OutputExt: ".png",
		Options: []Option{
			{ID: "tile", Name: "Tile", Type: OptionChoiceType, Default: "512",
				Choices: []OptionChoice{{"256", "256"}, {"512", "512"}}, Args: []string{"-t", "{value}"}},
			{ID: "tta", Name: "TTA", Type: OptionToggle, Default: "false", Args: []string{"-x"}},
			{ID: "gpuWork", Name: "GPU", Type: OptionToggle, Default: "true",
				Args: []string{"--work", "gpu"}, ArgsOff: []string{"--work", "cpu"}},
			{ID: "steps", Name: "Steps", Type: OptionNumber, Default: "50", Min: 1, Max: 200, Args: []string{"--steps={value}"}},
			{ID: OptionTileImage, Name: "Split", Type: OptionChoiceType, Default: "0",
				Choices: []OptionChoice{{"0", "Never"}, {"2048", "2048"}}},
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	values := m.OptionValues(map[string]string{
		"tile":    "1024", // not a choice: the default stays
		"tta":     "true",
		"gpuWork": "false",
		"steps":   "75",
	})
	got := m.Command(Job{Input: "in.png", Options: values})
	want := []string{"-i", "in.png", "-t", "512", "-x", "--work", "cpu", "--steps=75"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
	// Without values, defaults apply.
	got = m.Command(Job{Input: "in.png"})
	want = []string{"-i", "in.png", "-t", "512", "--work", "gpu", "--steps=50"}
	if !slices.Equal(got, want) {
		t.Errorf("defaults: got %q\nwant %q", got, want)
	}
	if m.OptionValues(map[string]string{"steps": "500"})["steps"] != "50" {
		t.Error("a number out of range was accepted")
	}

	bad := *m
	bad.Options = []Option{{ID: "a", Name: "A", Type: OptionChoiceType, Default: "x", Choices: []OptionChoice{{"y", "y"}}}}
	if bad.Validate() == nil {
		t.Error("an invalid default was accepted")
	}
}
