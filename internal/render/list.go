package render

import (
	"fmt"
	"image"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// layoutList lays out a list's items, indented from its left edge (STYLE-LIST-1). Stub: the M5
// placeholder, until the list is drawn.
func layoutList(gtx layout.Context, ctx blockCtx, l doc.List, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}, nil
}

// leadsWithText reports whether a block's first line is text, found through quotes and nested
// lists (STYLE-LIST-2). An empty item is an empty line of text. A rule, a table or an image has
// no text line, and an empty quote or list has none either. A block with no layout is an error
// naming its type.
func leadsWithText(b doc.Block) (bool, error) {
	switch b := b.(type) {
	case doc.Paragraph, doc.Heading, doc.CodeBlock:
		return true, nil
	case doc.Quote:
		if len(b.Blocks) == 0 {
			return false, nil
		}
		return leadsWithText(b.Blocks[0])
	case doc.List:
		if len(b.Items) == 0 {
			return false, nil
		}
		first := b.Items[0]
		if len(first.Blocks) == 0 {
			return true, nil
		}
		return leadsWithText(first.Blocks[0])
	case doc.Rule, doc.Table, doc.Image:
		return false, nil
	default:
		return false, fmt.Errorf("render: no first line for block %T", b)
	}
}
