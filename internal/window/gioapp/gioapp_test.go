package gioapp

import (
	"image"
	"testing"

	"gioui.org/app"
	"gioui.org/unit"
)

// configAt applies the options of title at the metric of pxPerDp pixels per dp, and returns the
// app.Config they make.
func configAt(title string, pxPerDp float32) app.Config {
	var cfg app.Config
	m := unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp}
	for _, o := range options(title) {
		o(m, &cfg)
	}
	return cfg
}

// TestWindowOptions: the window has the title, opens at 980 × 900 dp and cannot be made smaller
// than 320 × 240 dp (WIN-1, WIN-2). The sizes are literal dp, so at two pixels per dp they double.
func TestWindowOptions(t *testing.T) {
	t.Run("one pixel per dp", func(t *testing.T) {
		cfg := configAt("a.md - md-viewer", 1)
		if cfg.Title != "a.md - md-viewer" {
			t.Errorf("Title = %q, want %q", cfg.Title, "a.md - md-viewer")
		}
		if cfg.Size != image.Pt(980, 900) {
			t.Errorf("Size = %v, want 980 × 900", cfg.Size)
		}
		if cfg.MinSize != image.Pt(320, 240) {
			t.Errorf("MinSize = %v, want 320 × 240", cfg.MinSize)
		}
		if cfg.Mode != app.Windowed {
			t.Errorf("Mode = %v, want windowed", cfg.Mode)
		}
	})

	t.Run("two pixels per dp", func(t *testing.T) {
		cfg := configAt("a.md - md-viewer", 2)
		if cfg.Size != image.Pt(1960, 1800) {
			t.Errorf("Size = %v, want 1960 × 1800", cfg.Size)
		}
		if cfg.MinSize != image.Pt(640, 480) {
			t.Errorf("MinSize = %v, want 640 × 480", cfg.MinSize)
		}
	})
}
