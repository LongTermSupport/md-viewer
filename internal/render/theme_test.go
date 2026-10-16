package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestDarkViewColours (STYLE-PAL-1, STYLE-PAL-2): elements.md laid out with the dark palette takes
// every colour from that palette. Each mark's colour is one of Dark's fields. ImagePixels and
// LinkArea carry no colour of their own. A layout that hard-codes a Light colour fails this test.
func TestDarkViewColours(t *testing.T) {
	src, err := os.ReadFile("../../testdata/elements.md")
	if err != nil {
		t.Fatalf("read elements.md: %v", err)
	}
	dir, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	e := newViewEnvIn(t, string(src), dir, Dark, metric1)
	dark := []color.NRGBA{Dark.Fg, Dark.Canvas, Dark.FgMuted, Dark.Accent, Dark.Border,
		Dark.BorderMuted, Dark.BgMuted, Dark.CodeBg, Dark.OnAccent}
	seen := 0
	for i := range e.view.Len() {
		_, tr := e.layout(i, 800)
		for _, m := range tr.Marks {
			switch m.Kind {
			case MarkImagePixels, MarkLinkArea:
				continue
			}
			seen++
			if !slices.Contains(dark, m.Colour) {
				t.Errorf("element %d: %v mark at %v is %v, which is no Dark colour", i, m.Kind, m.Rect, m.Colour)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no marks laid out, so the check proves nothing")
	}
}

// writeHalfTransparentPNG writes a 8 by 8 PNG whose left half is fully transparent and whose right
// half is opaque black.
func writeHalfTransparentPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := 4; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	writeFile(t, path, buf.Bytes())
}

// TestImageTransparentNoUnderlay (STYLE-IMG-2): nothing is painted under a drawn image, so after its
// pixels are decoded the image is one ImagePixels mark, with no box and no BgMuted fill. Its
// transparent pixels show Canvas. The test is run for both palettes. A layout that fills BgMuted under
// the pixels fails it.
func TestImageTransparentNoUnderlay(t *testing.T) {
	dir := t.TempDir()
	writeHalfTransparentPNG(t, filepath.Join(dir, "half.png"))
	for _, p := range []struct {
		name string
		pal  Palette
	}{{"light", Light}, {"dark", Dark}} {
		t.Run(p.name, func(t *testing.T) {
			e, sp := deferredEnv(t, "![half](half.png)\n", dir, p.pal, metric1)
			e.layout(0, 800)
			runWorker(t, sp)
			_, tr := e.layout(0, 800)
			if n := len(marks(tr, MarkImagePixels)); n != 1 {
				t.Fatalf("%d ImagePixels marks after the decode, want 1", n)
			}
			if n := len(marks(tr, MarkImageBox)); n != 0 {
				t.Errorf("%d ImageBox marks after the decode, want none", n)
			}
			for _, m := range tr.Marks {
				if m.Colour == p.pal.BgMuted {
					t.Errorf("%v mark at %v is filled BgMuted under the image", m.Kind, m.Rect)
				}
			}
		})
	}
}
