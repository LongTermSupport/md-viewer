package render

import (
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// links holds one click gesture per link target (LINK-1). Every run of one link shares the
// target pointer (DOC-4), so every fragment of the link shares one gesture. order keeps the
// polling deterministic.
type links struct {
	clicks map[*doc.Target]*gesture.Click
	order  []*doc.Target
}

// newLinks returns an empty set of link gestures.
func newLinks() *links {
	return &links{}
}

// clicked drains the link gestures and returns the first click, if any (LINK-1). It is polled
// once per frame.
func (l *links) clicked(gtx layout.Context) (doc.Target, bool) {
	return doc.Target{}, false
}

// flowParams are the inputs to layoutFlow that do not change within one call.
type flowParams struct {
	shaper    *text.Shaper
	base      TextStyle // the block's base style; its size times its line height is each line's height
	palette   Palette
	inHeading bool // RunStyle's code rule (STYLE-TEXT-5)
	links     *links
}

// layoutFlow lays out runs as wrapped lines and records what it drew in tr, which may be nil.
func layoutFlow(gtx layout.Context, fp flowParams, runs []doc.Run, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{}, nil
}
