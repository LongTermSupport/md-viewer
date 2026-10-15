package render

import (
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
