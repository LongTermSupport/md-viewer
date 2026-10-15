package render

import (
	"cmp"
	"image"
	"math"
	"slices"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// vlines returns the TableLine marks one dp wide, which are the vertical lines, in order of x.
func vlines(tr *Trace) []Mark {
	var out []Mark
	for _, m := range marks(tr, MarkTableLine) {
		if m.Rect.Dx() == 1 {
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b Mark) int { return cmp.Compare(a.Rect.Min.X, b.Rect.Min.X) })
	return out
}

// hlines returns the TableLine marks one dp tall, which are the horizontal lines, in order of y.
func hlines(tr *Trace) []Mark {
	var out []Mark
	for _, m := range marks(tr, MarkTableLine) {
		if m.Rect.Dy() == 1 {
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b Mark) int { return cmp.Compare(a.Rect.Min.Y, b.Rect.Min.Y) })
	return out
}

// fills returns the RowFill marks, in trace order.
func fills(tr *Trace) []Mark {
	return marks(tr, MarkRowFill)
}

// blockOf lays out the first block of src at width px in ctx, and returns its dimensions and trace.
func blockOf(t *testing.T, ctx blockCtx, src string, width int) (layout.Dimensions, *Trace) {
	t.Helper()
	return layBlock(t, ctx, firstBlock(t, src), width)
}

// traceOf is the trace of the first block of src at width px. A table is never wider than its width.
func traceOf(t *testing.T, ctx blockCtx, src string, width int) *Trace {
	t.Helper()
	dims, tr := blockOf(t, ctx, src, width)
	if dims.Size.X > width {
		t.Fatalf("table is %d px wide at %d px", dims.Size.X, width)
	}
	return tr
}

// viewTrace is the trace of element 0 of e at width px. The element is that wide.
func viewTrace(t *testing.T, e *viewEnv, width int) *Trace {
	t.Helper()
	dims, tr := e.layout(0, width)
	if dims.Size.X != width {
		t.Fatalf("element 0 is %d px wide at %d px", dims.Size.X, width)
	}
	return tr
}

// needCount fails the test unless ms has n marks; what names them in the message.
func needCount(t *testing.T, what string, ms []Mark, n int) {
	t.Helper()
	if len(ms) != n {
		t.Fatalf("%d %s, want %d", len(ms), what, n)
	}
}

// needText returns the marks of kind whose text is s, failing the test unless there are at least n.
func needText(t *testing.T, tr *Trace, kind MarkKind, s string, n int) []Mark {
	t.Helper()
	ms := textMarks(tr, kind, s)
	if len(ms) < n {
		t.Fatalf("%d marks of kind %d with text %q, want at least %d", len(ms), kind, s, n)
	}
	return ms
}

// TestCellAlign: a column's alignment maps to the line placement of its cells (STYLE-TABLE-4): Left
// and a body None are at the start, Centre and a header None are centred, Right is at the end. An
// alignment that is not one of the four is an error that names it.
func TestCellAlign(t *testing.T) {
	tests := []struct {
		name   string
		align  doc.Align
		header bool
		want   textAlign
	}{
		{"left body", doc.AlignLeft, false, alignStart},
		{"left header", doc.AlignLeft, true, alignStart},
		{"centre body", doc.AlignCentre, false, alignCentre},
		{"centre header", doc.AlignCentre, true, alignCentre},
		{"right body", doc.AlignRight, false, alignEnd},
		{"right header", doc.AlignRight, true, alignEnd},
		{"none body", doc.AlignNone, false, alignStart},
		{"none header", doc.AlignNone, true, alignCentre},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cellAlign(tt.align, tt.header)
			if err != nil {
				t.Fatalf("cellAlign: %v", err)
			}
			if got != tt.want {
				t.Fatalf("cellAlign(%d, %t) = %d, want %d", tt.align, tt.header, got, tt.want)
			}
		})
	}
	t.Run("unknown", func(t *testing.T) {
		if _, err := cellAlign(doc.Align(9), false); err == nil || !strings.Contains(err.Error(), "9") {
			t.Fatalf("alignment 9: error %v, want one naming 9", err)
		}
	})
}

// TestTableNaturalWidth: a table whose columns fit in its width keeps each column's natural width,
// N, and is ΣN + 1 dp wide, left-aligned (STYLE-TABLE-5).
func TestTableNaturalWidth(t *testing.T) {
	t.Run("in a view", func(t *testing.T) {
		e := newViewEnv(t, "| h | h |\n| - | - |\n| a | bbbbbbbb |", Light, metric1)
		dims, tr := e.layout(0, 800)
		if dims.Size.Y != 165 {
			t.Errorf("element is %d px tall, want 165 (45 + 75 + 45)", dims.Size.Y)
		}
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 3)
		for i, x := range []int{45, 82, 181} {
			if vs[i].Rect.Min.X != x {
				t.Errorf("vertical line %d at x=%d, want %d", i, vs[i].Rect.Min.X, x)
			}
		}
		hs := hlines(tr)
		needCount(t, "horizontal lines", hs, 3)
		for _, h := range hs {
			if h.Rect.Min.X != 45 || h.Rect.Max.X != 182 {
				t.Errorf("horizontal line %v, want x 45 to 182", h.Rect)
			}
		}
	})

	ctx := newBlockCtx(t, Light)
	src := "| h |\n| - |\n| bbbbbbbb |"
	t.Run("fits exactly", func(t *testing.T) {
		dims, tr := blockOf(t, ctx, src, 100)
		if dims.Size != image.Pt(100, 75) {
			t.Fatalf("Size = %v, want (100,75)", dims.Size)
		}
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 2)
		if vs[0].Rect.Min.X != 0 || vs[1].Rect.Min.X != 99 {
			t.Fatalf("vertical lines at x %d and %d, want 0 and 99", vs[0].Rect.Min.X, vs[1].Rect.Min.X)
		}
		needText(t, tr, MarkGlyphs, "bbbbbbbb", 1)
	})
	t.Run("one dp too narrow", func(t *testing.T) {
		dims, tr := blockOf(t, ctx, src, 99)
		if dims.Size != image.Pt(99, 99) {
			t.Fatalf("Size = %v, want (99,99)", dims.Size)
		}
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 2)
		if vs[0].Rect.Min.X != 0 || vs[1].Rect.Min.X != 98 {
			t.Fatalf("vertical lines at x %d and %d, want 0 and 98", vs[0].Rect.Min.X, vs[1].Rect.Min.X)
		}
		want := []string{"h", "bbbbbbb", "b"}
		if got := markTexts(marks(tr, MarkGlyphs)); !slices.Equal(got, want) {
			t.Fatalf("glyph texts %q, want %q (the body cell wraps)", got, want)
		}
	})
}

// TestTableTooWide: a table wider than its width is fitted to W - 1 dp, and its cells wrap; if its
// minimum columns still do not fit, it is clipped at W (STYLE-TABLE-5).
func TestTableTooWide(t *testing.T) {
	words := strings.TrimSpace(strings.Repeat("word ", 20))
	t.Run("scaled", func(t *testing.T) {
		src := "| h | h |\n| - | - |\n| " + words + " | " + words + " |"
		e := newViewEnv(t, src, Light, metric1)
		tr := viewTrace(t, e, 400)
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 3)
		for i, x := range []int{15, 199, 383} {
			if vs[i].Rect.Min.X != x {
				t.Errorf("vertical line %d at x=%d, want %d", i, vs[i].Rect.Min.X, x)
			}
		}
		for _, m := range marks(tr, MarkGlyphs) {
			lo, hi := 29, 186
			if m.Rect.Min.X >= 199 {
				lo, hi = 213, 370
			}
			if m.Rect.Min.X < lo || m.Rect.Max.X > hi {
				t.Errorf("glyphs %q span x %d to %d, want within their cell's %d to %d", m.Text, m.Rect.Min.X, m.Rect.Max.X, lo, hi)
			}
		}
		hs := hlines(tr)
		needCount(t, "horizontal lines", hs, 3)
		if pitch := hs[2].Rect.Min.Y - hs[1].Rect.Min.Y; pitch <= 37 {
			t.Errorf("body row is %d px tall, want more than 37 (its text wraps)", pitch)
		}
		for _, m := range append(marks(tr, MarkTableLine), fills(tr)...) {
			if m.Rect.Max.X > 385 {
				t.Errorf("%v ends at x=%d, want at most 385", m.Rect, m.Rect.Max.X)
			}
		}
	})
	t.Run("clipped", func(t *testing.T) {
		src := "| h | h | h | h | h | h | h |\n| - | - | - | - | - | - | - |\n" +
			"| bbbbbbbb | bbbbbbbb | bbbbbbbb | bbbbbbbb | bbbbbbbb | bbbbbbbb | bbbbbbbb |"
		e := newViewEnv(t, src, Light, metric1)
		tr := viewTrace(t, e, 400)
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 6)
		for i, x := range []int{15, 79, 143, 207, 271, 335} {
			if vs[i].Rect.Min.X != x {
				t.Errorf("vertical line %d at x=%d, want %d", i, vs[i].Rect.Min.X, x)
			}
		}
		hs := hlines(tr)
		needCount(t, "horizontal lines", hs, 3)
		for _, h := range hs {
			if h.Rect.Min.X != 15 || h.Rect.Max.X != 385 {
				t.Errorf("horizontal line %v, want x 15 to 385", h.Rect)
			}
		}
		for _, m := range append(marks(tr, MarkTableLine), fills(tr)...) {
			if m.Rect.Max.X > 385 {
				t.Errorf("%v ends at x=%d, want at most 385", m.Rect, m.Rect.Max.X)
			}
		}
		clipped, clippedTr := blockOf(t, newBlockCtx(t, Light), src, 370)
		if clipped.Size.X != 370 {
			t.Errorf("clipped table is %d px wide, want 370", clipped.Size.X)
		}
		needCount(t, "vertical lines of the clipped table", vlines(clippedTr), 6)
	})
}

// TestTableLines: every cell edge is a 1 dp Border line, and neighbouring cells share theirs
// (STYLE-TABLE-1). A cell's text is inside the padding of its lines.
func TestTableLines(t *testing.T) {
	src := "| a | b | c |\n| - | - | - |\n| d | e | f |\n| g | h | i |"
	for _, pal := range []struct {
		name string
		p    Palette
	}{{"light", Light}, {"dark", Dark}} {
		t.Run(pal.name, func(t *testing.T) {
			dims, tr := blockOf(t, newBlockCtx(t, pal.p), src, 800)
			if dims.Size.Y != 112 {
				t.Errorf("table is %d px tall, want 112", dims.Size.Y)
			}
			vs := vlines(tr)
			needCount(t, "vertical lines", vs, 4)
			for i, want := range []image.Rectangle{
				image.Rect(0, 0, 1, 112), image.Rect(36, 0, 37, 112), image.Rect(73, 0, 74, 112), image.Rect(109, 0, 110, 112),
			} {
				if vs[i].Rect != want {
					t.Errorf("vertical line %d is %v, want %v", i, vs[i].Rect, want)
				}
			}
			hs := hlines(tr)
			needCount(t, "horizontal lines", hs, 4)
			for i, want := range []image.Rectangle{
				image.Rect(0, 0, 110, 1), image.Rect(0, 37, 110, 38), image.Rect(0, 74, 110, 75), image.Rect(0, 111, 110, 112),
			} {
				if hs[i].Rect != want {
					t.Errorf("horizontal line %d is %v, want %v", i, hs[i].Rect, want)
				}
			}
			all := marks(tr, MarkTableLine)
			for i, m := range all {
				if m.Colour != pal.p.Border {
					t.Errorf("line %v is %#v, want Border %#v", m.Rect, m.Colour, pal.p.Border)
				}
				for j := i + 1; j < len(all); j++ {
					if all[j].Rect == m.Rect {
						t.Errorf("line %v is recorded twice", m.Rect)
					}
				}
			}
			for _, g := range marks(tr, MarkGlyphs) {
				left := math.MinInt
				for _, v := range vs {
					if v.Rect.Min.X <= g.Rect.Min.X {
						left = max(left, v.Rect.Max.X)
					}
				}
				top := math.MinInt
				for _, h := range hs {
					if h.Rect.Min.Y <= g.Rect.Min.Y {
						top = max(top, h.Rect.Max.Y)
					}
				}
				if g.Rect.Min.X < left+13 || g.Rect.Min.Y < top+6 {
					t.Errorf("glyphs %q at %v are closer than 13 dp and 6 dp to their cell's lines (%d, %d)", g.Text, g.Rect, left, top)
				}
			}
		})
	}

	t.Run("header only", func(t *testing.T) {
		dims, tr := blockOf(t, newBlockCtx(t, Light), "| a |\n| - |", 800)
		needCount(t, "vertical lines", vlines(tr), 2)
		needCount(t, "horizontal lines", hlines(tr), 2)
		if dims.Size.Y != 38 {
			t.Errorf("header-only table is %d px tall, want 38", dims.Size.Y)
		}
	})
}

// TestTableHeaderBold: a header cell is set in the bold face, and a link in the header keeps its
// colour (STYLE-TABLE-2). Body cells keep their own runs' styles.
func TestTableHeaderBold(t *testing.T) {
	ctx := newBlockCtx(t, Light)
	t.Run("widths", func(t *testing.T) {
		tr := traceOf(t, ctx, "| wide wide wide | *e* |\n| - | - |\n| wide wide wide | *e* |", 800)
		wide := needText(t, tr, MarkGlyphs, "wide wide wide", 2)
		if wide[0].Rect.Dx() != 117 || wide[1].Rect.Dx() != 109 {
			t.Errorf("header is %d px and body %d px, want 117 (bold) and 109", wide[0].Rect.Dx(), wide[1].Rect.Dx())
		}
		if wide[0].Colour != Light.Fg {
			t.Errorf("header glyphs are %#v, want Fg", wide[0].Colour)
		}
		e := needText(t, tr, MarkGlyphs, "e", 2)
		if e[0].Rect.Dx() != 9 || e[1].Rect.Dx() != 10 {
			t.Errorf("header e is %d px and body e %d px, want 9 (bold italic) and 10 (italic)", e[0].Rect.Dx(), e[1].Rect.Dx())
		}
	})
	t.Run("link", func(t *testing.T) {
		tr := traceOf(t, ctx, "| [l](https://e.org/) |\n| - |", 800)
		l := needText(t, tr, MarkGlyphs, "l", 1)[0]
		if l.Colour != Light.Accent || l.Rect.Dx() != 5 {
			t.Errorf("header link is %#v, %d px wide, want Accent and 5 px", l.Colour, l.Rect.Dx())
		}
	})
}

// TestTableStripes: the header row is filled Canvas, and body rows alternate: the 2nd, 4th, ... body
// row is BgMuted, the others Canvas (STYLE-TABLE-2, STYLE-TABLE-3).
func TestTableStripes(t *testing.T) {
	src := "| h |\n| - |\n| 1 |\n| 2 |\n| 3 |\n| 4 |"
	for _, pal := range []struct {
		name string
		p    Palette
	}{{"light", Light}, {"dark", Dark}} {
		t.Run(pal.name, func(t *testing.T) {
			tr := traceOf(t, newBlockCtx(t, pal.p), src, 800)
			want := []struct {
				rect  image.Rectangle
				muted bool
			}{
				{image.Rect(0, 1, 38, 37), false},
				{image.Rect(0, 38, 38, 74), false},
				{image.Rect(0, 75, 38, 111), true},
				{image.Rect(0, 112, 38, 148), false},
				{image.Rect(0, 149, 38, 185), true},
			}
			fs := fills(tr)
			needCount(t, "row fills", fs, len(want))
			for i, w := range want {
				wantColour := pal.p.Canvas
				if w.muted {
					wantColour = pal.p.BgMuted
				}
				if fs[i].Rect != w.rect || fs[i].Colour != wantColour {
					t.Errorf("fill %d is %v in %#v, want %v in %#v", i, fs[i].Rect, fs[i].Colour, w.rect, wantColour)
				}
			}
		})
	}
}

// TestTableAlignment: a column's cells are placed by its alignment (STYLE-TABLE-4). In a 136 dp
// column, Right text ends 13 px from the cell's right edge and Centre text is centred, a None body
// starts 13 px from the left and a None header is centred. A wrapped centred line is centred too.
func TestTableAlignment(t *testing.T) {
	ctx := newBlockCtx(t, Light)
	src := "| Left | Centre | Right | None |\n| :--- | :----: | ----: | ---- |\n" +
		"| wide wide wide | wide wide wide | wide wide wide | wide wide wide |\n| a | a | a | a |"
	tr := traceOf(t, ctx, src, 800)
	vs := vlines(tr)
	needCount(t, "vertical lines", vs, 5)
	for i, x := range []int{0, 136, 272, 408, 544} {
		if vs[i].Rect.Min.X != x {
			t.Errorf("vertical line %d at x=%d, want %d", i, vs[i].Rect.Min.X, x)
		}
	}
	headers := []struct {
		text     string
		min, max int
	}{{"Left", 14, 44}, {"Centre", 179, 230}, {"Right", 353, 395}, {"None", 456, 497}}
	for _, h := range headers {
		g := needText(t, tr, MarkGlyphs, h.text, 1)[0]
		if g.Rect.Min.X != h.min || g.Rect.Max.X != h.max || g.Rect.Min.Y != 9 {
			t.Errorf("header %q spans x %d to %d from y %d, want x %d to %d from y 9", h.text, g.Rect.Min.X, g.Rect.Max.X, g.Rect.Min.Y, h.min, h.max)
		}
	}
	rowA := needText(t, tr, MarkGlyphs, "a", 4)
	for i, want := range []struct{ min, max int }{{14, 23}, {200, 209}, {386, 395}, {422, 431}} {
		if g := rowA[i]; g.Rect.Min.X != want.min || g.Rect.Max.X != want.max || g.Rect.Min.Y != 83 {
			t.Errorf("row 2 cell %d spans x %d to %d from y %d, want x %d to %d from y 83", i, g.Rect.Min.X, g.Rect.Max.X, g.Rect.Min.Y, want.min, want.max)
		}
	}

	wrapped := traceOf(t, ctx, "| x |\n| :-: |\n| aa aa aa aa aa aa |", 120)
	var lines []Mark
	for _, g := range marks(wrapped, MarkGlyphs) {
		if g.Text != "x" {
			lines = append(lines, g)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("%d body lines, want at least 2 (the cell wraps)", len(lines))
	}
	for _, g := range lines {
		left := g.Rect.Min.X - 14
		right := 106 - g.Rect.Max.X
		if d := left - right; d < -1 || d > 1 {
			t.Errorf("line %q has %d px before and %d px after, want within 1 px of each other", g.Text, left, right)
		}
	}
}

// TestTableRowHeight: a row is as tall as its tallest cell, and a short cell's text is at the top
// of the cell (STYLE-TABLE-6).
func TestTableRowHeight(t *testing.T) {
	ctx := newBlockCtx(t, Light)
	t.Run("tallest cell", func(t *testing.T) {
		tr := traceOf(t, ctx, "| h | hhhhhhhhhh |\n| - | - |\n| bbbbbbbb bbbbbbbb | a |", 200)
		hs := hlines(tr)
		needCount(t, "horizontal lines", hs, 3)
		bs := needText(t, tr, MarkGlyphs, "bbbbbbbb", 2)
		tops := map[int]bool{}
		for _, b := range bs {
			tops[b.Rect.Min.Y] = true
		}
		k := len(tops)
		if k < 2 {
			t.Fatalf("the body text has %d lines, want at least 2", k)
		}
		if pitch := hs[2].Rect.Min.Y - hs[1].Rect.Min.Y; pitch != 13+24*k {
			t.Errorf("body row pitch is %d px, want %d (13 + 24 per line of %d lines)", pitch, 13+24*k, k)
		}
		a := needText(t, tr, MarkGlyphs, "a", 1)[0]
		if want := hs[1].Rect.Max.Y + 6 + 2; a.Rect.Min.Y != want {
			t.Errorf("a is at y=%d, want %d (its cell's top)", a.Rect.Min.Y, want)
		}
		if a.Rect.Min.Y != bs[0].Rect.Min.Y {
			t.Errorf("a is at y=%d and the first bbbbbbbb at y=%d, want the same (top aligned)", a.Rect.Min.Y, bs[0].Rect.Min.Y)
		}
	})
	t.Run("three lines", func(t *testing.T) {
		d := &doc.Document{Blocks: []doc.Block{doc.Table{
			Align:  []doc.Align{doc.AlignNone},
			Header: []doc.Cell{{Runs: []doc.Run{plain("h")}}},
			Rows:   [][]doc.Cell{{{Runs: []doc.Run{plain("a"), brk, plain("b"), brk, plain("c")}}}},
		}}}
		e := newViewEnvDoc(t, d, Light, metric1)
		tr := viewTrace(t, e, 800)
		hs := hlines(tr)
		needCount(t, "horizontal lines", hs, 3)
		if pitch := hs[2].Rect.Min.Y - hs[1].Rect.Min.Y; pitch != 85 {
			t.Errorf("body row pitch is %d px, want 85 (13 + 72 for three lines)", pitch)
		}
	})
}

// TestTableEmptyCells: an empty cell is one empty line, so an empty table is as wide as its padding
// and borders, and a short row is padded with empty cells (DOC-10, STYLE-TABLE-6).
func TestTableEmptyCells(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dims, tr := blockOf(t, newBlockCtx(t, Light), "| |\n| - |\n| |", 800)
		if dims.Size != image.Pt(28, 75) {
			t.Errorf("Size = %v, want (28,75)", dims.Size)
		}
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 2)
		if vs[0].Rect.Min.X != 0 || vs[1].Rect.Min.X != 27 {
			t.Errorf("vertical lines at x %d and %d, want 0 and 27", vs[0].Rect.Min.X, vs[1].Rect.Min.X)
		}
		needCount(t, "horizontal lines", hlines(tr), 3)
		if n := len(marks(tr, MarkGlyphs)); n != 0 {
			t.Errorf("%d glyphs in empty cells, want none", n)
		}
	})
	t.Run("short row", func(t *testing.T) {
		tr := traceOf(t, newBlockCtx(t, Light), "| a | b |\n| - | - |\n| c |", 800)
		needCount(t, "vertical lines", vlines(tr), 3)
		hs := hlines(tr)
		needCount(t, "horizontal lines", hs, 3)
		if pitch := hs[2].Rect.Min.Y - hs[1].Rect.Min.Y; pitch != 37 {
			t.Errorf("body row pitch is %d px, want 37 (the padded cell is one empty line)", pitch)
		}
	})
}

// TestTableTinyWidth: a table lays out at every width from negative to narrow, at three metrics,
// with no error. Its height is the same at every width: each column is its minimum or its natural
// width, so each cell is one line. Its lines and fills stay in the content (STYLE-TABLE-5).
func TestTableTinyWidth(t *testing.T) {
	widths := []int{-5, 0, 1, 2, 5, 10, 31, 33, 40, 77}
	heights := map[float32]int{1: 105, 1.5: 160, 2.25: 238}
	for _, m := range []unit.Metric{
		{PxPerDp: 1, PxPerSp: 1},
		{PxPerDp: 1.5, PxPerSp: 1.5},
		{PxPerDp: 2.25, PxPerSp: 2.25},
	} {
		e := newViewEnv(t, "| a |\n| - |\n| b |", Light, m)
		pad := int(math.Round(15 * float64(m.PxPerDp)))
		for _, w := range widths {
			dims, tr := e.layout(0, w)
			if dims.Size.Y != heights[m.PxPerDp] {
				t.Errorf("metric %v, width %d: element is %d px tall, want %d", m.PxPerDp, w, dims.Size.Y, heights[m.PxPerDp])
			}
			hi := pad + max(w-2*pad, 0)
			for _, ms := range [][]Mark{marks(tr, MarkTableLine), fills(tr)} {
				for _, mk := range ms {
					if mk.Rect.Min.X < pad || mk.Rect.Max.X > hi {
						t.Errorf("metric %v, width %d: %v spans x %d to %d, want within %d to %d", m.PxPerDp, w, mk.Rect, mk.Rect.Min.X, mk.Rect.Max.X, pad, hi)
					}
				}
			}
		}
	}
}

// TestTableNested: a table in a quote is muted in its text, keeps its lines and fills, and is as
// tall as its quote; a table that starts a list item is marked on its header's baseline (STYLE-
// QUOTE-1, STYLE-LIST-2, STYLE-TABLE-1).
func TestTableNested(t *testing.T) {
	t.Run("quote", func(t *testing.T) {
		e := newViewEnv(t, "> | a |\n> | - |\n> | b |", Light, metric1)
		dims, tr := e.layout(0, 800)
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 2)
		if vs[0].Rect.Min.X != 65 || vs[1].Rect.Min.X != 101 {
			t.Errorf("vertical lines at x %d and %d, want 65 and 101", vs[0].Rect.Min.X, vs[1].Rect.Min.X)
		}
		for _, s := range []string{"a", "b"} {
			if g := needText(t, tr, MarkGlyphs, s, 1)[0]; g.Colour != Light.FgMuted {
				t.Errorf("table text %q is %#v, want FgMuted", s, g.Colour)
			}
		}
		for _, m := range marks(tr, MarkTableLine) {
			if m.Colour != Light.Border {
				t.Errorf("line %v is %#v, want Border", m.Rect, m.Colour)
			}
		}
		fs := fills(tr)
		needCount(t, "row fills", fs, 2)
		for _, f := range fs {
			if f.Colour != Light.Canvas && f.Colour != Light.BgMuted {
				t.Errorf("row fill %v is %#v, want Canvas or BgMuted", f.Rect, f.Colour)
			}
		}
		bar := needMarks(t, tr, MarkQuoteBar, "QuoteBar")[0]
		if bar.Rect.Dy() != 75 {
			t.Errorf("quote bar is %d px tall, want 75 (the table)", bar.Rect.Dy())
		}
		if dims.Size.Y != 165 {
			t.Errorf("quote of a table is %d px tall, want 165", dims.Size.Y)
		}
	})
	t.Run("list", func(t *testing.T) {
		e := newViewEnv(t, "- | a |\n  | - |\n  | b |", Light, metric1)
		dims, tr := e.layout(0, 800)
		vs := vlines(tr)
		needCount(t, "vertical lines", vs, 2)
		if vs[0].Rect.Min.X != 77 || vs[1].Rect.Min.X != 113 {
			t.Errorf("vertical lines at x %d and %d, want 77 and 113", vs[0].Rect.Min.X, vs[1].Rect.Min.X)
		}
		if b := onlyMark(t, tr, MarkBullet); b.Text != "disc" || b.Rect != image.Rect(64, 63, 69, 68) {
			t.Errorf("bullet is %q at %v, want disc at (64,63)-(69,68), on the header's baseline", b.Text, b.Rect)
		}
		if dims.Size.Y != 165 {
			t.Errorf("list of a table is %d px tall, want 165", dims.Size.Y)
		}
	})
}

// TestTableError: a malformed table is a layout error that names it, and its element is
// Dimensions{} (STYLE-TABLE-5, ARCHITECTURE "internal/render").
func TestTableError(t *testing.T) {
	cell := func(runs ...doc.Run) doc.Cell { return doc.Cell{Runs: runs} }
	tests := []struct {
		name  string
		table doc.Table
		want  []string
	}{
		{"header run kind", doc.Table{Align: []doc.Align{doc.AlignNone}, Header: []doc.Cell{cell(doc.Run{Kind: 9})}}, []string{"run kind 9"}},
		{"body run kind", doc.Table{Align: []doc.Align{doc.AlignNone}, Header: []doc.Cell{{}}, Rows: [][]doc.Cell{{cell(doc.Run{Kind: 9})}}}, []string{"run kind 9"}},
		{"no columns", doc.Table{}, []string{"no columns"}},
		{"row with too many cells", doc.Table{Align: []doc.Align{doc.AlignNone}, Header: []doc.Cell{{}}, Rows: [][]doc.Cell{{{}, {}}}}, []string{"row 1", "2 cells"}},
		{"too many alignments", doc.Table{Align: []doc.Align{doc.AlignNone, doc.AlignNone}, Header: []doc.Cell{{}}}, []string{"2 alignments"}},
		{"unknown alignment", doc.Table{Align: []doc.Align{9}, Header: []doc.Cell{{}}}, []string{"alignment 9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnvDoc(t, &doc.Document{Blocks: []doc.Block{tt.table}}, Light, metric1)
			if dims := e.view.Layout(e.context(800), 0, nil); dims != (layout.Dimensions{}) {
				t.Fatalf("failing element is %+v, want Dimensions{}", dims)
			}
			for _, w := range tt.want {
				if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), w) {
					t.Errorf("Err() = %v, want an error containing %q", err, w)
				}
			}
		})
	}
}
