package render

import (
	"image"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// flowEnv is a headless layout setting (ARCHITECTURE "Headless render tests"): one op list and
// one input router, at 1 px per dp and per sp, with the body style on the light palette.
type flowEnv struct {
	t      *testing.T
	ops    op.Ops
	router input.Router
	fp     flowParams
	width  int
}

func newFlowEnv(t *testing.T, width int) *flowEnv {
	t.Helper()
	faces, err := LoadFaces()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	e := &flowEnv{t: t, width: width}
	e.fp = flowParams{
		shaper:  NewShaper(faces),
		base:    BodyStyle(Light),
		palette: Light,
		links:   newLinks(),
	}
	return e
}

// context is a gtx on the env's ops, with the env's router as its input source.
func (e *flowEnv) context() layout.Context {
	return layout.Context{
		Ops:         &e.ops,
		Source:      e.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(e.width, 1<<20)},
	}
}

// lay lays out runs into a new trace, failing the test on an error.
func (e *flowEnv) lay(runs []doc.Run) (layout.Dimensions, *Trace) {
	e.t.Helper()
	tr := &Trace{}
	dims, err := layoutFlow(e.context(), e.fp, runs, tr)
	if err != nil {
		e.t.Fatalf("layoutFlow: %v", err)
	}
	return dims, tr
}

// trace is lay for a test that reads only the marks.
func (e *flowEnv) trace(runs []doc.Run) *Trace {
	e.t.Helper()
	tr := &Trace{}
	if _, err := layoutFlow(e.context(), e.fp, runs, tr); err != nil {
		e.t.Fatalf("layoutFlow: %v", err)
	}
	return tr
}

// frame is one router frame: reset the ops, lay out runs, then r.Frame.
func (e *flowEnv) frame(runs []doc.Run) *Trace {
	e.t.Helper()
	e.ops.Reset()
	tr := e.trace(runs)
	e.router.Frame(&e.ops)
	return tr
}

// poll is one frame's click poll, made before that frame is laid out, as View.Clicked is each
// frame (M5). A gesture takes its pointer events from the polls before them.
func (e *flowEnv) poll() (doc.Target, bool) {
	e.ops.Reset()
	return e.fp.links.clicked(e.context())
}

// heading1 is the level 1 heading style, which has a rule (STYLE-TEXT-2).
func heading1(t *testing.T) TextStyle {
	t.Helper()
	ts, rule, err := HeadingStyle(1, Light)
	if err != nil {
		t.Fatalf("HeadingStyle: %v", err)
	}
	if rule.Thickness != 1 {
		t.Fatalf("heading 1 rule is %v thick, want 1 dp", rule.Thickness)
	}
	return ts
}

func plain(s string) doc.Run { return doc.Run{Kind: doc.TextRun, Text: s} }

func codeRun(s string) doc.Run { return doc.Run{Kind: doc.TextRun, Text: s, Style: doc.Code} }

func linkRun(s string, tg *doc.Target) doc.Run {
	return doc.Run{Kind: doc.TextRun, Text: s, Link: tg}
}

var brk = doc.Run{Kind: doc.BreakRun}

// marks returns the marks of kind, in trace order.
func marks(tr *Trace, kind MarkKind) []Mark {
	var out []Mark
	for _, m := range tr.Marks {
		if m.Kind == kind {
			out = append(out, m)
		}
	}
	return out
}

// needMarks returns the marks of kind, failing the test with "no NAME mark" when there are none.
func needMarks(t *testing.T, tr *Trace, kind MarkKind, name string) []Mark {
	t.Helper()
	ms := marks(tr, kind)
	if len(ms) == 0 {
		t.Fatalf("no %s mark", name)
	}
	return ms
}

// glyphMark returns the first Glyphs mark whose text is s.
func glyphMark(t *testing.T, tr *Trace, s string) Mark {
	t.Helper()
	for _, m := range marks(tr, MarkGlyphs) {
		if m.Text == s {
			return m
		}
	}
	t.Fatalf("no Glyphs mark with text %q", s)
	return Mark{}
}

// joinGlyphText concatenates the Glyphs marks' text in order: text conservation (no text is
// dropped or truncated).
func joinGlyphText(tr *Trace) string {
	var b strings.Builder
	for _, m := range marks(tr, MarkGlyphs) {
		b.WriteString(m.Text)
	}
	return b.String()
}

// lineStarts returns the first Glyphs mark of each line, where a line is a run of marks at one
// top edge.
func lineStarts(gs []Mark) []Mark {
	var out []Mark
	for i, m := range gs {
		if i == 0 || m.Rect.Min.Y != gs[i-1].Rect.Min.Y {
			out = append(out, m)
		}
	}
	return out
}

// within reports whether inner lies inside outer, edges included.
func within(outer, inner image.Rectangle) bool {
	return inner.Min.X >= outer.Min.X && inner.Min.Y >= outer.Min.Y &&
		inner.Max.X <= outer.Max.X && inner.Max.Y <= outer.Max.Y
}

// centre is the middle pixel of r.
func centre(r image.Rectangle) image.Point {
	return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
}

func press(p image.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Press, Position: f32.Pt(float32(p.X), float32(p.Y)), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse}
}

func release(p image.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Release, Position: f32.Pt(float32(p.X), float32(p.Y)), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse}
}

func move(p image.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(p.X), float32(p.Y)), Source: pointer.Mouse}
}

// TestFlowLineHeight: a body line is 24 px (16 sp x 1.5, STYLE-TEXT-1), and each further line
// adds 24 px.
func TestFlowLineHeight(t *testing.T) {
	e := newFlowEnv(t, 800)
	dims, tr := e.lay([]doc.Run{plain("Hello")})
	if dims.Size.Y != 24 {
		t.Fatalf("one line: Size.Y = %d, want 24", dims.Size.Y)
	}
	if n := len(marks(tr, MarkGlyphs)); n != 1 {
		t.Fatalf("one line: %d Glyphs marks, want 1", n)
	}

	e = newFlowEnv(t, 50)
	dims, tr = e.lay([]doc.Run{plain("Hello world")})
	if dims.Size.Y != 48 {
		t.Fatalf("two lines: Size.Y = %d, want 48", dims.Size.Y)
	}
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 2 {
		t.Fatalf("two lines: %d Glyphs marks, want 2", len(gs))
	}
	if d := gs[1].Rect.Min.Y - gs[0].Rect.Min.Y; d != 24 {
		t.Fatalf("second line is %d px below the first, want 24", d)
	}
	if got := joinGlyphText(tr); got != "Hello world" {
		t.Fatalf("text = %q, want %q", got, "Hello world")
	}
}

// TestFlowWrapsToWidth: text wraps to the width, every line starts at the left edge, and the
// narrower width is taller (WRAP-1, STYLE-TEXT-6).
func TestFlowWrapsToWidth(t *testing.T) {
	source := strings.TrimSpace(strings.Repeat("word ", 60))
	heights := map[int]int{}
	for _, width := range []int{800, 200} {
		e := newFlowEnv(t, width)
		dims, tr := e.lay([]doc.Run{plain(source)})
		if dims.Size.Y == 0 || dims.Size.Y%24 != 0 {
			t.Fatalf("width %d: Size.Y = %d, want a non-zero multiple of 24", width, dims.Size.Y)
		}
		heights[width] = dims.Size.Y
		gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
		for _, m := range gs {
			if m.Rect.Max.X > width {
				t.Fatalf("width %d: mark %q ends at x=%d", width, m.Text, m.Rect.Max.X)
			}
		}
		for _, m := range lineStarts(gs) {
			if m.Rect.Min.X != 0 {
				t.Fatalf("width %d: line starting %q begins at x=%d, want 0", width, m.Text, m.Rect.Min.X)
			}
		}
		if got := joinGlyphText(tr); got != source {
			t.Fatalf("width %d: text is not conserved: %q", width, got)
		}
	}
	if heights[200] <= heights[800] {
		t.Fatalf("200 px is %d tall, not taller than 800 px (%d)", heights[200], heights[800])
	}
}

// TestFlowLongWord: a word wider than the line breaks at a character, 25 per 200 px line, so
// 300 characters are 12 lines (WRAP-1). After a word that fits, the first line keeps that word
// and the long word starts a line of its own (WrapWords first, WrapHeuristically only at a line
// start).
func TestFlowLongWord(t *testing.T) {
	word := strings.Repeat("x", 300)
	const perLine = 25

	e := newFlowEnv(t, 200)
	tr := e.trace([]doc.Run{plain(word)})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 12 {
		t.Fatalf("300 x at 200 px: %d Glyphs marks, want 12", len(gs))
	}
	if gs[0].Text != strings.Repeat("x", perLine) {
		t.Fatalf("first line is %q, want %d x", gs[0].Text, perLine)
	}
	for _, m := range gs {
		if m.Rect.Max.X > 200 {
			t.Fatalf("mark ends at x=%d, want at most 200", m.Rect.Max.X)
		}
	}
	if got := joinGlyphText(tr); got != word {
		t.Fatalf("text is not conserved: %d runes, want 300", utf8.RuneCountInString(got))
	}

	e = newFlowEnv(t, 200)
	tr = e.trace([]doc.Run{plain("ab " + word)})
	gs = needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 13 {
		t.Fatalf("after a word: %d Glyphs marks, want 13 (ab and 12 lines)", len(gs))
	}
	if gs[0].Text != "ab " {
		t.Fatalf("first line is %q, want %q", gs[0].Text, "ab ")
	}
	if gs[1].Text != strings.Repeat("x", perLine) {
		t.Fatalf("second line is %q, want %d x", gs[1].Text, perLine)
	}
	for _, m := range gs {
		if m.Rect.Max.X > 200 {
			t.Fatalf("after a word: mark ends at x=%d, want at most 200", m.Rect.Max.X)
		}
	}
	if got := joinGlyphText(tr); got != "ab "+word {
		t.Fatalf("after a word: text is not conserved")
	}
}

// TestFlowBreakRun: a hard break ends the line, and two in a row give an empty line between
// (DOC-3).
func TestFlowBreakRun(t *testing.T) {
	e := newFlowEnv(t, 800)
	dims, tr := e.lay([]doc.Run{plain("a"), brk, plain("b")})
	if dims.Size.Y != 48 {
		t.Fatalf("a, break, b: Size.Y = %d, want 48", dims.Size.Y)
	}
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 2 || gs[0].Text != "a" || gs[1].Text != "b" {
		t.Fatalf("a, break, b: marks are %v, want a then b", gs)
	}
	if gs[0].Rect.Min.X != 0 || gs[1].Rect.Min.X != 0 {
		t.Fatalf("a, break, b: lines start at x=%d and x=%d, want 0", gs[0].Rect.Min.X, gs[1].Rect.Min.X)
	}
	if d := gs[1].Rect.Min.Y - gs[0].Rect.Min.Y; d != 24 {
		t.Fatalf("b is %d px below a, want 24", d)
	}

	e = newFlowEnv(t, 800)
	dims, tr = e.lay([]doc.Run{plain("a"), brk, brk, plain("b")})
	if dims.Size.Y != 72 {
		t.Fatalf("a, break, break, b: Size.Y = %d, want 72", dims.Size.Y)
	}
	gs = needMarks(t, tr, MarkGlyphs, "Glyphs")
	if d := gs[1].Rect.Min.Y - gs[0].Rect.Min.Y; d != 48 {
		t.Fatalf("a, break, break, b: b is %d px below a, want 48", d)
	}
}

// TestFlowCodeBackground: a code run has one CodeBg rectangle around its glyphs, padded 0.4 em
// across and 0.2 em up and down at 13.6 sp, rounded to whole pixels (STYLE-TEXT-5). The padding
// is part of the run's width. A wrapped code run has one background per line.
func TestFlowCodeBackground(t *testing.T) {
	e := newFlowEnv(t, 800)
	dims, tr := e.lay([]doc.Run{plain("see "), codeRun("x := 1"), plain(" here")})
	bgs := needMarks(t, tr, MarkCodeBg, "CodeBg")
	if len(bgs) != 1 {
		t.Fatalf("one code run: %d CodeBg marks, want 1", len(bgs))
	}
	bg := bgs[0]
	if bg.Colour != Light.CodeBg {
		t.Fatalf("CodeBg colour = %v, want %v", bg.Colour, Light.CodeBg)
	}
	g := glyphMark(t, tr, "x := 1")
	if d := g.Rect.Min.X - bg.Rect.Min.X; d != 5 {
		t.Fatalf("left padding = %d, want 5 (0.4 em of 13.6 sp)", d)
	}
	if d := bg.Rect.Max.X - g.Rect.Max.X; d != 5 {
		t.Fatalf("right padding = %d, want 5", d)
	}
	if d := g.Rect.Min.Y - bg.Rect.Min.Y; d != 3 {
		t.Fatalf("top padding = %d, want 3 (0.2 em of 13.6 sp)", d)
	}
	if d := bg.Rect.Max.Y - g.Rect.Max.Y; d != 3 {
		t.Fatalf("bottom padding = %d, want 3", d)
	}
	if after := glyphMark(t, tr, " here"); after.Rect.Min.X < bg.Rect.Max.X {
		t.Fatalf("text after the code starts at x=%d, inside its background (ends at %d)", after.Rect.Min.X, bg.Rect.Max.X)
	}
	if dims.Size.Y != 24 {
		t.Fatalf("code keeps the line: Size.Y = %d, want 24", dims.Size.Y)
	}

	source := strings.TrimSpace(strings.Repeat("abc ", 40))
	e = newFlowEnv(t, 120)
	tr = e.trace([]doc.Run{codeRun(source)})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	bgs = needMarks(t, tr, MarkCodeBg, "CodeBg")
	if len(bgs) < 2 {
		t.Fatalf("wrapped code run: %d CodeBg marks, want at least 2", len(bgs))
	}
	for _, m := range bgs {
		if m.Rect.Max.X > 120 {
			t.Fatalf("CodeBg ends at x=%d, want at most 120", m.Rect.Max.X)
		}
	}
	for _, m := range gs {
		owned := false
		for _, b := range bgs {
			if within(b.Rect, m.Rect) {
				owned = true
			}
		}
		if !owned {
			t.Fatalf("code line %q has no CodeBg around it", m.Text)
		}
	}
}

// TestFlowCodeInHeading: inside a heading, code is the heading's size with 0.2 em horizontal
// padding and none vertical (STYLE-TEXT-5), and the line keeps the heading's height.
func TestFlowCodeInHeading(t *testing.T) {
	e := newFlowEnv(t, 800)
	e.fp.base = heading1(t)
	e.fp.inHeading = true
	dims, tr := e.lay([]doc.Run{plain("A "), codeRun("b")})
	bgs := needMarks(t, tr, MarkCodeBg, "CodeBg")
	if len(bgs) != 1 {
		t.Fatalf("heading code: %d CodeBg marks, want 1", len(bgs))
	}
	bg := bgs[0]
	g := glyphMark(t, tr, "b")
	if d := g.Rect.Min.X - bg.Rect.Min.X; d != 6 {
		t.Fatalf("left padding = %d, want 6 (0.2 em of 32 sp)", d)
	}
	if d := bg.Rect.Max.X - g.Rect.Max.X; d != 6 {
		t.Fatalf("right padding = %d, want 6", d)
	}
	if bg.Rect.Min.Y != g.Rect.Min.Y || bg.Rect.Max.Y != g.Rect.Max.Y {
		t.Fatalf("heading code has vertical padding: bg %v, glyphs %v", bg.Rect, g.Rect)
	}
	if dims.Size.Y != 40 {
		t.Fatalf("heading line: Size.Y = %d, want 40 (32 sp x 1.25)", dims.Size.Y)
	}

	body := newFlowEnv(t, 800)
	bodyTr := body.trace([]doc.Run{codeRun("b")})
	if bodyG := glyphMark(t, bodyTr, "b"); g.Rect.Dy() <= bodyG.Rect.Dy() {
		t.Fatalf("heading code is %d px tall, not taller than body code (%d): it must not be 85%%", g.Rect.Dy(), bodyG.Rect.Dy())
	}
}

// TestFlowStrike: a struck run has a line across its width, at 0.3 em above the baseline, of
// max(1 dp, size / 16) (STYLE-TEXT-4).
func TestFlowStrike(t *testing.T) {
	struck := doc.Run{Kind: doc.TextRun, Text: "struck", Style: doc.Strike}
	e := newFlowEnv(t, 800)
	tr := e.trace([]doc.Run{struck, plain(" text")})
	ss := needMarks(t, tr, MarkStrike, "Strike")
	if len(ss) != 1 {
		t.Fatalf("one struck run: %d Strike marks, want 1", len(ss))
	}
	s := ss[0]
	if s.Colour != Light.Fg {
		t.Fatalf("Strike colour = %v, want the run's colour %v", s.Colour, Light.Fg)
	}
	g := glyphMark(t, tr, "struck")
	if s.Rect.Min.X != g.Rect.Min.X || s.Rect.Max.X != g.Rect.Max.X {
		t.Fatalf("Strike spans x %d to %d, the glyphs span %d to %d", s.Rect.Min.X, s.Rect.Max.X, g.Rect.Min.X, g.Rect.Max.X)
	}
	if s.Rect.Dy() != 1 {
		t.Fatalf("Strike is %d px thick, want 1 (16 sp)", s.Rect.Dy())
	}
	if !(s.Rect.Min.Y > g.Rect.Min.Y && s.Rect.Max.Y < g.Rect.Max.Y) {
		t.Fatalf("Strike %v is not inside the glyph box %v", s.Rect, g.Rect)
	}

	e = newFlowEnv(t, 800)
	e.fp.base = heading1(t)
	e.fp.inHeading = true
	tr = e.trace([]doc.Run{struck})
	ss = needMarks(t, tr, MarkStrike, "Strike")
	if ss[0].Rect.Dy() != 2 {
		t.Fatalf("Strike at 32 sp is %d px thick, want 2 (32 / 16)", ss[0].Rect.Dy())
	}
}

// TestFlowLinkArea: a link run has a LinkArea that carries its target, and its glyphs are drawn
// in Accent. A link that wraps has one area per fragment, each with the same target.
func TestFlowLinkArea(t *testing.T) {
	tg := &doc.Target{Kind: doc.External, Value: "https://e.org/"}
	e := newFlowEnv(t, 800)
	tr := e.trace([]doc.Run{plain("go to "), linkRun("the docs", tg)})
	las := needMarks(t, tr, MarkLinkArea, "LinkArea")
	if len(las) != 1 {
		t.Fatalf("one link fragment: %d LinkArea marks, want 1", len(las))
	}
	if las[0].Link != tg {
		t.Fatalf("LinkArea target is %p, want the run's %p", las[0].Link, tg)
	}
	g := glyphMark(t, tr, "the docs")
	if !within(las[0].Rect, g.Rect) {
		t.Fatalf("LinkArea %v does not contain the link's glyphs %v", las[0].Rect, g.Rect)
	}
	if g.Colour != Light.Accent {
		t.Fatalf("link glyphs are %v, want Accent %v", g.Colour, Light.Accent)
	}
	if plainG := glyphMark(t, tr, "go to "); plainG.Colour != Light.Fg {
		t.Fatalf("plain glyphs are %v, want Fg %v", plainG.Colour, Light.Fg)
	}

	e = newFlowEnv(t, 60)
	tr = e.trace([]doc.Run{plain("go to "), linkRun("the docs of the project", tg)})
	las = needMarks(t, tr, MarkLinkArea, "LinkArea")
	if len(las) < 2 {
		t.Fatalf("wrapped link: %d LinkArea marks, want at least 2", len(las))
	}
	for _, m := range las {
		if m.Link != tg {
			t.Fatalf("LinkArea target is %p, want the run's %p", m.Link, tg)
		}
	}
}

// TestFlowLinkClick: a primary press and release inside a link's area is one click on its
// target, reported once (LINK-1). The next poll reports nothing, and a click on plain text
// reports nothing.
func TestFlowLinkClick(t *testing.T) {
	tg := &doc.Target{Kind: doc.External, Value: "https://e.org/"}
	runs := []doc.Run{plain("go to "), linkRun("the docs", tg)}
	e := newFlowEnv(t, 800)
	// Frame 1 lays out, which makes the link's gesture. Frame 2 polls before it lays out, as
	// every frame does, so the gesture takes the events queued after it.
	e.frame(runs)
	if early, polled := e.poll(); polled {
		t.Fatalf("a poll with no input reported %+v", early)
	}
	tr := e.frame(runs)
	la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
	e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))

	got, ok := e.poll()
	if !ok || got != *tg {
		t.Fatalf("poll after the click = %+v, %t; want the link's target and true", got, ok)
	}
	if again, more := e.poll(); more {
		t.Fatalf("second poll in the same frame reported %+v again", again)
	}
	e.frame(runs)
	if later, more := e.poll(); more {
		t.Fatalf("the poll after that frame reported %+v again", later)
	}

	e = newFlowEnv(t, 800)
	tr = e.frame(runs)
	e.poll()
	tr = e.frame(runs)
	g := glyphMark(t, tr, "go to ")
	e.router.Queue(move(centre(g.Rect)), press(centre(g.Rect)), release(centre(g.Rect)))
	if plainHit, hit := e.poll(); hit {
		t.Fatalf("a click on plain text reported %+v", plainHit)
	}
}

// TestFlowLinkCursor: over a link the pointer is the hand, and over plain text it is the
// default (STYLE-TEXT-3). The cursor changes at the frame that takes the move.
func TestFlowLinkCursor(t *testing.T) {
	tg := &doc.Target{Kind: doc.External, Value: "https://e.org/"}
	runs := []doc.Run{plain("go to "), linkRun("the docs", tg)}
	e := newFlowEnv(t, 800)
	tr := e.frame(runs)
	la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
	g := glyphMark(t, tr, "go to ")

	e.router.Queue(move(centre(la.Rect)))
	e.frame(runs)
	if c := e.router.Cursor(); c != pointer.CursorPointer {
		t.Fatalf("over a link the cursor is %v, want CursorPointer", c)
	}

	e.router.Queue(move(centre(g.Rect)))
	e.frame(runs)
	if c := e.router.Cursor(); c != pointer.CursorDefault {
		t.Fatalf("over plain text the cursor is %v, want CursorDefault", c)
	}
}

// TestFlowInvalidUTF8: each invalid byte is shown as one U+FFFD, with no error (DOC-15). A
// U+FFFD glyph is as wide as the invalid byte's glyph.
func TestFlowInvalidUTF8(t *testing.T) {
	e := newFlowEnv(t, 800)
	tr := e.trace([]doc.Run{plain("a\xffb")})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 1 {
		t.Fatalf("a\\xffb: %d Glyphs marks, want 1", len(gs))
	}
	if gs[0].Text != "a�b" {
		t.Fatalf("a\\xffb: text = %q, want %q", gs[0].Text, "a�b")
	}
	if n := utf8.RuneCountInString(gs[0].Text); n != 3 {
		t.Fatalf("a\\xffb: %d runes, want 3", n)
	}
	ref := newFlowEnv(t, 800)
	refTr := ref.trace([]doc.Run{plain("a�b")})
	want := needMarks(t, refTr, MarkGlyphs, "Glyphs")[0]
	if gs[0].Rect.Dx() != want.Rect.Dx() {
		t.Fatalf("a\\xffb is %d px wide, a U+FFFD b is %d", gs[0].Rect.Dx(), want.Rect.Dx())
	}

	e = newFlowEnv(t, 800)
	tr = e.trace([]doc.Run{plain("a\xff\xfeb")})
	gs = needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 1 || gs[0].Text != "a��b" {
		t.Fatalf("a\\xff\\xfeb: marks %v, want one with text %q", gs, "a��b")
	}
}

// TestFlowUnknownRunKind: a run kind the flow does not know is an error that names it, and no
// dimensions are returned with it.
func TestFlowUnknownRunKind(t *testing.T) {
	e := newFlowEnv(t, 800)
	dims, err := layoutFlow(e.context(), e.fp, []doc.Run{{Kind: doc.RunKind(9)}}, &Trace{})
	if err == nil {
		t.Fatal("run kind 9: no error")
	}
	if !strings.Contains(err.Error(), "9") {
		t.Fatalf("run kind 9: error %q does not name the kind", err)
	}
	if dims != (layout.Dimensions{}) {
		t.Fatalf("run kind 9: dimensions %+v with an error", dims)
	}
}

// TestFlowNilTrace: a nil trace is allowed and lays out the same as a recording one.
func TestFlowNilTrace(t *testing.T) {
	runs := []doc.Run{plain(strings.TrimSpace(strings.Repeat("word ", 60)))}
	e := newFlowEnv(t, 200)
	withTrace, tr := e.lay(runs)
	if len(marks(tr, MarkGlyphs)) == 0 {
		t.Fatal("the recording trace has no Glyphs marks")
	}
	withNil, err := layoutFlow(e.context(), e.fp, runs, nil)
	if err != nil {
		t.Fatalf("layoutFlow with a nil trace: %v", err)
	}
	if withNil != withTrace {
		t.Fatalf("nil trace gives %+v, a recording trace gives %+v", withNil, withTrace)
	}
	if withNil.Size.Y <= 0 {
		t.Fatalf("nil trace: Size.Y = %d, want more than 0", withNil.Size.Y)
	}
}

// TestFlowEmptyRuns: no runs give one empty line of body height and no text (DOC-8's empty
// item, STYLE-LIST-5's first line).
func TestFlowEmptyRuns(t *testing.T) {
	e := newFlowEnv(t, 800)
	dims, tr := e.lay(nil)
	if dims.Size.Y != 24 {
		t.Fatalf("no runs: Size.Y = %d, want 24", dims.Size.Y)
	}
	if n := len(marks(tr, MarkGlyphs)); n != 0 {
		t.Fatalf("no runs: %d Glyphs marks, want 0", n)
	}
}

// TestFlowLeadingSpace: a non-code run that starts a line drops its leading space; a code run
// keeps it (WRAP-1).
func TestFlowLeadingSpace(t *testing.T) {
	e := newFlowEnv(t, 800)
	tr := e.trace([]doc.Run{plain("a"), brk, plain(" b")})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 2 || gs[1].Text != "b" {
		t.Fatalf("plain run after a break: marks %v, want the second text %q", gs, "b")
	}

	e = newFlowEnv(t, 800)
	tr = e.trace([]doc.Run{plain("a"), brk, codeRun(" c")})
	gs = needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 2 || gs[1].Text != " c" {
		t.Fatalf("code run after a break: marks %v, want the second text %q", gs, " c")
	}
}

// TestFlowTinyWidth: a line narrower than one character still lays out, with no error. Each
// character takes a line of its own, and the text is kept whole. A regression that loops fails
// by the test timeout.
func TestFlowTinyWidth(t *testing.T) {
	for _, width := range []int{0, 1} {
		e := newFlowEnv(t, width)
		tr := e.trace([]doc.Run{plain("ab")})
		gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
		if len(gs) != 2 || gs[0].Text != "a" || gs[1].Text != "b" {
			t.Fatalf("width %d: marks %v, want a then b", width, gs)
		}
		if got := joinGlyphText(tr); got != "ab" {
			t.Fatalf("width %d: text = %q, want %q", width, got, "ab")
		}
	}
}

// TestFlowRunBoundary: runs shape alone, so a space that ends a line stays on it and the long
// run wraps on its own (WRAP-1). A word split between two runs breaks at the boundary, the
// known limitation in ARCHITECTURE "internal/render".
func TestFlowRunBoundary(t *testing.T) {
	x := strings.Repeat("x", 30)
	e := newFlowEnv(t, 100)
	tr := e.trace([]doc.Run{plain("ab"), plain(" " + x)})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	wantText := []string{"ab", " ", x[:12], x[:12], x[:6]}
	wantTop := []int{2, 2, 26, 50, 74}
	if len(gs) != len(wantText) {
		t.Fatalf("ab, then a 30 x run at 100 px: %d Glyphs marks, want %d", len(gs), len(wantText))
	}
	for i, m := range gs {
		if m.Text != wantText[i] || m.Rect.Min.Y != wantTop[i] {
			t.Fatalf("mark %d is %q at y=%d, want %q at y=%d", i, m.Text, m.Rect.Min.Y, wantText[i], wantTop[i])
		}
	}
	if got := joinGlyphText(tr); got != "ab "+x {
		t.Fatalf("text is not conserved: %q", got)
	}

	e = newFlowEnv(t, 20)
	tr = e.trace([]doc.Run{plain("ab"), plain("cd")})
	gs = needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 2 || gs[0].Text != "ab" || gs[1].Text != "cd" {
		t.Fatalf("ab, cd at 20 px: marks %v, want ab then cd on the next line", gs)
	}
	if d := gs[1].Rect.Min.Y - gs[0].Rect.Min.Y; d != 24 {
		t.Fatalf("cd is %d px below ab, want 24", d)
	}
}

// TestFlowTrailingSpace: a space at a wrap stays on the line before it, so no text is lost
// (WRAP-1).
func TestFlowTrailingSpace(t *testing.T) {
	e := newFlowEnv(t, 30)
	tr := e.trace([]doc.Run{plain("ab cd")})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 2 || gs[0].Text != "ab " || gs[1].Text != "cd" {
		t.Fatalf("ab cd at 30 px: marks %v, want %q then %q", gs, "ab ", "cd")
	}
	if got := joinGlyphText(tr); got != "ab cd" {
		t.Fatalf("text = %q, want %q", got, "ab cd")
	}
}

// TestFlowStrikeCode: a struck code run's line spans its padding too, so it is as wide as the
// code background (STYLE-TEXT-4, STYLE-TEXT-5).
func TestFlowStrikeCode(t *testing.T) {
	struck := doc.Run{Kind: doc.TextRun, Text: "x := 1", Style: doc.Code | doc.Strike}
	e := newFlowEnv(t, 800)
	tr := e.trace([]doc.Run{struck})
	bg := needMarks(t, tr, MarkCodeBg, "CodeBg")[0]
	ss := needMarks(t, tr, MarkStrike, "Strike")
	if len(ss) != 1 {
		t.Fatalf("one struck code run: %d Strike marks, want 1", len(ss))
	}
	s := ss[0]
	if s.Rect.Min.X != bg.Rect.Min.X || s.Rect.Max.X != bg.Rect.Max.X {
		t.Fatalf("Strike spans x %d to %d, the code background spans %d to %d", s.Rect.Min.X, s.Rect.Max.X, bg.Rect.Min.X, bg.Rect.Max.X)
	}
	g := glyphMark(t, tr, "x := 1")
	if d := g.Rect.Min.X - s.Rect.Min.X; d != 5 {
		t.Fatalf("Strike starts %d px left of the glyphs, want 5 (the code padding)", d)
	}
}

// TestFlowLinkWrap: every fragment of a wrapped link is clickable, and a click on its second
// fragment reports the link (LINK-1).
func TestFlowLinkWrap(t *testing.T) {
	tg := &doc.Target{Kind: doc.External, Value: "https://e.org/"}
	runs := []doc.Run{plain("go to "), linkRun("the docs of the project", tg)}
	e := newFlowEnv(t, 60)
	e.frame(runs)
	e.poll()
	tr := e.frame(runs)
	las := needMarks(t, tr, MarkLinkArea, "LinkArea")
	if len(las) < 2 {
		t.Fatalf("wrapped link: %d LinkArea marks, want at least 2", len(las))
	}
	second := las[1].Rect
	e.router.Queue(move(centre(second)), press(centre(second)), release(centre(second)))
	got, ok := e.poll()
	if !ok || got != *tg {
		t.Fatalf("click on the second fragment = %+v, %t; want the link's target and true", got, ok)
	}
}

// TestFlowBidi: a right-to-left run's marks span its glyphs from the left edge to the right
// edge, whatever order the glyphs are in, and the text after it starts where it ends.
func TestFlowBidi(t *testing.T) {
	cases := []struct {
		text  string
		width int
	}{{"שלום", 48}, {"مرحبا", 60}}
	for _, c := range cases {
		e := newFlowEnv(t, 800)
		tr := e.trace([]doc.Run{plain(c.text), plain(" x")})
		g := glyphMark(t, tr, c.text)
		if g.Rect.Min.X != 0 || g.Rect.Max.X != c.width {
			t.Fatalf("%q spans x %d to %d, want 0 to %d", c.text, g.Rect.Min.X, g.Rect.Max.X, c.width)
		}
		after := glyphMark(t, tr, " x")
		if after.Rect.Min.X != c.width {
			t.Fatalf("text after %q starts at x=%d, want %d", c.text, after.Rect.Min.X, c.width)
		}
	}
}

// markTexts is the text of each mark, in order.
func markTexts(ms []Mark) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Text)
	}
	return out
}

// flowModeEnv is a flow in a monospaced 14 px style with 21 px lines, in the given wrap mode.
// Each character is 8.40625 px wide (Go Mono advance), so 45 px holds five.
func flowModeEnv(t *testing.T, width int, wrap wrapMode) *flowEnv {
	t.Helper()
	e := newFlowEnv(t, width)
	e.fp.base = TextStyle{Font: monoRegular, Size: 14, LineHeight: 1.5, Colour: Light.Fg}
	e.fp.wrap = wrap
	return e
}

// TestFlowCodeMode: code mode wraps at the character that overflows (STYLE-CODE-2), keeps leading
// spaces, and closes the last line after a trailing break; prose mode does none of these.
func TestFlowCodeMode(t *testing.T) {
	// A line of 45 px: code takes five characters, prose takes the word break "ab ".
	e := flowModeEnv(t, 45, wrapCode)
	_, tr := e.lay([]doc.Run{plain("ab cdefgh")})
	if got, want := markTexts(lineStarts(needMarks(t, tr, MarkGlyphs, "Glyphs"))), []string{"ab cd", "efgh"}; !slices.Equal(got, want) {
		t.Fatalf("code at 45 px: line starts %q, want %q", got, want)
	}
	e = flowModeEnv(t, 45, wrapProse)
	_, tr = e.lay([]doc.Run{plain("ab cdefgh")})
	if first := needMarks(t, tr, MarkGlyphs, "Glyphs")[0]; first.Text != "ab " {
		t.Fatalf("prose at 45 px: first line %q, want %q", first.Text, "ab ")
	}

	// Leading spaces are shaped and kept in code: one mark, "   x", at the left edge.
	e = flowModeEnv(t, 800, wrapCode)
	_, tr = e.lay([]doc.Run{plain("   x")})
	gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
	if len(gs) != 1 || gs[0].Text != "   x" || gs[0].Rect.Min.X != 0 {
		t.Fatalf("code with leading spaces: marks %q, want one mark \"   x\" at x 0", markTexts(gs))
	}
	e = flowModeEnv(t, 800, wrapProse)
	_, tr = e.lay([]doc.Run{plain("   x")})
	if gs := needMarks(t, tr, MarkGlyphs, "Glyphs"); len(gs) != 1 || gs[0].Text != "x" {
		t.Fatalf("prose with leading spaces: marks %q, want \"x\"", markTexts(gs))
	}

	// A trailing break closes a second, empty line in code (STYLE-CODE-2); prose closes none.
	e = flowModeEnv(t, 800, wrapCode)
	dims, tr := e.lay([]doc.Run{plain("a"), brk})
	if dims.Size.Y != 42 {
		t.Fatalf("code, text then break: Size.Y = %d, want 42 (two lines)", dims.Size.Y)
	}
	if n := len(marks(tr, MarkGlyphs)); n != 1 {
		t.Fatalf("code, text then break: %d Glyphs marks, want 1", n)
	}
	e = flowModeEnv(t, 800, wrapProse)
	dims, tr = e.lay([]doc.Run{plain("a"), brk})
	if dims.Size.Y != 21 {
		t.Fatalf("prose, text then break: Size.Y = %d, want 21", dims.Size.Y)
	}

	// No runs give one empty line, with nothing drawn, in both modes.
	for _, wrap := range []wrapMode{wrapProse, wrapCode} {
		e = flowModeEnv(t, 800, wrap)
		dims, tr = e.lay(nil)
		if dims.Size.Y != 21 || len(tr.Marks) != 0 {
			t.Fatalf("wrap %d, no runs: Size.Y = %d with %d marks, want 21 and none", wrap, dims.Size.Y, len(tr.Marks))
		}
	}
}

// TestFlowAlign: a centred or end-aligned line is moved within its box by its spare width
// (STYLE-TABLE-4): centre by the spare pixels rounded down, end by all of them, and the marks it
// recorded move with it. A line wider than the box is not moved, and start alignment is unchanged.
// Each row is a subtest, so the rows that hold on start alignment are visible as passing.
func TestFlowAlign(t *testing.T) {
	alignedEnv := func(t *testing.T, width int, align textAlign) *flowEnv {
		t.Helper()
		e := newFlowEnv(t, width)
		e.fp.align = align
		return e
	}
	layoutDims := func(t *testing.T, e *flowEnv, runs []doc.Run) layout.Dimensions {
		t.Helper()
		dims, err := layoutFlow(e.context(), e.fp, runs, &Trace{})
		if err != nil {
			t.Fatalf("layoutFlow: %v", err)
		}
		return dims
	}
	t.Run("start", func(t *testing.T) {
		e := alignedEnv(t, 200, alignStart)
		tr := e.trace([]doc.Run{plain("aa")})
		if g := glyphMark(t, tr, "aa"); g.Rect.Min.X != 0 {
			t.Fatalf("start: aa at x=%d, want 0", g.Rect.Min.X)
		}
	})
	t.Run("centre", func(t *testing.T) {
		e := alignedEnv(t, 200, alignCentre)
		tr := e.trace([]doc.Run{plain("aa")})
		if g := glyphMark(t, tr, "aa"); g.Rect.Min.X != 91 {
			t.Fatalf("centre: aa at x=%d, want 91 ((200 - 18) / 2)", g.Rect.Min.X)
		}
	})
	t.Run("end", func(t *testing.T) {
		e := alignedEnv(t, 200, alignEnd)
		tr := e.trace([]doc.Run{plain("aa")})
		g := glyphMark(t, tr, "aa")
		if g.Rect.Min.X != 182 || g.Rect.Max.X != 200 {
			t.Fatalf("end: aa spans x %d to %d, want 182 to 200", g.Rect.Min.X, g.Rect.Max.X)
		}
	})
	t.Run("centre wrapped", func(t *testing.T) {
		runs := []doc.Run{plain("aa"), brk, plain("bbbbbbbb")}
		e := alignedEnv(t, 200, alignCentre)
		tr := e.trace(runs)
		if g := glyphMark(t, tr, "aa"); g.Rect.Min.X != 91 {
			t.Fatalf("centre: first line at x=%d, want 91", g.Rect.Min.X)
		}
		if g := glyphMark(t, tr, "bbbbbbbb"); g.Rect.Min.X != 64 {
			t.Fatalf("centre: second line at x=%d, want 64 ((200 - 72) / 2)", g.Rect.Min.X)
		}
	})
	t.Run("end wrapped", func(t *testing.T) {
		runs := []doc.Run{plain("aa"), brk, plain("bbbbbbbb")}
		e := alignedEnv(t, 200, alignEnd)
		tr := e.trace(runs)
		if g := glyphMark(t, tr, "aa"); g.Rect.Max.X != 200 {
			t.Fatalf("end: first line ends at x=%d, want 200", g.Rect.Max.X)
		}
		if g := glyphMark(t, tr, "bbbbbbbb"); g.Rect.Min.X != 128 || g.Rect.Max.X != 200 {
			t.Fatalf("end: second line spans x %d to %d, want 128 to 200", g.Rect.Min.X, g.Rect.Max.X)
		}
	})
	t.Run("centre code", func(t *testing.T) {
		e := alignedEnv(t, 200, alignCentre)
		tr := e.trace([]doc.Run{codeRun("e")})
		bg := needMarks(t, tr, MarkCodeBg, "CodeBg")[0]
		if bg.Rect.Min.X != 90 {
			t.Fatalf("centre: code background at x=%d, want 90 ((200 - 19) / 2, rounded down)", bg.Rect.Min.X)
		}
		if g := glyphMark(t, tr, "e"); !within(bg.Rect, g.Rect) {
			t.Fatalf("centre: code glyphs %v are outside their background %v", g.Rect, bg.Rect)
		}
	})
	t.Run("end struck", func(t *testing.T) {
		e := alignedEnv(t, 200, alignEnd)
		struck := doc.Run{Kind: doc.TextRun, Text: "struck", Style: doc.Strike}
		tr := e.trace([]doc.Run{struck})
		if s := needMarks(t, tr, MarkStrike, "Strike")[0]; s.Rect.Max.X != 200 {
			t.Fatalf("end: strike ends at x=%d, want 200", s.Rect.Max.X)
		}
	})
	t.Run("end link", func(t *testing.T) {
		tg := &doc.Target{Kind: doc.External, Value: "https://e.org/"}
		runs := []doc.Run{linkRun("l", tg)}
		e := alignedEnv(t, 200, alignEnd)
		// As TestFlowLinkClick: a frame lays out, then a poll, then the frame whose area is clicked.
		e.frame(runs)
		if early, polled := e.poll(); polled {
			t.Fatalf("a poll with no input reported %+v", early)
		}
		tr := e.frame(runs)
		la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
		if la.Rect.Max.X != 200 {
			t.Fatalf("end: link area ends at x=%d, want 200", la.Rect.Max.X)
		}
		e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))
		if got, ok := e.poll(); !ok || got != *tg {
			t.Fatalf("end: click on the moved link = %+v, %t; want the link's target and true", got, ok)
		}
	})
	t.Run("dims", func(t *testing.T) {
		runs := []doc.Run{plain("aa"), brk, plain("bbbbbbbb")}
		startDims := layoutDims(t, alignedEnv(t, 200, alignStart), runs)
		for _, align := range []textAlign{alignCentre, alignEnd} {
			if dims := layoutDims(t, alignedEnv(t, 200, align), runs); dims != startDims {
				t.Fatalf("align %d: dimensions %+v, start gives %+v", align, dims, startDims)
			}
		}
	})
	t.Run("nil trace", func(t *testing.T) {
		runs := []doc.Run{plain("aa"), brk, plain("bbbbbbbb")}
		e := alignedEnv(t, 200, alignCentre)
		withTrace := layoutDims(t, e, runs)
		withNil, err := layoutFlow(e.context(), e.fp, runs, nil)
		if err != nil {
			t.Fatalf("centre with a nil trace: %v", err)
		}
		if withNil != withTrace {
			t.Fatalf("centre: nil trace gives %+v, a recording trace gives %+v", withNil, withTrace)
		}
	})
	t.Run("overflowing line", func(t *testing.T) {
		e := alignedEnv(t, 1, alignEnd)
		tr := e.trace([]doc.Run{plain("ab")})
		for _, g := range needMarks(t, tr, MarkGlyphs, "Glyphs") {
			if g.Rect.Min.X != 0 {
				t.Fatalf("end at 1 px: %q starts at x=%d, want 0 (a line wider than its box is not moved)", g.Text, g.Rect.Min.X)
			}
		}
	})
}

// TestFlowAlignUnknown: an alignment that is not one of the three is an error that names it.
func TestFlowAlignUnknown(t *testing.T) {
	e := newFlowEnv(t, 800)
	e.fp.align = textAlign(9)
	f, err := newFlow(e.context(), e.fp, &Trace{})
	if err == nil || !strings.Contains(err.Error(), "9") {
		t.Fatalf("alignment 9: error %v, want one naming 9", err)
	}
	if f != nil {
		t.Fatal("alignment 9: a flow with an error")
	}
}

// TestFlowAbandonAligned: an error after an aligned line has opened its recording is returned
// with no dimensions, and leaves no macro open. A macro left open swallows every later operation
// on the same ops, so a link laid out after the error must still be clickable (STYLE-TABLE-4).
func TestFlowAbandonAligned(t *testing.T) {
	e := newFlowEnv(t, 200)
	e.fp.align = alignCentre
	dims, err := layoutFlow(e.context(), e.fp, []doc.Run{plain("a"), {Kind: doc.RunKind(9)}}, &Trace{})
	if err == nil {
		t.Fatal("run kind 9 after an aligned line: no error")
	}
	if dims != (layout.Dimensions{}) {
		t.Fatalf("run kind 9 after an aligned line: dimensions %+v with an error", dims)
	}
	// No reset between these steps: the link must be laid out on the ops the error left behind.
	tg := &doc.Target{Kind: doc.External, Value: "https://e.org/"}
	runs := []doc.Run{linkRun("l", tg)}
	e.trace(runs)
	e.router.Frame(&e.ops)
	if early, polled := e.fp.links.clicked(e.context()); polled {
		t.Fatalf("a poll with no input reported %+v", early)
	}
	tr := e.trace(runs)
	e.router.Frame(&e.ops)
	la := needMarks(t, tr, MarkLinkArea, "LinkArea")[0]
	e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))
	if got, ok := e.fp.links.clicked(e.context()); !ok || got != *tg {
		t.Fatalf("click after an abandoned flow = %+v, %t; want the link's target and true", got, ok)
	}
}

// TestFlowZeroGlyphs: a fragment with no glyphs is an error, not a silent skip, so a rune that
// shapes to nothing cannot stall textRun's loop. No input shapes to nothing in this font, so
// the check is called directly.
func TestFlowZeroGlyphs(t *testing.T) {
	e := newFlowEnv(t, 800)
	f, err := newFlow(e.context(), e.fp, &Trace{})
	if err != nil {
		t.Fatalf("newFlow: %v", err)
	}
	if perr := f.place(plain("x"), e.fp.base, "x", nil, 0, 0); perr == nil {
		t.Fatal("a fragment with no glyphs: no error")
	}
}
