package render

import (
	"image"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// layoutList lays out a list's items, indented from its left edge (STYLE-LIST-1). Stub: the M5
// placeholder, until the list is drawn.
func layoutList(gtx layout.Context, ctx blockCtx, l doc.List, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}, nil
}

// leadsWithText reports whether a block's first line is text (STYLE-LIST-2). Stub.
func leadsWithText(b doc.Block) (bool, error) {
	return false, nil
}
