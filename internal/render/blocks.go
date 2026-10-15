package render

import (
	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// layoutBlock lays out one block at the width of gtx and returns its size. Stub: an empty
// dispatcher that lays out nothing.
func layoutBlock(gtx layout.Context, b doc.Block, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{}, nil
}
