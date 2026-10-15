package render

import (
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// newBlockCtx is a block context on palette pal: text and root are both pal, with the faces and
// the link state a View gives its blocks.
func newBlockCtx(t *testing.T, pal Palette) blockCtx {
	t.Helper()
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	return blockCtx{text: pal, root: pal, shaper: NewShaper(faces), links: newLinks()}
}

// layBlock lays out b at width px at metric 1 into a new trace, and fails the test on an error.
func layBlock(t *testing.T, ctx blockCtx, b doc.Block, width int) (layout.Dimensions, *Trace) {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      metric1,
		Constraints: layout.Constraints{Max: image.Pt(width, 1<<20)},
	}
	tr := &Trace{}
	dims, err := layoutBlock(gtx, ctx, b, tr)
	if err != nil {
		t.Fatalf("layoutBlock: %v", err)
	}
	return dims, tr
}

// firstBlock is the first top-level block of src, parsed as a document.
func firstBlock(t *testing.T, src string) doc.Block {
	t.Helper()
	d, err := doc.Parse([]byte(src), "/doc")
	if err != nil {
		t.Fatalf("doc.Parse(%q): %v", src, err)
	}
	if len(d.Blocks) == 0 {
		t.Fatalf("%q has no blocks", src)
	}
	return d.Blocks[0]
}

// countItems is the number of list items in b. Lists inside items and inside quotes count too,
// since a quote does not end a list's nesting (STYLE-LIST-2).
func countItems(t *testing.T, b doc.Block) int {
	t.Helper()
	switch b := b.(type) {
	case doc.Quote:
		n := 0
		for _, c := range b.Blocks {
			n += countItems(t, c)
		}
		return n
	case doc.List:
		n := 0
		for _, item := range b.Items {
			n++
			for _, c := range item.Blocks {
				n += countItems(t, c)
			}
		}
		return n
	case doc.Heading, doc.Paragraph, doc.CodeBlock, doc.Rule, doc.Table, doc.Image:
		return 0
	default:
		t.Fatalf("countItems: no rule for block %T", b)
		return 0
	}
}

// markerMarks is the marks of the list markers in tr: the bullets, the numbers, and the task
// boxes whose text is open or done. The border and check marks of a box are not markers.
func markerMarks(tr *Trace) []Mark {
	var out []Mark
	for _, m := range tr.Marks {
		if m.Kind == MarkBullet || m.Kind == MarkNumber {
			out = append(out, m)
		} else if m.Kind == MarkTaskBox && (m.Text == "open" || m.Text == "done") {
			out = append(out, m)
		}
	}
	return out
}

// textMarks returns the marks of kind whose text is s, in trace order.
func textMarks(tr *Trace, kind MarkKind, s string) []Mark {
	var out []Mark
	for _, m := range marks(tr, kind) {
		if m.Text == s {
			out = append(out, m)
		}
	}
	return out
}

// markAt returns the mark of kind and text s at rectangle r, failing the test when there is none.
// Marks are matched by place, not by trace order: a marker is recorded after its item's content.
func markAt(t *testing.T, tr *Trace, kind MarkKind, s string, r image.Rectangle) Mark {
	t.Helper()
	for _, m := range tr.Marks {
		if m.Kind == kind && m.Text == s && m.Rect == r {
			return m
		}
	}
	t.Fatalf("no mark of kind %d with text %q at %v", kind, s, r)
	return Mark{}
}

// onlyMark returns the one mark of kind in tr, failing the test unless there is exactly one.
func onlyMark(t *testing.T, tr *Trace, kind MarkKind) Mark {
	t.Helper()
	ms := marks(tr, kind)
	if len(ms) != 1 {
		t.Fatalf("%d marks of kind %d, want 1", len(ms), kind)
	}
	return ms[0]
}

// glyphNamed is the first Glyphs mark whose text, without its spaces, is s.
func glyphNamed(t *testing.T, tr *Trace, s string) Mark {
	t.Helper()
	for _, m := range marks(tr, MarkGlyphs) {
		if strings.TrimSpace(m.Text) == s {
			return m
		}
	}
	t.Fatalf("no Glyphs mark with text %q", s)
	return Mark{}
}

// numberNamed is the first Number mark whose text is s, failing the test when there is none.
func numberNamed(t *testing.T, tr *Trace, s string) Mark {
	t.Helper()
	ms := textMarks(tr, MarkNumber, s)
	if len(ms) == 0 {
		t.Fatalf("no Number mark with text %q", s)
	}
	return ms[0]
}

// TestLeadsWithText: a block leads with a text line when its first line is text, found through
// quotes and nested lists. An empty item is an empty line of text. A block whose first line is
// not text (a rule, a table, an image, an empty quote or list) does not, and a block with no
// layout is an error naming it (STYLE-LIST-2).
func TestLeadsWithText(t *testing.T) {
	tests := []struct {
		name  string
		block doc.Block
		want  bool
	}{
		{"paragraph", doc.Paragraph{}, true},
		{"heading", doc.Heading{Level: 1}, true},
		{"code block", doc.CodeBlock{}, true},
		{"quote of a paragraph", doc.Quote{Blocks: []doc.Block{doc.Paragraph{}}}, true},
		{"list whose first item is empty", doc.List{Items: []doc.Item{{}}}, true},
		{"rule", doc.Rule{}, false},
		{"table", doc.Table{}, false},
		{"image", doc.Image{}, false},
		{"empty quote", doc.Quote{}, false},
		{"quote of a rule", doc.Quote{Blocks: []doc.Block{doc.Rule{}}}, false},
		{"empty list", doc.List{}, false},
		{"list whose first item starts with a rule", doc.List{Items: []doc.Item{{Blocks: []doc.Block{doc.Rule{}}}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := leadsWithText(tt.block)
			if err != nil {
				t.Fatalf("leadsWithText: %v", err)
			}
			if got != tt.want {
				t.Errorf("leadsWithText = %t, want %t", got, tt.want)
			}
		})
	}
	errs := []struct {
		name  string
		block doc.Block
		want  string
	}{
		{"nil block", nil, "<nil>"},
		{"pointer to paragraph", &doc.Paragraph{}, "*doc.Paragraph"},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := leadsWithText(tt.block)
			if err == nil {
				t.Fatalf("leadsWithText(%s): nil error, want one naming %s", tt.name, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name %s", err.Error(), tt.want)
			}
		})
	}
}

// TestListIndent: an item's content starts 32 dp right of the list's left edge (STYLE-LIST-1), and
// a nested list indents again from its item's content edge. A list in a quote starts past the
// quote's 20 dp inset. A wrapped line hangs at the content edge.
func TestListIndent(t *testing.T) {
	tests := []struct {
		name, src, text string
		x               int
	}{
		{"one level", "- a", "a", 77},
		{"nested", "- a\n  - b", "b", 109},
		{"nested twice", "1. a\n   1. b\n      1. c", "c", 141},
		{"in a quote", "> - a", "a", 97},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnv(t, tt.src, Light, metric1)
			_, tr := e.layout(0, 800)
			if got := glyphNamed(t, tr, tt.text).Rect.Min.X; got != tt.x {
				t.Errorf("%q starts at x=%d, want %d", tt.text, got, tt.x)
			}
		})
	}

	e := newViewEnv(t, "- a", Light, metric1)
	if h := e.dims(0, 800).Size.Y; h != 114 {
		t.Errorf("- a is %d px tall, want 114 (45 padding, 24 line, 45 padding)", h)
	}

	// Items of 40 words at 300 px: the content starts at 15 + 32 = 47 px, and every line starts
	// there and stays inside the right padding at 285 px.
	src := "- " + strings.TrimSpace(strings.Repeat("word ", 40))
	e = newViewEnv(t, src, Light, metric1)
	_, tr := e.layout(0, 300)
	gs := marks(tr, MarkGlyphs)
	starts := lineStarts(gs)
	if len(starts) < 2 {
		t.Fatalf("%d lines at 300 px, want at least 2", len(starts))
	}
	for _, s := range starts {
		if s.Rect.Min.X != 47 {
			t.Errorf("line starting %q is at x=%d, want 47 (the hanging indent)", s.Text, s.Rect.Min.X)
		}
	}
	for _, m := range gs {
		if m.Rect.Min.X < 47 || m.Rect.Max.X > 285 {
			t.Errorf("glyphs %q span x %d to %d, want within 47 to 285", m.Text, m.Rect.Min.X, m.Rect.Max.X)
		}
	}
}

// TestBullets: the bullets of depth 0, 1 and 2 are a disc, a circle outline and a square
// (STYLE-LIST-2). Each is 5 dp, centred 11 dp left of the content edge, with its top-left corner at
// the rounded centre less half its size. Depth counts ordered lists too.
func TestBullets(t *testing.T) {
	t.Run("depths", func(t *testing.T) {
		e := newViewEnv(t, "- a\n  - b\n    - c\n      - d", Light, metric1)
		_, tr := e.layout(0, 800)
		want := []struct {
			rect image.Rectangle
			text string
		}{
			{image.Rect(64, 56, 69, 61), "disc"},
			{image.Rect(96, 80, 101, 85), "circle"},
			{image.Rect(128, 104, 133, 109), "square"},
			{image.Rect(160, 128, 165, 133), "square"},
		}
		if n := len(marks(tr, MarkBullet)); n != len(want) {
			t.Fatalf("%d bullets, want %d", n, len(want))
		}
		for _, w := range want {
			m := markAt(t, tr, MarkBullet, w.text, w.rect)
			if m.Colour != Light.Fg {
				t.Errorf("%s bullet at %v is %#v, want Light.Fg", w.text, w.rect, m.Colour)
			}
		}
	})

	t.Run("tight items", func(t *testing.T) {
		e := newViewEnv(t, "- a\n- b\n- c", Light, metric1)
		_, tr := e.layout(0, 800)
		for _, top := range []int{56, 84, 112} {
			markAt(t, tr, MarkBullet, "disc", image.Rect(64, top, 69, top+5))
		}
		if n := len(marks(tr, MarkBullet)); n != 3 {
			t.Fatalf("%d bullets, want 3", n)
		}
	})

	t.Run("in an ordered list", func(t *testing.T) {
		e := newViewEnv(t, "1. a\n   - b", Light, metric1)
		_, tr := e.layout(0, 800)
		m := onlyMark(t, tr, MarkBullet)
		if m.Text != "circle" || m.Rect != image.Rect(96, 80, 101, 85) {
			t.Errorf("bullet under an ordered item is %q at %v, want circle at (96,80)-(101,85)", m.Text, m.Rect)
		}
	})

	t.Run("dark", func(t *testing.T) {
		e := newViewEnv(t, "- a", Dark, metric1)
		_, tr := e.layout(0, 800)
		if m := onlyMark(t, tr, MarkBullet); m.Colour != Dark.Fg {
			t.Errorf("bullet is %#v, want Dark.Fg", m.Colour)
		}
	})

	t.Run("metric 2", func(t *testing.T) {
		e := newViewEnv(t, "- a", Light, metric2)
		_, tr := e.layout(0, 1600)
		m := onlyMark(t, tr, MarkBullet)
		if m.Text != "disc" || m.Rect != image.Rect(127, 112, 137, 122) {
			t.Errorf("bullet at 2 px per dp is %q at %v, want disc at (127,112)-(137,122)", m.Text, m.Rect)
		}
	})
}

// TestOrderedNumbers: a number is its text right-aligned 4 dp left of the content edge (STYLE-
// LIST-3), with the numbering of its depth. Numbers are not bullets.
func TestOrderedNumbers(t *testing.T) {
	t.Run("start at 3", func(t *testing.T) {
		e := newViewEnv(t, "3. a\n4. b", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkNumber, "3.", image.Rect(59, 47, 73, 67))
		markAt(t, tr, MarkNumber, "4.", image.Rect(59, 75, 73, 95))
		if n := len(marks(tr, MarkNumber)); n != 2 {
			t.Fatalf("%d numbers, want 2", n)
		}
		if m := numberNamed(t, tr, "3."); m.Colour != Light.Fg {
			t.Errorf("number is %#v, want Light.Fg", m.Colour)
		}
	})

	t.Run("no bullets", func(t *testing.T) {
		e := newViewEnv(t, "1. a\n2. b", Light, metric1)
		_, tr := e.layout(0, 800)
		if n := len(marks(tr, MarkBullet)); n != 0 {
			t.Errorf("%d bullets in an ordered list, want none", n)
		}
	})

	t.Run("nested styles", func(t *testing.T) {
		e := newViewEnv(t, "1. a\n   1. b\n      1. c", Light, metric1)
		_, tr := e.layout(0, 800)
		for _, w := range []struct {
			text string
			maxX int
		}{{"1.", 73}, {"i.", 105}, {"a.", 137}} {
			if got := numberNamed(t, tr, w.text).Rect.Max.X; got != w.maxX {
				t.Errorf("%q ends at x=%d, want %d", w.text, got, w.maxX)
			}
		}
	})

	t.Run("right aligned", func(t *testing.T) {
		e := newViewEnv(t, "8. a\n9. b\n10. c", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkNumber, "8.", image.Rect(59, 47, 73, 67))
		markAt(t, tr, MarkNumber, "9.", image.Rect(59, 75, 73, 95))
		markAt(t, tr, MarkNumber, "10.", image.Rect(50, 103, 73, 123))
	})

	t.Run("zero", func(t *testing.T) {
		e := newViewEnv(t, "0. a", Light, metric1)
		_, tr := e.layout(0, 800)
		if m := onlyMark(t, tr, MarkNumber); m.Text != "0." {
			t.Errorf("number is %q, want %q", m.Text, "0.")
		}
	})

	t.Run("beyond roman", func(t *testing.T) {
		// A list that does not start at 1 cannot interrupt a paragraph, so the blank line is needed.
		e := newViewEnv(t, "1. x\n\n   4000. a", Light, metric1)
		_, tr := e.layout(0, 800)
		numberNamed(t, tr, "4000.")
	})

	t.Run("dark", func(t *testing.T) {
		e := newViewEnv(t, "3. a", Dark, metric1)
		_, tr := e.layout(0, 800)
		if m := onlyMark(t, tr, MarkNumber); m.Colour != Dark.Fg {
			t.Errorf("number is %#v, want Dark.Fg", m.Colour)
		}
	})
}

// TestTightAndLooseSpacing: consecutive items of a tight list are 4 dp apart, and of a loose
// list 16 dp (STYLE-LIST-4). The paragraphs and images of a tight item have no margins; a loose
// item keeps its paragraph margins. Distances are between the tops of glyphs.
func TestTightAndLooseSpacing(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		texts  []string // the text of each line whose top is compared, in order
		gaps   []int    // the distance between each pair of consecutive tops
		elem   int      // the element whose height is checked
		height int      // that element's height, or 0 for no check
	}{
		{"tight items", "- a\n- b", []string{"a", "b"}, []int{28}, 0, 142},
		{"loose items", "- a\n\n- b", []string{"a", "b"}, []int{40}, 0, 154},
		{"loose item with two paragraphs", "- a\n\n  b", []string{"a", "b"}, []int{40}, 0, 0},
		{"tight ordered items", "1. a\n2. b", []string{"a", "b"}, []int{28}, 0, 0},
		{"tight item with a nested list", "- a\n  - b\n- c", []string{"a", "b", "c"}, []int{24, 28}, 0, 0},
		{"loose item with a nested list", "- a\n\n  - b\n- c", []string{"a", "b", "c"}, []int{40, 40}, 0, 0},
		{"loose list then paragraph", "- a\n- b\n\nc", nil, nil, 1, 85},
		{"paragraph then list", "a\n\n- b", nil, nil, 1, 85},
		{"image in a tight item", "- a ![i](x.png) b", []string{"a", "b"}, []int{24}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnv(t, tt.src, Light, metric1)
			_, tr := e.layout(tt.elem, 800)
			if tt.height != 0 {
				if got := e.dims(tt.elem, 800).Size.Y; got != tt.height {
					t.Errorf("element %d is %d px tall, want %d", tt.elem, got, tt.height)
				}
			}
			tops := make([]int, len(tt.texts))
			for i, s := range tt.texts {
				tops[i] = glyphNamed(t, tr, s).Rect.Min.Y
			}
			for i, want := range tt.gaps {
				if got := tops[i+1] - tops[i]; got != want {
					t.Errorf("%q to %q is %d px, want %d", tt.texts[i], tt.texts[i+1], got, want)
				}
			}
		})
	}
}

// TestTaskBoxes: a task item draws a box and no bullet or number (STYLE-LIST-5). An open box is
// a Canvas fill with a FgMuted border; a done box is an Accent fill with an OnAccent check mark.
// The box's left edge is 22 dp left of the content edge, centred on the first line's baseline
// less 0.35 em, and it is 13 dp square.
func TestTaskBoxes(t *testing.T) {
	palettes := []struct {
		name string
		pal  Palette
	}{{"light", Light}, {"dark", Dark}}
	for _, p := range palettes {
		t.Run(p.name, func(t *testing.T) {
			e := newViewEnv(t, "- [ ] a\n- [x] b", p.pal, metric1)
			_, tr := e.layout(0, 800)
			open := markAt(t, tr, MarkTaskBox, "open", image.Rect(55, 51, 68, 64))
			if open.Colour != p.pal.Canvas {
				t.Errorf("open box is %#v, want Canvas %#v", open.Colour, p.pal.Canvas)
			}
			border := markAt(t, tr, MarkTaskBox, "border", image.Rect(55, 51, 68, 64))
			if border.Colour != p.pal.FgMuted {
				t.Errorf("border is %#v, want FgMuted %#v", border.Colour, p.pal.FgMuted)
			}
			done := markAt(t, tr, MarkTaskBox, "done", image.Rect(55, 79, 68, 92))
			if done.Colour != p.pal.Accent {
				t.Errorf("done box is %#v, want Accent %#v", done.Colour, p.pal.Accent)
			}
			check := markAt(t, tr, MarkTaskBox, "check", image.Rect(57, 82, 66, 89))
			if check.Colour != p.pal.OnAccent {
				t.Errorf("check mark is %#v, want OnAccent %#v", check.Colour, p.pal.OnAccent)
			}
			if n := len(textMarks(tr, MarkTaskBox, "check")); n != 1 {
				t.Errorf("%d check marks, want 1 (none on the open box)", n)
			}
			if n := len(textMarks(tr, MarkTaskBox, "border")); n != 1 {
				t.Errorf("%d border marks, want 1 (none on the done box)", n)
			}
			if n := len(marks(tr, MarkBullet)); n != 0 {
				t.Errorf("%d bullets on task items, want none", n)
			}
			if got := glyphNamed(t, tr, "a").Rect.Min.X; got != 77 {
				t.Errorf("task text starts at x=%d, want 77", got)
			}
		})
	}

	t.Run("bullet beside a task", func(t *testing.T) {
		e := newViewEnv(t, "- a\n- [x] b", Light, metric1)
		_, tr := e.layout(0, 800)
		onlyMark(t, tr, MarkBullet)
		onlyMark(t, tr, MarkTaskBox)
	})

	t.Run("number beside a task", func(t *testing.T) {
		e := newViewEnv(t, "1. [ ] a\n2. b", Light, metric1)
		_, tr := e.layout(0, 800)
		m := onlyMark(t, tr, MarkNumber)
		if m.Text != "2." {
			t.Errorf("number is %q, want %q: a task still counts", m.Text, "2.")
		}
	})

	t.Run("empty task", func(t *testing.T) {
		e := newViewEnv(t, "- [ ]", Light, metric1)
		dims, tr := e.layout(0, 800)
		markAt(t, tr, MarkTaskBox, "open", image.Rect(55, 51, 68, 64))
		if dims.Size.Y != 114 {
			t.Errorf("- [ ] is %d px tall, want 114", dims.Size.Y)
		}
	})
}

// TestListMarkerBaseline: a marker sits on the first text line of its item's first block, found
// through headings, quotes, nested lists and code blocks (STYLE-LIST-2, STYLE-LIST-3). With no
// text line it sits on an empty body line at the item's top, and the item's height is unchanged.
func TestListMarkerBaseline(t *testing.T) {
	t.Run("heading", func(t *testing.T) {
		e := newViewEnv(t, "- # h", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "disc", image.Rect(64, 70, 69, 75))
	})

	t.Run("number above a heading", func(t *testing.T) {
		e := newViewEnv(t, "1. # h", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkNumber, "1.", image.Rect(59, 61, 73, 81))
	})

	t.Run("code block", func(t *testing.T) {
		e := newViewEnv(t, "- ```\n  x\n  ```", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "disc", image.Rect(64, 69, 69, 74))
	})

	t.Run("quote", func(t *testing.T) {
		e := newViewEnv(t, "- > q", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "disc", image.Rect(64, 56, 69, 61))
	})

	t.Run("nested list", func(t *testing.T) {
		e := newViewEnv(t, "- - a", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "disc", image.Rect(64, 56, 69, 61))
		markAt(t, tr, MarkBullet, "circle", image.Rect(96, 56, 101, 61))
	})

	t.Run("empty item", func(t *testing.T) {
		e := newViewEnv(t, "-", Light, metric1)
		dims, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "disc", image.Rect(64, 56, 69, 61))
		if dims.Size.Y != 114 {
			t.Errorf("- is %d px tall, want 114", dims.Size.Y)
		}
	})

	t.Run("rule first", func(t *testing.T) {
		list := doc.List{Tight: true, Start: 1, Items: []doc.Item{{Blocks: []doc.Block{doc.Rule{}}}}}
		e := newViewEnvDoc(t, &doc.Document{Blocks: []doc.Block{list}}, Light, metric1)
		dims, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "disc", image.Rect(64, 56, 69, 61))
		if dims.Size.Y != 94 {
			t.Errorf("a list of a rule is %d px tall, want 94 (45 padding, 4 rule, 45 padding)", dims.Size.Y)
		}
	})
}

// TestListInQuote: a list in a quote is drawn in the quote's muted text (STYLE-QUOTE-1). Task
// boxes keep their own colours. A list inside a quote inside an item is nested, with no margins
// between it and the quote's paragraph (STYLE-LIST-4).
func TestListInQuote(t *testing.T) {
	t.Run("markers are muted", func(t *testing.T) {
		e := newViewEnv(t, "> - a\n>\n> 1. b", Light, metric1)
		_, tr := e.layout(0, 800)
		if m := markAt(t, tr, MarkBullet, "disc", image.Rect(84, 56, 89, 61)); m.Colour != Light.FgMuted {
			t.Errorf("bullet in a quote is %#v, want FgMuted", m.Colour)
		}
		if m := numberNamed(t, tr, "1."); m.Colour != Light.FgMuted || m.Rect.Max.X != 93 {
			t.Errorf("number in a quote is %#v ending at x=%d, want FgMuted ending at 93", m.Colour, m.Rect.Max.X)
		}
		if m := glyphNamed(t, tr, "a"); m.Colour != Light.FgMuted {
			t.Errorf("item text in a quote is %#v, want FgMuted", m.Colour)
		}
	})

	t.Run("boxes keep their colours", func(t *testing.T) {
		e := newViewEnv(t, "> - [ ] a\n> - [x] b", Light, metric1)
		_, tr := e.layout(0, 800)
		open := markAt(t, tr, MarkTaskBox, "open", image.Rect(75, 51, 88, 64))
		if open.Colour != Light.Canvas {
			t.Errorf("open box in a quote is %#v, want Canvas", open.Colour)
		}
		if m := markAt(t, tr, MarkTaskBox, "border", image.Rect(75, 51, 88, 64)); m.Colour != Light.FgMuted {
			t.Errorf("border in a quote is %#v, want FgMuted", m.Colour)
		}
		if m := markAt(t, tr, MarkTaskBox, "done", image.Rect(75, 79, 88, 92)); m.Colour != Light.Accent {
			t.Errorf("done box in a quote is %#v, want Accent", m.Colour)
		}
		if m := markAt(t, tr, MarkTaskBox, "check", image.Rect(77, 82, 86, 89)); m.Colour != Light.OnAccent {
			t.Errorf("check in a quote is %#v, want OnAccent", m.Colour)
		}
	})

	t.Run("quote in an item", func(t *testing.T) {
		e := newViewEnv(t, "- a\n  > - b", Light, metric1)
		_, tr := e.layout(0, 800)
		markAt(t, tr, MarkBullet, "circle", image.Rect(116, 80, 121, 85))
	})

	t.Run("no margins in a quote in an item", func(t *testing.T) {
		e := newViewEnv(t, "- a\n  > - b\n  >\n  > c", Light, metric1)
		_, tr := e.layout(0, 800)
		if gap := glyphNamed(t, tr, "c").Rect.Min.Y - glyphNamed(t, tr, "b").Rect.Min.Y; gap != 24 {
			t.Errorf("c is %d px below b, want 24 (the nested list has no margin)", gap)
		}
	})
}

// TestListTinyWidth: a list lays out at every width, from negative to narrow, at three metrics,
// with no layout error, and every item gets its marker (STYLE-LIST-2, STYLE-LIST-5). A width too
// narrow for its text is a degenerate window, not a reason to fail.
func TestListTinyWidth(t *testing.T) {
	src := strings.Join([]string{"- a", "  - b", "1. c", "- [x] d", "-"}, "\n")
	widths := []int{-5, 0, 1, 2, 5, 10, 31, 33, 40, 77}
	metrics := []unit.Metric{
		{PxPerDp: 1, PxPerSp: 1},
		{PxPerDp: 1.5, PxPerSp: 1.5},
		{PxPerDp: 2.25, PxPerSp: 2.25},
	}
	for _, m := range metrics {
		e := newViewEnv(t, src, Light, m)
		for _, w := range widths {
			for i, b := range e.d.Blocks {
				_, tr := e.layout(i, w)
				if got, want := len(markerMarks(tr)), countItems(t, b); got != want {
					t.Errorf("metric %v, width %d, block %d: %d markers, want %d (one per item)", m.PxPerDp, w, i, got, want)
				}
			}
		}
	}
}
