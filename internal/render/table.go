package render

import (
	"image"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// layoutTable is a stub for the red commit: it lays out nothing.
func layoutTable(gtx layout.Context, ctx blockCtx, t doc.Table, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}, nil
}

// cellAlign is a stub for the red commit: every cell is at the start.
func cellAlign(a doc.Align, header bool) (textAlign, error) {
	return alignStart, nil
}
