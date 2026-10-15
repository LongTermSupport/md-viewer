package render

import (
	"fmt"
	"image"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// layoutTable is a stub for the red commit: it lays out nothing.
func layoutTable(gtx layout.Context, ctx blockCtx, t doc.Table, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}, nil
}

// cellAlign is how the lines of a column's cells sit (STYLE-TABLE-4): Left and a body None are at
// the start, Centre and a header None are centred, and Right is at the end. An alignment that is
// not one of the four is an error that names it.
func cellAlign(a doc.Align, header bool) (textAlign, error) {
	switch a {
	case doc.AlignLeft:
		return alignStart, nil
	case doc.AlignCentre:
		return alignCentre, nil
	case doc.AlignRight:
		return alignEnd, nil
	case doc.AlignNone:
		if header {
			return alignCentre, nil
		}
		return alignStart, nil
	default:
		return alignStart, fmt.Errorf("render: no cell alignment for alignment %d", a)
	}
}
