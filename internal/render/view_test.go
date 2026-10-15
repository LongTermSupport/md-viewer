package render

import (
	"image"
	"os"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// viewEnv is a headless View (ARCHITECTURE "Headless render tests"): one op list and one input
// router, at one metric. Each call makes its own context at the width it is given.
type viewEnv struct {
	t      *testing.T
	ops    op.Ops
	router input.Router
	d      *doc.Document
	view   *View
	metric unit.Metric
}

// metric1 is 1 px per dp and per sp, the metric most tests use.
var metric1 = unit.Metric{PxPerDp: 1, PxPerSp: 1}

// metric2 is 2 px per dp and per sp, a high-density screen.
var metric2 = unit.Metric{PxPerDp: 2, PxPerSp: 2}

// newViewEnv parses src as a document in /doc and makes a View of it on palette pal.
func newViewEnv(t *testing.T, src string, pal Palette, metric unit.Metric) *viewEnv {
	t.Helper()
	d, err := doc.Parse([]byte(src), "/doc")
	if err != nil {
		t.Fatalf("doc.Parse: %v", err)
	}
	return newViewEnvDoc(t, d, pal, metric)
}

// newViewEnvDoc makes a View of d, with the faces loaded as the window loads them.
func newViewEnvDoc(t *testing.T, d *doc.Document, pal Palette, metric unit.Metric) *viewEnv {
	t.Helper()
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	return &viewEnv{t: t, d: d, view: NewView(d, NewShaper(faces), pal), metric: metric}
}

// elementsEnv is a View of testdata/elements.md, the document that shows every block kind.
func elementsEnv(t *testing.T, metric unit.Metric) *viewEnv {
	t.Helper()
	src, err := os.ReadFile("../../testdata/elements.md")
	if err != nil {
		t.Fatalf("read elements.md: %v", err)
	}
	return newViewEnv(t, string(src), Light, metric)
}

// context resets the ops and returns a context of width px for the env's metric.
func (e *viewEnv) context(width int) layout.Context {
	e.ops.Reset()
	return layout.Context{
		Ops:         &e.ops,
		Source:      e.router.Source(),
		Metric:      e.metric,
		Constraints: layout.Constraints{Max: image.Pt(width, 1<<20)},
	}
}

// layout lays out element i at width px into a new trace, and fails the test on a layout error.
func (e *viewEnv) layout(i, width int) (layout.Dimensions, *Trace) {
	e.t.Helper()
	tr := &Trace{}
	dims := e.view.Layout(e.context(width), i, tr)
	if err := e.view.Err(); err != nil {
		e.t.Fatalf("element %d at %d px: %v", i, width, err)
	}
	return dims, tr
}

// dims lays out element i at width px without a trace, and fails the test on a layout error.
func (e *viewEnv) dims(i, width int) layout.Dimensions {
	e.t.Helper()
	dims := e.view.Layout(e.context(width), i, nil)
	if err := e.view.Err(); err != nil {
		e.t.Fatalf("element %d at %d px: %v", i, width, err)
	}
	return dims
}

// frame is layout followed by one router frame, so the link gestures made in it take the events
// queued after it (as flowEnv.frame does).
func (e *viewEnv) frame(i, width int) *Trace {
	e.t.Helper()
	_, tr := e.layout(i, width)
	e.router.Frame(&e.ops)
	return tr
}

// poll is one frame's click poll, made before that frame lays out, as the window makes it.
func (e *viewEnv) poll() (doc.Target, bool) {
	return e.view.Clicked(e.context(800))
}

// TestViewLen: a View has one element per top-level block of its document; an empty document has
// none.
func TestViewLen(t *testing.T) {
	e := elementsEnv(t, metric1)
	if n := e.view.Len(); n != len(e.d.Blocks) || n <= 20 {
		t.Fatalf("Len() = %d, want %d (the top-level blocks of elements.md, more than 20)", n, len(e.d.Blocks))
	}
	empty := newViewEnv(t, "", Light, metric1)
	if n := empty.view.Len(); n != 0 {
		t.Fatalf("empty document: Len() = %d, want 0", n)
	}
}

// TestViewFirstAndLastPadding: the first block starts 45 dp down at 768 dp and wider, and 15 dp
// under that (DOC-LAYOUT-1); the last block ends with the same padding below it. Padding moves the
// content and nothing else.
func TestViewFirstAndLastPadding(t *testing.T) {
	rows := []struct {
		width  int
		metric unit.Metric
		pad    int
	}{
		{800, metric1, 45},
		{768, metric1, 45},
		{767, metric1, 15},
		{700, metric1, 15},
		{1600, metric2, 90},
		{1534, metric2, 30},
	}
	// The first block's offset below the padding, per pixel-per-dp: it is the same at every width.
	offsets := map[float32]int{}
	for _, row := range rows {
		e := elementsEnv(t, row.metric)
		first, tr := e.layout(0, row.width)
		gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
		if gs[0].Rect.Min.X != row.pad || gs[0].Rect.Min.Y < row.pad {
			t.Fatalf("width %d: first glyphs at %v, want x %d and y at least %d", row.width, gs[0].Rect.Min, row.pad, row.pad)
		}
		off := gs[0].Rect.Min.Y - row.pad
		if want, ok := offsets[row.metric.PxPerDp]; ok && off != want {
			t.Fatalf("width %d: first glyphs %d px below the padding, want %d as at the other widths", row.width, off, want)
		}
		offsets[row.metric.PxPerDp] = off
		if first.Size.X != row.width {
			t.Fatalf("width %d: element 0 is %d px wide", row.width, first.Size.X)
		}

		_, tr1 := e.layout(1, row.width)
		for _, m := range needMarks(t, tr1, MarkGlyphs, "Glyphs") {
			if m.Rect.Min.X < row.pad || m.Rect.Max.X > row.width-row.pad {
				t.Fatalf("width %d: element 1 glyphs %q span x %d to %d, want within %d to %d", row.width, m.Text, m.Rect.Min.X, m.Rect.Max.X, row.pad, row.width-row.pad)
			}
		}

		// The last element is "The end." (one 24 px line at 16 sp x 1.5) under a 16 dp gap and
		// the bottom padding.
		last, lastTr := e.layout(e.view.Len()-1, row.width)
		needMarks(t, lastTr, MarkGlyphs, "Glyphs")
		gapPx := int(row.metric.PxPerDp) * 16
		linePx := int(row.metric.PxPerSp) * 24
		if want := gapPx + linePx + row.pad; last.Size.Y != want {
			t.Fatalf("width %d: last element is %d px tall, want %d (gap %d + line %d + padding %d)", row.width, last.Size.Y, want, gapPx, linePx, row.pad)
		}
	}
}

// TestViewGaps: the space between two blocks is the larger of the first's bottom margin and the
// second's top margin (DOC-LAYOUT-3). An element is its top gap and its content, so the gap is
// the middle element's height less its content's height, which a one-block document gives.
// The first element has no bottom margin inside it.
func TestViewGaps(t *testing.T) {
	tests := []struct {
		name       string
		prev, next string
		gap        int
	}{
		{"paragraph then paragraph", "a", "b", 16},
		{"paragraph then heading", "a", "### b", 24},
		{"heading then paragraph", "### a", "b", 16},
		{"heading then heading", "### a", "### b", 24},
		{"paragraph then rule", "a", "---", 24},
		{"rule then paragraph", "---", "b", 24},
		{"paragraph then fenced code", "a", "```\nc\n```", 16},
		{"paragraph then quote", "a", "> q", 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnv(t, tt.prev+"\n\n"+tt.next+"\n\nz", Light, metric1)
			ref := newViewEnv(t, tt.next, Light, metric1).dims(0, 800)
			content := ref.Size.Y - 2*45 // the one-block element is 45 px, the content, 45 px
			got := e.dims(1, 800).Size.Y - content
			if got != tt.gap {
				t.Fatalf("%q then %q: gap %d px, want %d", tt.prev, tt.next, got, tt.gap)
			}
		})
	}

	e := newViewEnv(t, "a\n\n### b\n\nz", Light, metric1)
	if h := e.dims(0, 800).Size.Y; h != 45+24 {
		t.Fatalf("element 0 of a, ### b, z is %d px tall, want %d (45 padding + 24 line, no bottom margin)", h, 45+24)
	}
}

// TestNoMaxWidth: the content is as wide as the window less the padding, with no 980 px column
// (DOC-LAYOUT-2): at 2000 px wide a long paragraph reaches past 980 px and not past 1955 px.
func TestNoMaxWidth(t *testing.T) {
	src := strings.TrimSpace(strings.Repeat("word ", 60))
	e := newViewEnv(t, src, Light, metric1)
	_, tr := e.layout(0, 2000)
	reach := 0
	for _, m := range needMarks(t, tr, MarkGlyphs, "Glyphs") {
		reach = max(reach, m.Rect.Max.X)
	}
	if reach <= 980 || reach > 1955 {
		t.Fatalf("paragraph reaches x=%d at 2000 px wide, want past 980 and at most 1955", reach)
	}
}

// TestViewPlaceholders: a list, a table and an image are height 0 until M6 to M8, with their
// margins (DOC-LAYOUT-4) and no marks.
func TestViewPlaceholders(t *testing.T) {
	tests := []struct {
		name, block string
	}{
		{"list", "- item"},
		{"table", "| x |\n| - |\n| y |"},
		{"image", "![alt](i.png)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnv(t, "a\n\n"+tt.block+"\n\nb", Light, metric1)
			if n := e.view.Len(); n != 3 {
				t.Fatalf("%d elements, want 3", n)
			}
			mid, tr := e.layout(1, 800)
			if mid.Size.Y != 16 {
				t.Fatalf("placeholder is %d px tall with its gap, want 16 (its top gap is 16 and its height 0)", mid.Size.Y)
			}
			if len(tr.Marks) != 0 {
				t.Fatalf("placeholder recorded %d marks, want none", len(tr.Marks))
			}
			if last := e.dims(2, 800).Size.Y; last != 16+24+45 {
				t.Fatalf("element after the placeholder is %d px tall, want %d (gap 16, line 24, padding 45)", last, 16+24+45)
			}
		})
	}
}

// TestViewLayoutError: a layout error is kept as the View's first error, which Err reports. The
// failing element is Dimensions{}, and a later layout does not clear the error (ARCHITECTURE,
// View).
func TestViewLayoutError(t *testing.T) {
	tests := []struct {
		name  string
		block doc.Block
		want  string
	}{
		{"heading level 7", doc.Heading{Level: 7}, "level 7"},
		{"nil block", nil, "<nil>"},
		{"heading level 0 in a quote", doc.Quote{Blocks: []doc.Block{doc.Heading{Level: 0}}}, "level 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnvDoc(t, &doc.Document{Blocks: []doc.Block{tt.block}}, Light, metric1)
			e.view.Layout(e.context(800), 0, nil)
			if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Err() = %v, want an error containing %q", err, tt.want)
			}
		})
	}

	e := newViewEnvDoc(t, &doc.Document{Blocks: []doc.Block{
		doc.Heading{Level: 7},
		doc.Heading{Level: 8},
		doc.Paragraph{Runs: []doc.Run{plain("ok")}},
	}}, Light, metric1)
	e.view.Layout(e.context(800), 0, nil)
	e.view.Layout(e.context(800), 1, nil)
	e.view.Layout(e.context(800), 2, nil)
	if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), "level 7") || strings.Contains(err.Error(), "level 8") {
		t.Fatalf("after two failures and a valid element: Err() = %v, want the first error (level 7)", err)
	}

	e = newViewEnvDoc(t, &doc.Document{Blocks: []doc.Block{doc.Paragraph{Runs: []doc.Run{plain("a")}}}}, Light, metric1)
	e.view.Layout(e.context(800), e.view.Len(), nil)
	if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("element 1 of a one-block document: Err() = %v, want an error naming index 1", err)
	}

	e = newViewEnvDoc(t, &doc.Document{Blocks: []doc.Block{doc.Paragraph{Runs: []doc.Run{plain("a")}}}}, Light, metric1)
	e.layout(0, 800)
}

// TestViewNilTrace: a nil trace changes no dimensions, and every element is taller than 0.
func TestViewNilTrace(t *testing.T) {
	e := elementsEnv(t, metric1)
	if n := e.view.Len(); n != len(e.d.Blocks) {
		t.Fatalf("Len() = %d, want %d: the loop must reach every element", n, len(e.d.Blocks))
	}
	for i := 0; i < e.view.Len(); i++ {
		want := e.dims(i, 800)
		got, tr := e.layout(i, 800)
		if tr == nil {
			t.Fatalf("element %d: layout with a trace returned no trace", i)
		}
		if got != want {
			t.Fatalf("element %d: %+v with a trace, %+v without", i, got, want)
		}
		if want.Size.Y <= 0 {
			t.Fatalf("element %d is %d px tall, want more than 0", i, want.Size.Y)
		}
	}
}

// TestViewClicked: a link in the View is clicked as in a flow, and its area is in the element's
// coordinates, so it starts at the 45 px padding (LINK-1, STYLE-TEXT-3).
func TestViewClicked(t *testing.T) {
	tg := doc.Target{Kind: doc.External, Value: "https://e.org/"}
	e := newViewEnv(t, "[l](https://e.org/)", Light, metric1)
	// Frame 1 lays out, which makes the link's gesture. Frame 2 polls before it lays out, as
	// every frame does, so the gesture takes the events queued after it (TestFlowLinkClick).
	e.frame(0, 800)
	if early, polled := e.poll(); polled {
		t.Fatalf("a poll with no input reported %+v", early)
	}
	tr := e.frame(0, 800)
	la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
	if la.Rect.Min.X != 45 {
		t.Fatalf("link area starts at x=%d, want 45 (the element's padding is in its trace)", la.Rect.Min.X)
	}
	e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))

	got, ok := e.poll()
	if !ok || got != tg {
		t.Fatalf("poll after the click = %+v, %t; want the link's target and true", got, ok)
	}
	if again, more := e.poll(); more {
		t.Fatalf("second poll in the same frame reported %+v again", again)
	}
	e.frame(0, 800)
	if later, more := e.poll(); more {
		t.Fatalf("the poll after that frame reported %+v again", later)
	}
}

// TestViewElements: every element of elements.md lays out 800 px wide with no error, and each
// block kind that is drawn so far has its marks: text in headings, paragraphs and quotes, the
// background of a code block and the bar of a rule.
func TestViewElements(t *testing.T) {
	e := elementsEnv(t, metric1)
	for i, b := range e.d.Blocks {
		dims, tr := e.layout(i, 800)
		if dims.Size.X != 800 {
			t.Fatalf("element %d is %d px wide, want 800", i, dims.Size.X)
		}
		switch b := b.(type) {
		case doc.Heading, doc.Paragraph, doc.Quote:
			needMarks(t, tr, MarkGlyphs, "Glyphs")
		case doc.CodeBlock:
			needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		case doc.Rule:
			needMarks(t, tr, MarkRuleBar, "RuleBar")
		case doc.List, doc.Table, doc.Image:
			// Placeholders until M6 to M8: height 0 with their margins.
		default:
			t.Fatalf("element %d: no expectation for block %T", i, b)
		}
	}
}
