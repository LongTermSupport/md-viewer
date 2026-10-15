package render

import (
	"image"
	"image/color"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// MarkKind says what a Mark records (ARCHITECTURE "internal/render"). The zero value is
// MarkGlyphs.
type MarkKind uint8

// The kinds of mark. The Mark prefix avoids the clash with the type names HeadingRule and
// Bullet (ARCHITECTURE).
const (
	MarkGlyphs      MarkKind = iota // text drawn in Rect
	MarkCodeBg                      // a code run's rounded background (STYLE-TEXT-5)
	MarkCodeBlockBg                 // a code block's background (STYLE-CODE-1)
	MarkHeadingRule                 // the rule under a heading (STYLE-TEXT-2)
	MarkQuoteBar                    // a block quote's bar (STYLE-QUOTE-1)
	MarkRuleBar                     // a thematic break (STYLE-RULE-1)
	MarkBullet                      // a list bullet (STYLE-LIST-2)
	MarkNumber                      // an ordered list marker (STYLE-LIST-3)
	MarkTaskBox                     // a task list check box (STYLE-LIST-5)
	MarkTableLine                   // a table border (STYLE-TABLE-1)
	MarkRowFill                     // a table row's fill (STYLE-TABLE-3)
	MarkImageBox                    // an image's box before its pixels (STYLE-IMG-5)
	MarkImagePixels                 // decoded image pixels (STYLE-IMG-1)
	MarkPlaceholder                 // an image placeholder (STYLE-IMG-3)
	MarkStrike                      // a strikethrough line (STYLE-TEXT-4)
	MarkLinkArea                    // a link's clickable area (LINK-1, STYLE-TEXT-3)
)

// Trace records what layout drew. Its origin is the top-left corner of what the trace was made
// for: the flow's in a flow trace, the list element's (gap and padding included) in a View
// trace. Nested layout translates by its origin (push and pop), so a sub-layout records in the
// coordinates of the trace it was given. Tests read it to assert structure.
type Trace struct {
	Marks  []Mark
	origin image.Point // added to every mark's Rect as it is recorded
}

// Mark is one thing layout drew. Rect is in pixels relative to the trace's origin (see Trace,
// and ARCHITECTURE "internal/render", mark rectangles).
type Mark struct {
	Kind   MarkKind
	Rect   image.Rectangle
	Colour color.NRGBA
	Text   string      // for Glyphs and Number: the text drawn in Rect
	Link   *doc.Target // for LinkArea
}

// add appends m to the trace, moved by the origin. A nil Trace records nothing, so layout code
// need not check for one (ARCHITECTURE "internal/render": tr may be nil).
func (t *Trace) add(m Mark) {
	if t == nil {
		return
	}
	m.Rect = m.Rect.Add(t.origin)
	t.Marks = append(t.Marks, m)
}

// count is the number of marks recorded so far. A nil Trace has none.
func (t *Trace) count() int {
	if t == nil {
		return 0
	}
	return len(t.Marks)
}

// shiftFrom moves the marks from index i on by d, so that a line laid out in a macro and then
// moved takes its marks with it (STYLE-TABLE-4). A nil trace has none.
func (t *Trace) shiftFrom(i int, d image.Point) {
	if t == nil {
		return
	}
	for j := i; j < len(t.Marks); j++ {
		t.Marks[j].Rect = t.Marks[j].Rect.Add(d)
	}
}

// push moves the origin by d, so that marks recorded inside a nested layout are in the outer
// trace's coordinates, and returns the previous origin for pop. A nil Trace has no origin.
func (t *Trace) push(d image.Point) image.Point {
	if t == nil {
		return image.Point{}
	}
	prev := t.origin
	t.origin = t.origin.Add(d)
	return prev
}

// pop restores the origin that the matching push returned.
func (t *Trace) pop(prev image.Point) {
	if t == nil {
		return
	}
	t.origin = prev
}
