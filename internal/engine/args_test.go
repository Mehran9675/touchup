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

func TestSummarize(t *testing.T) {
	traceback := []string{
		"Traceback (most recent call last):",
		`  File "C:\\x\\module.py", line 1790, in _call_impl`,
		"    return forward_call(*args, **kwargs)",
		"           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^",
		"RuntimeError: CUDA error: no kernel image is available",
	}
	if got := summarize(traceback); got != "RuntimeError: CUDA error: no kernel image is available" {
		t.Errorf("traceback: %q", got)
	}
	withError := append(traceback, "Error: The GPU ran out of memory.")
	if got := summarize(withError); got != "The GPU ran out of memory." {
		t.Errorf("runner error line: %q", got)
	}
}
