package render

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// viewEnv is a headless View (ARCHITECTURE "Headless render tests"): one op list and one input
// router, at one metric. Each call makes its own context at the width it is given. warn holds what
// the images printed to stderr, and images is the View's Images.
type viewEnv struct {
	t      *testing.T
	ops    op.Ops
	router input.Router
	d      *doc.Document
	view   *View
	metric unit.Metric
	warn   bytes.Buffer
	images *Images
}

// metric1 is 1 px per dp and per sp, the metric most tests use.
var metric1 = unit.Metric{PxPerDp: 1, PxPerSp: 1}

// metric2 is 2 px per dp and per sp, a high-density screen.
var metric2 = unit.Metric{PxPerDp: 2, PxPerSp: 2}

// newViewEnv parses src as a document in /doc and makes a View of it on palette pal.
func newViewEnv(t *testing.T, src string, pal Palette, metric unit.Metric) *viewEnv {
	t.Helper()
	return newViewEnvIn(t, src, "/doc", pal, metric)
}

// newViewEnvIn parses src as a document in dir, so that its relative image paths resolve there, and
// makes a View of it with its images probed by NewImages.
func newViewEnvIn(t *testing.T, src, dir string, pal Palette, metric unit.Metric) *viewEnv {
	t.Helper()
	return newViewEnvDoc(t, parseIn(t, src, dir), pal, metric)
}

// parseIn parses src as a document in dir.
func parseIn(t *testing.T, src, dir string) *doc.Document {
	t.Helper()
	d, err := doc.Parse([]byte(src), dir)
	if err != nil {
		t.Fatalf("doc.Parse: %v", err)
	}
	return d
}

// newViewEnvDoc makes a View of d, whose images NewImages probes.
func newViewEnvDoc(t *testing.T, d *doc.Document, pal Palette, metric unit.Metric) *viewEnv {
	t.Helper()
	return viewWith(t, d, pal, metric, func(w io.Writer) (*Images, error) {
		return NewImages(d, w)
	})
}

// newViewEnvNoImages makes a View of d with an empty Images. It is for a document that NewImages
// rejects (a block type its walk does not know), so that the layout reports the error instead.
func newViewEnvNoImages(t *testing.T, d *doc.Document, pal Palette, metric unit.Metric) *viewEnv {
	t.Helper()
	return viewWith(t, d, pal, metric, func(io.Writer) (*Images, error) {
		return &Images{}, nil
	})
}

// viewWith makes a View of d on palette pal, with the faces loaded as the window loads them. build
// makes the View's images, and is given the env's warn writer for their stderr lines.
func viewWith(t *testing.T, d *doc.Document, pal Palette, metric unit.Metric, build func(warn io.Writer) (*Images, error)) *viewEnv {
	t.Helper()
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	e := &viewEnv{t: t, d: d, metric: metric}
	img, err := build(&e.warn)
	if err != nil {
		t.Fatalf("images: %v", err)
	}
	e.images = img
	e.view = NewView(d, NewShaper(faces), pal, img)
	return e
}

// elementsEnv is a View of testdata/elements.md, the document that shows every block kind. Its
// images resolve in testdata.
func elementsEnv(t *testing.T, metric unit.Metric) *viewEnv {
	t.Helper()
	src, err := os.ReadFile("../../testdata/elements.md")
	if err != nil {
		t.Fatalf("read elements.md: %v", err)
	}
	dir, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	return newViewEnvIn(t, string(src), dir, Light, metric)
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
			e := newViewEnvNoImages(t, &doc.Document{Blocks: []doc.Block{tt.block}}, Light, metric1)
			if dims := e.view.Layout(e.context(800), 0, nil); dims != (layout.Dimensions{}) {
				t.Fatalf("failing element is %+v, want Dimensions{}", dims)
			}
			if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Err() = %v, want an error containing %q", err, tt.want)
			}
		})
	}

	e := newViewEnvNoImages(t, &doc.Document{Blocks: []doc.Block{
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

	e = newViewEnvNoImages(t, &doc.Document{Blocks: []doc.Block{doc.Paragraph{Runs: []doc.Run{plain("a")}}}}, Light, metric1)
	if dims := e.view.Layout(e.context(800), e.view.Len(), nil); dims != (layout.Dimensions{}) {
		t.Fatalf("element at a bad index is %+v, want Dimensions{}", dims)
	}
	if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("element 1 of a one-block document: Err() = %v, want an error naming index 1", err)
	}

	e = newViewEnvNoImages(t, &doc.Document{Blocks: []doc.Block{doc.Paragraph{Runs: []doc.Run{plain("a")}}}}, Light, metric1)
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
// coordinates: at the 45 px padding in a paragraph, and past the quote's 20 px inset (4 px bar and
// 16 px padding) in a quote (LINK-1, STYLE-TEXT-3, STYLE-QUOTE-1).
func TestViewClicked(t *testing.T) {
	tests := []struct {
		name, src string
		x         int
	}{
		{"paragraph", "[l](https://e.org/)", 45},
		{"quote", "> [l](https://e.org/)", 65},
		{"list", "- [l](https://e.org/)", 77},
		{"table", "| h |\n| - |\n| [l](https://e.org/) |", 59},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := doc.Target{Kind: doc.External, Value: "https://e.org/", Dest: "https://e.org/"}
			e := newViewEnv(t, tt.src, Light, metric1)
			// Frame 1 lays out, which makes the link's gesture. Frame 2 polls before it lays out, as
			// every frame does, so the gesture takes the events queued after it (TestFlowLinkClick).
			e.frame(0, 800)
			if early, polled := e.poll(); polled {
				t.Fatalf("a poll with no input reported %+v", early)
			}
			tr := e.frame(0, 800)
			la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
			if la.Rect.Min.X != tt.x {
				t.Fatalf("link area starts at x=%d, want %d (the element's padding and any quote inset are in its trace)", la.Rect.Min.X, tt.x)
			}
			e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))

			got, ok := e.poll()
			if !ok || got != want {
				t.Fatalf("poll after the click = %+v, %t; want %+v and true", got, ok, want)
			}
			if again, more := e.poll(); more {
				t.Fatalf("second poll in the same frame reported %+v again", again)
			}
			e.frame(0, 800)
			if later, more := e.poll(); more {
				t.Fatalf("the poll after that frame reported %+v again", later)
			}
		})
	}
}

// TestViewClickedClipped: a link in a table column that the table clips is not clickable: a press
// and release at the centre of its area report no click. Three columns of at least 64 dp each in a
// 100 px window put the third column past the window's edge (STYLE-TABLE-6, LINK-1). This pins the
// current behaviour, which the table's clip gives to a link's input as well as to its drawing.
func TestViewClickedClipped(t *testing.T) {
	e := newViewEnv(t, "| a | b | c |\n| - | - | - |\n| 1 | 2 | [l](https://e.org/) |", Light, metric1)
	// Frame 1 lays out, which makes the link's gesture. Frame 2 polls before it lays out (TestViewClicked).
	e.frame(0, 100)
	if early, polled := e.poll(); polled {
		t.Fatalf("a poll with no input reported %+v", early)
	}
	tr := e.frame(0, 100)
	la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
	if la.Rect.Min.X < 100 {
		t.Fatalf("link area starts at x=%d, want past the window's 100 px, so that its column is clipped", la.Rect.Min.X)
	}
	e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))
	if got, ok := e.poll(); ok {
		t.Fatalf("a click on a link in a clipped column reported %+v", got)
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
		case doc.List:
			needMarks(t, tr, MarkGlyphs, "Glyphs")
			if got, want := len(markerMarks(tr)), countItems(t, b); got != want {
				t.Fatalf("element %d: %d list markers, want %d (one per item)", i, got, want)
			}
		case doc.Table:
			// Lines: m + 1 vertical and n + 2 horizontal (STYLE-TABLE-1). Fills: n + 1 (STYLE-TABLE-2, STYLE-TABLE-3).
			if got, want := len(vlines(tr))+len(hlines(tr)), (len(b.Align)+1)+(len(b.Rows)+2); got != want {
				t.Fatalf("element %d: %d table lines, want %d", i, got, want)
			}
			needCount(t, "row fills", fills(tr), len(b.Rows)+1)
			needMarks(t, tr, MarkGlyphs, "Glyphs")
		case doc.Image:
			// A missing or remote image is a placeholder. A local file that exists is its box or its
			// pixels, and the two references to red-4x3.png may differ, as the decode runs meanwhile.
			ph := len(marks(tr, MarkPlaceholder))
			shown := len(marks(tr, MarkImageBox)) + len(marks(tr, MarkImagePixels))
			if b.Target.Kind != doc.LocalFile || b.Dest == "img/missing.png" {
				needCount(t, "placeholders", marks(tr, MarkPlaceholder), 1)
				if shown != 0 {
					t.Fatalf("element %d: %d image boxes or pixels for a placeholder, want none", i, shown)
				}
			} else {
				needCount(t, "image boxes or pixels", append(marks(tr, MarkImageBox), marks(tr, MarkImagePixels)...), 1)
				if ph != 0 {
					t.Fatalf("element %d: %d placeholders for a local image that exists, want none", i, ph)
				}
			}
			if b.Link != nil {
				needCount(t, "link areas", marks(tr, MarkLinkArea), 1)
			}
		default:
			t.Fatalf("element %d: no expectation for block %T", i, b)
		}
	}
}

// TestViewTinyWidth: a document with every drawn block kind lays out at widths from negative to
// narrow, at three metrics, with no panic and no layout error. A width too narrow for any text
// is a degenerate window, not a reason to fail (WRAP-1, STYLE-CODE-2).
func TestViewTinyWidth(t *testing.T) {
	src := strings.Join([]string{
		"# Heading",
		"",
		"A [link](https://e.org/) and `code`.",
		"",
		"```",
		"first",
		"",
		"    indented",
		"last",
		"```",
		"",
		"> ## Quoted heading",
		">",
		"> ```",
		"> quoted code",
		"> ```",
		">",
		"> > nested quote",
		"",
		"---",
		"",
		"- item",
	}, "\n")
	widths := []int{-5, 0, 1, 2, 5, 10, 31, 33, 40, 77}
	metrics := []unit.Metric{
		{PxPerDp: 1, PxPerSp: 1},
		{PxPerDp: 1.5, PxPerSp: 1.5},
		{PxPerDp: 2.25, PxPerSp: 2.25},
	}
	// Images: a local PNG, and a missing image's placeholder (STYLE-IMG-2, STYLE-IMG-3).
	src += "\n\n![a](red-4x3.png)\n\n![A missing image](missing.png)"
	for _, m := range metrics {
		e := newViewEnvIn(t, src, fixtureDir(t), Light, m)
		for _, w := range widths {
			for i := 0; i < e.view.Len(); i++ {
				e.layout(i, w)
			}
		}
	}
}

// metricAt is a metric of px pixels per dp and per sp.
func metricAt(px float32) unit.Metric { return unit.Metric{PxPerDp: px, PxPerSp: px} }

// corpusFiles lists the documents that the geometry tests read: the rendering corpus and
// elements.md.
func corpusFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../testdata/corpus/*.md")
	if err != nil {
		t.Fatalf("glob the corpus: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files in testdata/corpus")
	}
	return append(files, "../../testdata/elements.md")
}

// fileEnv is a View of the Markdown file at path, its images resolved beside it, at metric m.
func fileEnv(t *testing.T, path string, m unit.Metric) *viewEnv {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		t.Fatalf("abs %s: %v", path, err)
	}
	return newViewEnvIn(t, string(src), dir, Light, m)
}

// TestGlyphOriginIsBaseline: every fragment's Origin is strictly inside its glyph box, so the
// baseline it names is one the ink stands on (STYLE-TEXT-7). Over the corpus and elements.md, at
// three metrics, and in one line of body and code and bold text, every fragment shares one Origin.Y.
func TestGlyphOriginIsBaseline(t *testing.T) {
	for _, path := range corpusFiles(t) {
		for _, px := range []float32{1, 1.25, 2} {
			e := fileEnv(t, path, metricAt(px))
			for i := 0; i < e.view.Len(); i++ {
				_, tr := e.layout(i, 800)
				for _, m := range tr.Marks {
					if m.Kind != MarkGlyphs && m.Kind != MarkNumber {
						continue
					}
					if m.Rect.Min.Y >= m.Origin.Y || m.Origin.Y >= m.Rect.Max.Y {
						t.Errorf("%s at %v, block %d: %q origin %v not inside glyph rect %v",
							filepath.Base(path), px, i, m.Text, m.Origin, m.Rect)
					}
				}
			}
		}
	}
	for _, px := range []float32{1, 1.25, 2} {
		e := newFlowEnvAt(t, 800, px)
		tr := e.trace([]doc.Run{plain("body "), codeRun("mono "), styledRun("bold", doc.Strong), plain("end")})
		gs := marks(tr, MarkGlyphs)
		if len(gs) != 4 {
			t.Fatalf("at %v: %d Glyphs marks for four runs on one line", px, len(gs))
		}
		for _, g := range gs {
			if g.Origin.Y != gs[0].Origin.Y {
				t.Errorf("at %v: %q origin y %d, %q has %d; one line has one baseline", px, g.Text, g.Origin.Y, gs[0].Text, gs[0].Origin.Y)
			}
		}
	}
}

// TestDecorationsShareGeometry: the decorations of each fragment are placed from the baseline its
// glyphs are drawn on (STYLE-TEXT-7). Over the corpus, at three metrics and two widths: a link
// area holds its glyph box and its origin; a code background holds its glyphs; a strike starts
// between the glyph box's top and the origin; and a bullet or task box is centred between the top
// and the origin of the nearest fragment to its right.
func TestDecorationsShareGeometry(t *testing.T) {
	for _, path := range corpusFiles(t) {
		for _, px := range []float32{1, 1.25, 2} {
			for _, w := range []int{600, 970} {
				e := fileEnv(t, path, metricAt(px))
				for i := 0; i < e.view.Len(); i++ {
					_, tr := e.layout(i, w)
					checkDecorations(t, fmt.Sprintf("%s at %v, %d px, block %d", filepath.Base(path), px, w, i), tr)
				}
			}
		}
	}
}

// checkDecorations checks the geometry of tr's decorations against the Glyphs marks they belong
// to (TestDecorationsShareGeometry). Marks are paired by record order: a code background precedes
// the glyphs it lies behind, and a strike or link area follows them. A link area that does not
// hold the last glyphs' x span is an image's, and is not paired.
func checkDecorations(t *testing.T, where string, tr *Trace) {
	t.Helper()
	var glyphs []Mark
	for _, m := range tr.Marks {
		if m.Kind == MarkGlyphs {
			glyphs = append(glyphs, m)
		}
	}
	var last *Mark
	var code *Mark
	for k := range tr.Marks {
		m := tr.Marks[k]
		switch m.Kind {
		case MarkCodeBg:
			code = &m
		case MarkGlyphs:
			if code != nil && !within(code.Rect, m.Rect) {
				t.Errorf("%s: code background %v does not hold its glyphs %v", where, code.Rect, m.Rect)
			}
			code = nil
			last = &tr.Marks[k]
		case MarkStrike:
			if last == nil {
				t.Errorf("%s: strike with no glyphs before it", where)
				continue
			}
			if top := m.Rect.Min.Y; top < last.Rect.Min.Y || top > last.Origin.Y {
				t.Errorf("%s: strike top %d outside [%d, %d] of %q", where, top, last.Rect.Min.Y, last.Origin.Y, last.Text)
			}
		case MarkLinkArea:
			if last == nil || m.Rect.Min.X > last.Rect.Min.X || last.Rect.Max.X > m.Rect.Max.X {
				continue
			}
			if !within(m.Rect, last.Rect) || last.Origin.Y < m.Rect.Min.Y || last.Origin.Y > m.Rect.Max.Y {
				t.Errorf("%s: link area %v does not hold %q (glyphs %v, origin y %d)", where, m.Rect, last.Text, last.Rect, last.Origin.Y)
			}
		case MarkBullet, MarkTaskBox:
			if m.Kind == MarkTaskBox && m.Text != taskMarkOpen && m.Text != taskMarkDone {
				continue
			}
			checkMarkerLine(t, where, m, glyphs)
		}
	}
}

// checkMarkerLine checks that a marker's centre lies between the top and the origin of the
// nearest Glyphs mark to its right, which is the first line of its item (STYLE-LIST-2, STYLE-LIST-5).
func checkMarkerLine(t *testing.T, where string, m Mark, glyphs []Mark) {
	t.Helper()
	cy := (m.Rect.Min.Y + m.Rect.Max.Y) / 2
	var near *Mark
	for k := range glyphs {
		g := &glyphs[k]
		if g.Rect.Min.X < m.Rect.Max.X {
			continue
		}
		if near == nil || abs(cy-g.Origin.Y) < abs(cy-near.Origin.Y) {
			near = g
		}
	}
	if near == nil {
		t.Errorf("%s: marker %v has no text to its right", where, m.Rect)
		return
	}
	if cy < near.Rect.Min.Y || cy > near.Origin.Y {
		t.Errorf("%s: marker centre %d outside [%d, %d] of %q", where, cy, near.Rect.Min.Y, near.Origin.Y, near.Text)
	}
}

// abs is the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestNoMarkPastContentEdge: no mark of a block reaches past the content edge, the width less the
// content padding (DOC-LAYOUT-1), over the corpus and elements.md at 310, 600 and 970 dp and three
// metrics. A table is skipped: STYLE-TABLE-5 clips it at the edge. The test guards against text
// that runs past the window's right side.
func TestNoMarkPastContentEdge(t *testing.T) {
	for _, path := range corpusFiles(t) {
		for _, px := range []float32{1, 1.25, 2} {
			for _, dp := range []int{310, 600, 970} {
				e := fileEnv(t, path, metricAt(px))
				width := int(float32(dp) * px)
				gtx := e.context(width)
				edge := width - gtx.Dp(Padding(gtx.Metric.PxToDp(width)))
				for i := 0; i < e.view.Len(); i++ {
					if _, ok := e.d.Blocks[i].(doc.Table); ok {
						continue
					}
					_, tr := e.layout(i, width)
					for _, m := range tr.Marks {
						if m.Rect.Max.X > edge {
							t.Errorf("%s at %d dp, %v: block %d mark %v (%q) ends at %d, past the content edge %d",
								filepath.Base(path), dp, px, i, m.Kind, m.Text, m.Rect.Max.X, edge)
						}
					}
				}
			}
		}
	}
}
