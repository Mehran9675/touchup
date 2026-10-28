// Package engine describes, installs and runs upscaling engines.
//
// An engine is any command-line program that turns an input file (or a
// folder of them) into a better one. A JSON manifest says how to install
// it (download an archive, create a Python environment, or point at a
// program already on disk), how to call it, and which models it offers.
// TouchUp ships manifests for well-known open-source engines, and users
// add their own the same way.
package engine

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Manifest describes an engine.
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Vendor      string   `json:"vendor,omitempty"`
	Description string   `json:"description,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	// Requirements is shown to users before they install: "Vulkan GPU",
	// "Python 3.10, NVIDIA GPU recommended".
	Requirements string `json:"requirements,omitempty"`

	// Media lists what the engine can upscale: "image", "video", "audio".
	Media []string `json:"media"`
	// Input is what the command itself consumes: "image", "video" or
	// "audio". An image engine listed for video runs on extracted frames.
	Input string `json:"input"`
	// Batch means the command accepts folders for {input} and {output},
	// which makes frame-by-frame video much faster.
	Batch bool `json:"batch,omitempty"`

	Install InstallSpec `json:"install"`
	// Executable is the program to run, by GOOS ("linux", "windows",
	// "darwin") or "*". A relative name is looked up in the engine's
	// folder (or its Python environment), then on PATH.
	Executable map[string]string `json:"executable"`
	// Args are the command's arguments, with placeholders replaced:
	// {input}, {output}, {inputDir}, {outputDir}, {scale}, {gpu},
	// {engineDir}, {model}. An argument that is exactly "{modelArgs}"
	// expands to the selected model's arguments.
	Args []string `json:"args"`
	// GPUArgs are appended when the user picked a GPU.
	GPUArgs []string `json:"gpuArgs,omitempty"`
	// OutputExt is the extension the engine writes: ".png", ".wav".
	OutputExt string `json:"outputExt"`

	// Scales offered to the user; empty for engines with a fixed output
	// (most audio engines).
	Scales       []int   `json:"scales,omitempty"`
	DefaultScale int     `json:"defaultScale,omitempty"`
	Models       []Model `json:"models,omitempty"`

	// Progress is a regular expression whose first group is a
	// percentage in the engine's output. Defaults to "N%".
	Progress string `json:"progress,omitempty"`

	// Custom marks engines the user added; set by the registry.
	Custom bool `json:"-"`
}

// Model is one set of weights or one mode of an engine.
type Model struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Args        []string `json:"args,omitempty"`
	// Scales overrides the engine's scales for this model.
	Scales []int `json:"scales,omitempty"`
}

// InstallSpec says how to install an engine.
type InstallSpec struct {
	// Type is "archive" (download and unpack), "pip" (a Python virtual
	// environment with Packages) or "none" (Executable already exists).
	Type string `json:"type"`
	// URLs of archives by "goos/goarch", "goos" or "*".
	URLs map[string]string `json:"urls,omitempty"`
	// Packages are pip requirement specifiers.
	Packages []string `json:"packages,omitempty"`
	// Size is a display hint: "45 MB", "2.5 GB with models".
	Size string `json:"size,omitempty"`
}

// Handles reports whether the engine upscales kind ("image", "video",
// "audio").
func (m *Manifest) Handles(kind string) bool { return slices.Contains(m.Media, kind) }

// Model returns the model with id, or the first model, or nil when the
// engine has none.
func (m *Manifest) Model(id string) *Model {
	for i := range m.Models {
		if m.Models[i].ID == id {
			return &m.Models[i]
		}
	}
	if len(m.Models) > 0 {
		return &m.Models[0]
	}
	return nil
}

// ScalesFor returns the scales offered with model.
func (m *Manifest) ScalesFor(model *Model) []int {
	if model != nil && len(model.Scales) > 0 {
		return model.Scales
	}
	return m.Scales
}

// PreferredScale returns the default scale for model.
func (m *Manifest) PreferredScale(model *Model) int {
	scales := m.ScalesFor(model)
	if len(scales) == 0 {
		return 1
	}
	if slices.Contains(scales, m.DefaultScale) {
		return m.DefaultScale
	}
	return scales[len(scales)-1]
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

// Validate checks that a manifest is complete enough to install and run.
func (m *Manifest) Validate() error {
	var problems []string
	if !idPattern.MatchString(m.ID) {
		problems = append(problems, "id must be 2-64 lowercase letters, digits, dots, dashes or underscores")
	}
	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, "name is required")
	}
	if len(m.Media) == 0 {
		problems = append(problems, "media must list image, video or audio")
	}
	for _, k := range m.Media {
		if k != "image" && k != "video" && k != "audio" {
			problems = append(problems, fmt.Sprintf("unknown media %q", k))
		}
	}
	switch m.Input {
	case "image", "video", "audio":
	default:
		problems = append(problems, "input must be image, video or audio")
	}
	if m.Input == "image" && m.Handles("audio") || m.Input == "audio" && (m.Handles("image") || m.Handles("video")) {
		problems = append(problems, "input doesn't fit the media it handles")
	}
	switch m.Install.Type {
	case "archive":
		if len(m.Install.URLs) == 0 {
			problems = append(problems, "an archive install needs urls")
		}
	case "pip":
		if len(m.Install.Packages) == 0 {
			problems = append(problems, "a pip install needs packages")
		}
	case "none":
	default:
		problems = append(problems, "install.type must be archive, pip or none")
	}
	if len(m.Executable) == 0 {
		problems = append(problems, "executable is required")
	}
	if !slices.ContainsFunc(m.Args, func(a string) bool { return strings.Contains(a, "{input") }) {
		problems = append(problems, "args must use {input} or {inputDir}")
	}
	if !strings.HasPrefix(m.OutputExt, ".") {
		problems = append(problems, `outputExt must start with a dot, like ".png"`)
	}
	if m.Progress != "" {
		if _, err := regexp.Compile(m.Progress); err != nil {
			problems = append(problems, "progress is not a valid regular expression")
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
