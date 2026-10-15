package render

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/font"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

// goFaceFonts are the six faces of STYLE-FONT-1, in the order LoadFaces returns them. Go Bold
// declares weight 600, which Gio names font.SemiBold (STYLE-FONT-3).
var goFaceFonts = []font.Font{
	{Typeface: "Go", Style: font.Regular, Weight: font.Normal},
	{Typeface: "Go", Style: font.Italic, Weight: font.Normal},
	{Typeface: "Go", Style: font.Regular, Weight: font.SemiBold},
	{Typeface: "Go", Style: font.Italic, Weight: font.SemiBold},
	{Typeface: "Go Mono", Style: font.Regular, Weight: font.Normal},
	{Typeface: "Go Mono", Style: font.Regular, Weight: font.SemiBold},
}

// TestLoadFaces checks that the six embedded faces load with the typefaces, styles and weights
// of STYLE-FONT-1.
func TestLoadFaces(t *testing.T) {
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	if len(faces.Collection) != 6 {
		t.Fatalf("len(Collection) = %d, want 6", len(faces.Collection))
	}
	got := make([]font.Font, 0, len(faces.Collection))
	for i, f := range faces.Collection {
		if f.Face == nil {
			t.Errorf("face %d: Face is nil", i)
		}
		got = append(got, f.Font)
	}
	for i := range goFaceFonts {
		if i < len(got) && got[i] != goFaceFonts[i] {
			t.Errorf("face %d:\n got %#v\nwant %#v", i, got[i], goFaceFonts[i])
		}
	}
}

// TestParseFacesError checks that a source that is not a font, or is empty, is an error that
// names the face.
func TestParseFacesError(t *testing.T) {
	tests := []struct {
		name    string
		sources []faceSource
		want    string
	}{
		{name: "not a font", sources: []faceSource{{"broken", []byte("not a font")}}, want: "broken"},
		{name: "empty", sources: []faceSource{{"empty", nil}}, want: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFaces(tt.sources)
			if err == nil {
				t.Fatalf("parseFaces: nil error, want one naming %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name %q", err.Error(), tt.want)
			}
		})
	}
}

// hello is what a layout of "Hello" gives: its glyph count and total advance.
type hello struct {
	glyphs  int
	advance fixed.Int26_6
}

// layoutHello lays out "Hello" at 16 px in the given typeface with s.
func layoutHello(s *text.Shaper, tf font.Typeface) hello {
	s.LayoutString(text.Parameters{
		Font:     font.Font{Typeface: tf},
		PxPerEm:  fixed.I(16),
		MaxWidth: 1 << 20,
	}, "Hello")
	var h hello
	for {
		g, ok := s.NextGlyph()
		if !ok {
			return h
		}
		h.glyphs++
		h.advance += g.Advance
	}
}

// TestNewShaper checks the shaper built from the embedded faces, and that it does not use
// system fonts (SPEED-4).
func TestNewShaper(t *testing.T) {
	t.Run("uses the Go faces", func(t *testing.T) {
		faces, err := LoadFaces()
		if err != nil {
			t.Fatalf("LoadFaces: %v", err)
		}
		s := NewShaper(faces)
		goText := layoutHello(s, typefaceText)
		goMono := layoutHello(s, typefaceMono)
		if goText.advance <= 0 || goMono.advance <= 0 {
			t.Errorf("widths of \"Hello\": Go %v, Go Mono %v, want both positive", goText.advance, goMono.advance)
		}
		if goText.advance == goMono.advance {
			t.Errorf("Go and Go Mono both %v wide: the mono face was not used", goText.advance)
		}
	})
	t.Run("no system fonts", func(t *testing.T) {
		s := NewShaper(Faces{})
		got := layoutHello(s, typefaceText)
		if got.glyphs >= len("Hello") {
			t.Errorf("\"Hello\" has %d glyphs with no collection, want fewer than %d: a system font was used", got.glyphs, len("Hello"))
		}
	})
}

// noSystemFontsMarkerEnv names the variable that makes TestNoSystemFontsHelperProcess run: its value
// is the file the helper writes once it has shaped text.
const noSystemFontsMarkerEnv = "MDV_NO_SYSTEM_FONTS_MARKER"

// glyphsOf lays out str in the typeface at 16 px and returns its glyphs in order.
func glyphsOf(s *text.Shaper, tf font.Typeface, str string) []text.Glyph {
	s.LayoutString(text.Parameters{
		Font:     font.Font{Typeface: tf},
		PxPerEm:  fixed.I(16),
		MaxWidth: 1 << 20,
	}, str)
	var out []text.Glyph
	for {
		g, ok := s.NextGlyph()
		if !ok {
			return out
		}
		out = append(out, g)
	}
}

// TestNoSystemFonts (STYLE-FONT-2, SPEED-4): with the six embedded faces only, a character they
// lack is the font's missing-glyph box. Part one shapes "日" and "☃" in process: each is one glyph
// of one rune with glyph id 0. The control "a" has a non-zero id, so the check cannot pass
// everything. In Gio v0.10.3 the glyph id is the low 32 bits of text.GlyphID (the size and the
// face index sit above it), so uint32(g.ID) is the font's glyph id; 0 is the notdef glyph.
//
// Part two runs a subprocess with HOME, XDG_CACHE_HOME and LocalAppData set to one empty directory.
// It shapes text and writes a marker, then the directory must still be empty: no system font
// index is written. The subprocess is needed because go-text builds that index once per process.
func TestNoSystemFonts(t *testing.T) {
	t.Run("missing glyphs", func(t *testing.T) {
		faces, err := LoadFaces()
		if err != nil {
			t.Fatalf("LoadFaces: %v", err)
		}
		s := NewShaper(faces)
		for _, str := range []string{"日", "☃"} {
			gs := glyphsOf(s, typefaceText, str)
			if len(gs) != 1 {
				t.Errorf("%q lays out as %d glyphs, want 1 missing-glyph cluster", str, len(gs))
				continue
			}
			if gs[0].Runes != 1 {
				t.Errorf("%q has %d runes in its cluster, want 1", str, gs[0].Runes)
			}
			if id := uint32(gs[0].ID); id != 0 {
				t.Errorf("%q has glyph id %d, want 0 (the missing-glyph box): a font other than the Go faces drew it", str, id)
			}
		}
		ctl := glyphsOf(s, typefaceText, "a")
		if len(ctl) != 1 || uint32(ctl[0].ID) == 0 {
			t.Errorf("control \"a\" gives %d glyphs with id %d, want one glyph with a non-zero id", len(ctl), glyphsIDs(ctl))
		}
	})

	t.Run("no font cache file", func(t *testing.T) {
		cache := t.TempDir()
		marker := filepath.Join(t.TempDir(), "shaped")
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNoSystemFontsHelperProcess$")
		cmd.Env = append(os.Environ(),
			noSystemFontsMarkerEnv+"="+marker,
			"HOME="+cache, "XDG_CACHE_HOME="+cache, "LocalAppData="+cache)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("helper process: %v\n%s", err, out)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("the helper process did not shape text: %v", err)
		}
		var files []string
		err := filepath.WalkDir(cache, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", cache, err)
		}
		if len(files) != 0 {
			t.Errorf("shaping wrote files to the user cache directory: %q, want none (SPEED-4)", files)
		}
	})
}

// glyphsIDs lists the glyph ids of gs, for a failure message.
func glyphsIDs(gs []text.Glyph) []uint32 {
	ids := make([]uint32, 0, len(gs))
	for _, g := range gs {
		ids = append(ids, uint32(g.ID))
	}
	return ids
}

// TestNoSystemFontsHelperProcess is the process TestNoSystemFonts runs. With the marker variable
// set, it shapes text with the embedded faces and writes the marker. Run as an ordinary test,
// without the variable, it does nothing.
func TestNoSystemFontsHelperProcess(t *testing.T) {
	marker, ok := os.LookupEnv(noSystemFontsMarkerEnv)
	if !ok {
		return
	}
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	// Gio builds its system-font index on the first shaping, not in NewShaper, so this must shape.
	glyphsOf(NewShaper(faces), typefaceText, "日")
	if err := os.WriteFile(marker, []byte("shaped\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", marker, err)
	}
}

// sinkShaper keeps the benchmark's result alive.
var sinkShaper *text.Shaper

// BenchmarkLoadFonts measures the start-up cost of the fonts: LoadFaces, then NewShaper.
func BenchmarkLoadFonts(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		faces, err := LoadFaces()
		if err != nil {
			b.Fatal(err)
		}
		sinkShaper = NewShaper(faces)
	}
}
