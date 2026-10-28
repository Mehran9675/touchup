package engine

import (
	"slices"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	got := SplitArgs(`-i {input} -o "{output}"  --name 'two words' -x`)
	want := []string{"-i", "{input}", "-o", "{output}", "--name", "two words", "-x"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"My Upscaler 2":   "my-upscaler-2",
		"  SeedVR2 (7B) ": "seedvr2-7b",
		"ÄÖ":              "",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
