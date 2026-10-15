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
// Bullet (ARCHITECTURE, F2).
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

// Trace records what layout drew, in pixels relative to the block's top-left corner. The view
// keeps one per visible block; tests read it to assert structure.
type Trace struct{ Marks []Mark }

// Mark is one thing layout drew. Rect is in pixels relative to the flow's top-left corner
// (ARCHITECTURE "internal/render", mark rectangles).
type Mark struct {
	Kind   MarkKind
	Rect   image.Rectangle
	Colour color.NRGBA
	Text   string      // for Glyphs and Number: the text drawn in Rect
	Link   *doc.Target // for LinkArea
}

// add appends m to the trace. A nil Trace records nothing, so layout code need not check for
// one (ARCHITECTURE "internal/render": tr may be nil).
func (t *Trace) add(m Mark) {
	if t == nil {
		return
	}
	t.Marks = append(t.Marks, m)
}
