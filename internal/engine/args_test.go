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

	// SeedVR2 flags its error, raises, then signs off with links.
	seedvr2 := []string{
		"[15:58:39.784] 🎨   Encoding tiles 1-4 / 4",
		"[15:58:42.086] ❌ [ERROR] Error in Phase 1 (Encoding): Allocation on device 0 would exceed allowed memory. (out of memory)",
		"Currently allocated     : 7.73 GiB",
		"Traceback (most recent call last):",
		`  File "C:\\x\\attn_video_vae.py", line 360, in forward`,
		"    output_tensor = (input_tensor + hidden_states) / self.output_scale_factor",
		"torch.OutOfMemoryError: Allocation on device 0 would exceed allowed memory. (out of memory)",
		"Currently allocated     : 7.73 GiB",
		"[15:58:42.307] 🧹 Process 27656 terminating - VRAM will be automatically freed",
		"[15:58:42.308] ⭐💝 https://github.com/numz/ComfyUI-SeedVR2_VideoUpscaler",
	}
	if got := summarize(seedvr2); got != "OutOfMemoryError: Allocation on device 0 would exceed allowed memory. (out of memory)" {
		t.Errorf("seedvr2: %q", got)
	}
	flagged := []string{"[12:00:00.000] ❌ [ERROR] Failed to download required models.", "[12:00:00.001] ⭐ https://example.com"}
	if got := summarize(flagged); got != "Failed to download required models." {
		t.Errorf("flagged: %q", got)
	}
	// Warnings aren't failures.
	if got := summarize([]string{"UserWarning: deprecated", "it stopped"}); got != "it stopped" {
		t.Errorf("warning: %q", got)
	}
}
