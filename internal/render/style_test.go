package render

import (
	"image/color"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gioui.org/font"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// Fonts written out by hand: the six faces of STYLE-FONT-1, in the form the styles use.
var (
	goRegular    = font.Font{Typeface: "Go", Style: font.Regular, Weight: font.Normal}
	goItalic     = font.Font{Typeface: "Go", Style: font.Italic, Weight: font.Normal}
	goBold       = font.Font{Typeface: "Go", Style: font.Regular, Weight: font.SemiBold}
	goBoldItalic = font.Font{Typeface: "Go", Style: font.Italic, Weight: font.SemiBold}
	monoRegular  = font.Font{Typeface: "Go Mono", Style: font.Regular, Weight: font.Normal}
	monoBold     = font.Font{Typeface: "Go Mono", Style: font.Regular, Weight: font.SemiBold}
)

// bodyBase is the body text's style with its font left as the Go family's default.
var bodyBase = TextStyle{Font: font.Font{Typeface: "Go"}, Size: 16, LineHeight: 1.5, Colour: Light.Fg}

// h1Base is heading level 1's style, written out.
var h1Base = TextStyle{Font: goBold, Size: 32, LineHeight: 1.25, Colour: Light.Fg}

// TestCodeBlockStyle checks a code block's text against STYLE-CODE-1: Go Mono at 13.6 sp (85% of
// 16), line height 1.45, in Fg.
func TestCodeBlockStyle(t *testing.T) {
	want := TextStyle{Font: monoRegular, Size: 13.6, LineHeight: 1.45, Colour: Light.Fg}
	if got := codeBlockStyle(Light); got != want {
		t.Errorf("codeBlockStyle(Light):\n got %#v\nwant %#v", got, want)
	}
	if got := codeBlockStyle(Dark).Colour; got != Dark.Fg {
		t.Errorf("codeBlockStyle(Dark).Colour = %#v, want Dark.Fg %#v", got, Dark.Fg)
	}
}

// TestBodyStyle checks the body text of STYLE-TEXT-1 in both palettes.
func TestBodyStyle(t *testing.T) {
	want := TextStyle{Font: goRegular, Size: 16, LineHeight: 1.5, Colour: Light.Fg}
	if got := BodyStyle(Light); got != want {
		t.Errorf("BodyStyle(Light):\n got %#v\nwant %#v", got, want)
	}
	if got := BodyStyle(Dark).Colour; got != Dark.Fg {
		t.Errorf("BodyStyle(Dark).Colour = %#v, want Dark.Fg %#v", got, Dark.Fg)
	}
}

// TestHeadingStyle checks each heading level against STYLE-TEXT-2, and the levels outside 1 to 6.
func TestHeadingStyle(t *testing.T) {
	tests := []struct {
		level  int
		pal    Palette
		size   unit.Sp
		colour color.NRGBA
		rule   HeadingRule
	}{
		{level: 1, pal: Light, size: 32, colour: Light.Fg, rule: HeadingRule{PaddingBelow: 9.6, Thickness: 1, Colour: Light.BorderMuted}},
		{level: 2, pal: Light, size: 24, colour: Light.Fg, rule: HeadingRule{PaddingBelow: 7.2, Thickness: 1, Colour: Light.BorderMuted}},
		{level: 3, pal: Light, size: 20, colour: Light.Fg},
		{level: 4, pal: Light, size: 16, colour: Light.Fg},
		{level: 5, pal: Light, size: 14, colour: Light.Fg},
		{level: 6, pal: Light, size: 13.6, colour: Light.FgMuted},
		{level: 1, pal: Dark, size: 32, colour: Dark.Fg, rule: HeadingRule{PaddingBelow: 9.6, Thickness: 1, Colour: Dark.BorderMuted}},
		{level: 6, pal: Dark, size: 13.6, colour: Dark.FgMuted},
	}
	for _, tt := range tests {
		t.Run("level "+strconv.Itoa(tt.level), func(t *testing.T) {
			wantStyle := TextStyle{Font: goBold, Size: tt.size, LineHeight: 1.25, Colour: tt.colour}
			gotStyle, gotRule, err := HeadingStyle(tt.level, tt.pal)
			if err != nil {
				t.Fatalf("HeadingStyle(%d): %v", tt.level, err)
			}
			if gotStyle != wantStyle {
				t.Errorf("style:\n got %#v\nwant %#v", gotStyle, wantStyle)
			}
			if gotRule != tt.rule {
				t.Errorf("rule:\n got %#v\nwant %#v", gotRule, tt.rule)
			}
		})
	}
	for _, level := range []int{0, 7} {
		t.Run("error at level "+strconv.Itoa(level), func(t *testing.T) {
			_, _, err := HeadingStyle(level, Light)
			if err == nil {
				t.Fatalf("HeadingStyle(%d): nil error, want one naming the level", level)
			}
			if !strings.Contains(err.Error(), strconv.Itoa(level)) {
				t.Errorf("error %q does not name level %d", err.Error(), level)
			}
		})
	}
}

// TestRunStyle checks each style flag, link and heading context against STYLE-FONT-3, 4 and
// STYLE-TEXT-3, 5. The line height always stays the base's.
func TestRunStyle(t *testing.T) {
	quoteBase := TextStyle{Font: font.Font{Typeface: "Go"}, Size: 16, LineHeight: 1.5, Colour: Light.FgMuted}
	tests := []struct {
		name      string
		base      TextStyle
		style     doc.Style
		link      bool
		pal       Palette
		inHeading bool
		want      TextStyle
	}{
		{name: "plain", base: bodyBase, pal: Light, want: TextStyle{Font: goRegular, Size: 16, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "emphasis", base: bodyBase, style: doc.Emphasis, pal: Light, want: TextStyle{Font: goItalic, Size: 16, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "strong", base: bodyBase, style: doc.Strong, pal: Light, want: TextStyle{Font: goBold, Size: 16, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "emphasis and strong", base: bodyBase, style: doc.Emphasis | doc.Strong, pal: Light, want: TextStyle{Font: goBoldItalic, Size: 16, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "strike", base: bodyBase, style: doc.Strike, pal: Light, want: TextStyle{Font: goRegular, Size: 16, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "code", base: bodyBase, style: doc.Code, pal: Light, want: TextStyle{Font: monoRegular, Size: 13.6, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "code and strong", base: bodyBase, style: doc.Code | doc.Strong, pal: Light, want: TextStyle{Font: monoBold, Size: 13.6, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "code and emphasis", base: bodyBase, style: doc.Code | doc.Emphasis, pal: Light, want: TextStyle{Font: monoRegular, Size: 13.6, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "code, emphasis and strong", base: bodyBase, style: doc.Code | doc.Emphasis | doc.Strong, pal: Light, want: TextStyle{Font: monoBold, Size: 13.6, LineHeight: 1.5, Colour: Light.Fg}},
		{name: "link", base: bodyBase, link: true, pal: Light, want: TextStyle{Font: goRegular, Size: 16, LineHeight: 1.5, Colour: Light.Accent}},
		{name: "link, code and strong", base: bodyBase, style: doc.Code | doc.Strong, link: true, pal: Light, want: TextStyle{Font: monoBold, Size: 13.6, LineHeight: 1.5, Colour: Light.Accent}},
		{name: "link in dark", base: bodyBase, link: true, pal: Dark, want: TextStyle{Font: goRegular, Size: 16, LineHeight: 1.5, Colour: Dark.Accent}},
		{name: "code in heading", base: h1Base, style: doc.Code, pal: Light, inHeading: true, want: TextStyle{Font: monoBold, Size: 32, LineHeight: 1.25, Colour: Light.Fg}},
		{name: "emphasis in heading", base: h1Base, style: doc.Emphasis, pal: Light, inHeading: true, want: TextStyle{Font: goBoldItalic, Size: 32, LineHeight: 1.25, Colour: Light.Fg}},
		{name: "link in quote", base: quoteBase, link: true, pal: Light, want: TextStyle{Font: goRegular, Size: 16, LineHeight: 1.5, Colour: Light.Accent}},
		{name: "code in quote", base: quoteBase, style: doc.Code, pal: Light, want: TextStyle{Font: monoRegular, Size: 13.6, LineHeight: 1.5, Colour: Light.FgMuted}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RunStyle(tt.base, tt.style, tt.link, tt.pal, tt.inHeading)
			if got != tt.want {
				t.Errorf("RunStyle:\n got %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

// TestStyleFontsAreLoaded checks that every font the styles can return is one of the six
// faces LoadFaces parses, so no style asks for a face that does not exist.
func TestStyleFontsAreLoaded(t *testing.T) {
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	loaded := make(map[font.Font]bool, len(faces.Collection))
	for _, f := range faces.Collection {
		loaded[f.Font] = true
	}
	check := func(name string, f font.Font) {
		t.Helper()
		if !loaded[f] {
			t.Errorf("%s: font %#v is not one of the six faces", name, f)
		}
	}
	for level := 1; level <= 6; level++ {
		hs, hr, err := HeadingStyle(level, Light)
		if err != nil {
			t.Fatalf("HeadingStyle(%d): %v", level, err)
		}
		check("heading "+strconv.Itoa(level), hs.Font)
		if level <= 2 && hr == (HeadingRule{}) {
			t.Errorf("heading %d has no rule below it, want one (STYLE-TEXT-2)", level)
		}
	}
	for _, base := range []TextStyle{bodyBase, h1Base} {
		for s := doc.Style(0); s < 16; s++ {
			for _, link := range []bool{false, true} {
				for _, inHeading := range []bool{false, true} {
					check("RunStyle", RunStyle(base, s, link, Light, inHeading).Font)
				}
			}
		}
	}
}

// TestMargins checks DOC-LAYOUT-4's table, including nested lists, and the error for a block
// with no row.
func TestMargins(t *testing.T) {
	tests := []struct {
		name       string
		block      doc.Block
		nested     bool
		wantTop    unit.Dp
		wantBottom unit.Dp
	}{
		{name: "paragraph", block: doc.Paragraph{}, wantTop: 0, wantBottom: 16},
		{name: "heading 1", block: doc.Heading{Level: 1}, wantTop: 24, wantBottom: 16},
		{name: "heading 6", block: doc.Heading{Level: 6}, wantTop: 24, wantBottom: 16},
		{name: "quote", block: doc.Quote{}, wantTop: 0, wantBottom: 16},
		{name: "list", block: doc.List{}, wantTop: 0, wantBottom: 16},
		{name: "nested list", block: doc.List{}, nested: true, wantTop: 0, wantBottom: 0},
		{name: "code block", block: doc.CodeBlock{}, wantTop: 0, wantBottom: 16},
		{name: "table", block: doc.Table{}, wantTop: 0, wantBottom: 16},
		{name: "rule", block: doc.Rule{}, wantTop: 24, wantBottom: 24},
		{name: "image", block: doc.Image{}, wantTop: 0, wantBottom: 16},
		{name: "paragraph nested", block: doc.Paragraph{}, nested: true, wantTop: 0, wantBottom: 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			top, bottom, err := Margins(tt.block, tt.nested)
			if err != nil {
				t.Fatalf("Margins: %v", err)
			}
			if top != tt.wantTop || bottom != tt.wantBottom {
				t.Errorf("Margins = %v, %v; want %v, %v", top, bottom, tt.wantTop, tt.wantBottom)
			}
		})
	}
	errTests := []struct {
		name  string
		block doc.Block
		want  string
	}{
		{name: "nil block", block: nil, want: "<nil>"},
		{name: "pointer to paragraph", block: &doc.Paragraph{}, want: "*doc.Paragraph"},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Margins(tt.block, false)
			if err == nil {
				t.Fatalf("Margins(%s): nil error, want one naming %s", tt.name, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name %s", err.Error(), tt.want)
			}
		})
	}
}

// TestGap checks the margin collapse of DOC-LAYOUT-3: the larger of the two margins.
func TestGap(t *testing.T) {
	tests := []struct {
		prev, next, want unit.Dp
	}{
		{16, 24, 24},
		{24, 0, 24},
		{0, 0, 0},
		{16, 16, 16},
		{0, 24, 24},
	}
	for _, tt := range tests {
		if got := Gap(tt.prev, tt.next); got != tt.want {
			t.Errorf("Gap(%v, %v) = %v, want %v", tt.prev, tt.next, got, tt.want)
		}
	}
}

// TestPadding checks the content padding of DOC-LAYOUT-1 at and around the 768 dp break.
func TestPadding(t *testing.T) {
	tests := []struct {
		width, want unit.Dp
	}{
		{768, 45},
		{980, 45},
		{2000, 45},
		{767, 15},
		{767.9, 15},
		{320, 15},
	}
	for _, tt := range tests {
		if got := Padding(tt.width); got != tt.want {
			t.Errorf("Padding(%v) = %v, want %v", tt.width, got, tt.want)
		}
	}
}

// TestOrderedMarker checks the numbering of STYLE-LIST-3 by depth, and the decimal fallback for
// values outside each style's range (STYLE-LIST-3).
func TestOrderedMarker(t *testing.T) {
	tests := []struct {
		n, depth int
		want     string
	}{
		{1, 0, "1."},
		{12, 0, "12."},
		{0, 0, "0."},
		{1, 1, "i."},
		{4, 1, "iv."},
		{9, 1, "ix."},
		{14, 1, "xiv."},
		{40, 1, "xl."},
		{3999, 1, "mmmcmxcix."},
		{4000, 1, "4000."},
		{0, 1, "0."},
		{1, 2, "a."},
		{26, 2, "z."},
		{27, 2, "aa."},
		{52, 2, "az."},
		{702, 2, "zz."},
		{703, 2, "aaa."},
		{0, 2, "0."},
		{1, 5, "a."},
	}
	for _, tt := range tests {
		if got := OrderedMarker(tt.n, tt.depth); got != tt.want {
			t.Errorf("OrderedMarker(%d, %d) = %q, want %q", tt.n, tt.depth, got, tt.want)
		}
	}
}

// TestBulletShape checks the marker of each nesting depth (STYLE-LIST-2): a disc at depth 0, a
// circle outline at depth 1, a square at depth 2 or more, and a disc below depth 0. Each shape's
// mark text is its name, and an unknown shape names its value.
func TestBulletShape(t *testing.T) {
	depths := []struct {
		depth int
		want  Bullet
	}{
		{-1, BulletDisc},
		{0, BulletDisc},
		{1, BulletCircle},
		{2, BulletSquare},
		{3, BulletSquare},
		{9, BulletSquare},
	}
	for _, tt := range depths {
		if got := BulletShape(tt.depth); got != tt.want {
			t.Errorf("BulletShape(%d) = %v, want %v", tt.depth, got, tt.want)
		}
	}
	names := []struct {
		shape Bullet
		want  string
	}{
		{BulletDisc, "disc"},
		{BulletCircle, "circle"},
		{BulletSquare, "square"},
	}
	for _, tt := range names {
		if got := tt.shape.String(); got != tt.want {
			t.Errorf("Bullet %d String() = %q, want %q", tt.shape, got, tt.want)
		}
	}
	if got := Bullet(7).String(); !strings.Contains(got, "7") {
		t.Errorf("Bullet(7).String() = %q, want it to name the value 7", got)
	}
}

// TestColumnWidths checks STYLE-TABLE-5: natural widths kept when they fit, scaled when not,
// the 64 dp minimum, zero natural widths, a negative width and the empty table. The input is not
// modified.
func TestColumnWidths(t *testing.T) {
	tests := []struct {
		name    string
		natural []int
		width   int
		want    []int
	}{
		{name: "fits", natural: []int{100, 200}, width: 400, want: []int{100, 200}},
		{name: "exactly fits, inclusive", natural: []int{100, 300}, width: 400, want: []int{100, 300}},
		{name: "scaled", natural: []int{600, 200}, width: 400, want: []int{300, 100}},
		{name: "floor", natural: []int{100, 100, 100}, width: 200, want: []int{66, 66, 66}},
		{name: "minimum applied", natural: []int{700, 100}, width: 400, want: []int{350, 64}},
		{name: "minimums exceed width", natural: []int{100, 100, 100, 100, 100, 100, 100}, width: 400, want: []int{64, 64, 64, 64, 64, 64, 64}},
		{name: "empty", natural: []int{}, width: 400, want: []int{}},
		{name: "zero sum fits (pins existing)", natural: []int{0, 0}, width: 400, want: []int{0, 0}},
		{name: "zero sum at zero width fits (pins existing)", natural: []int{0}, width: 0, want: []int{0}},
		{name: "negative width, zero sum: minimum applied", natural: []int{0}, width: -1, want: []int{64}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := slices.Clone(tt.natural)
			got := ColumnWidths(tt.natural, tt.width, 64)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ColumnWidths(%v, %d) = %v, want %v", tt.natural, tt.width, got, tt.want)
			}
			if !slices.Equal(tt.natural, before) {
				t.Errorf("input modified: got %v, was %v", tt.natural, before)
			}
		})
	}
}
