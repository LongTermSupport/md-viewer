package render

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Typeface names of the embedded families (STYLE-FONT-1).
const (
	typefaceText font.Typeface = "Go"
	typefaceMono font.Typeface = "Go Mono"
)

// Faces holds the six parsed Go font faces (STYLE-FONT-1).
type Faces struct{ Collection []font.FontFace }

// faceSource is one embedded font file and the name it is reported by in errors.
type faceSource struct {
	name string
	ttf  []byte
}

// goFaceSources returns the six embedded faces in a fixed order (STYLE-FONT-1). This list is
// the seam for the fonts decision (MILESTONES "Decisions for the owner", 01): an emoji or
// system font is one more source.
func goFaceSources() []faceSource {
	return []faceSource{
		{"goregular", goregular.TTF},
		{"goitalic", goitalic.TTF},
		{"gobold", gobold.TTF},
		{"gobolditalic", gobolditalic.TTF},
		{"gomono", gomono.TTF},
		{"gomonobold", gomonobold.TTF},
	}
}

// parseFaces parses each source into one face. A source that does not parse, or that yields
// other than one face, is an error that names it.
func parseFaces(sources []faceSource) (Faces, error) {
	out := make([]font.FontFace, 0, len(sources))
	for _, s := range sources {
		faces, err := opentype.ParseCollection(s.ttf)
		if err != nil {
			return Faces{}, fmt.Errorf("render: font %s: %w", s.name, err)
		}
		if len(faces) != 1 {
			return Faces{}, fmt.Errorf("render: font %s has %d faces, want 1", s.name, len(faces))
		}
		out = append(out, faces[0])
	}
	return Faces{Collection: out}, nil
}

// LoadFaces parses the six embedded faces (STYLE-FONT-1). Nothing is loaded before this is
// called (SPEED-6).
func LoadFaces() (Faces, error) {
	return parseFaces(goFaceSources())
}

// NewShaper returns a shaper that uses only the faces in f, never a system font (STYLE-FONT-1,
// SPEED-4).
func NewShaper(f Faces) *text.Shaper {
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(f.Collection))
}
