package render

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/text"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// View lays out one document (ARCHITECTURE "internal/render"). It owns the per-link click state,
// so it lives as long as the window.
type View struct {
	doc     *doc.Document
	shaper  *text.Shaper
	palette Palette
	links   *links
	err     error // the first layout error; kept, never cleared
}

// NewView makes a View of d, shaped by sh and painted in palette p. M8 adds the image cache.
func NewView(d *doc.Document, sh *text.Shaper, p Palette) *View {
	return &View{doc: d, shaper: sh, palette: p, links: newLinks()}
}

// Len is the number of top-level blocks.
func (v *View) Len() int {
	return len(v.doc.Blocks)
}

// Layout draws block i as a layout.ListElement: the gap above it, and for the first and last
// block the content padding (DOC-LAYOUT-1, DOC-LAYOUT-3). The element is gtx.Constraints.Max.X
// wide, and the padding is chosen from that width. tr may be nil, and its origin is the element's
// top-left corner. A failing element returns Dimensions{} and sets Err, which keeps the first
// error.
func (v *View) Layout(gtx layout.Context, i int, tr *Trace) layout.Dimensions {
	dims, err := v.layoutElement(gtx, i, tr)
	if err != nil {
		if v.err == nil {
			v.err = err
		}
		return layout.Dimensions{}
	}
	return dims
}

// layoutElement is Layout for an element that can fail.
func (v *View) layoutElement(gtx layout.Context, i int, tr *Trace) (layout.Dimensions, error) {
	if i < 0 || i >= len(v.doc.Blocks) {
		return layout.Dimensions{}, fmt.Errorf("render: no block at index %d", i)
	}
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(Padding(gtx.Metric.PxToDp(width)))
	top := pad // the first element starts below the padding
	if i > 0 {
		gap, err := blockGap(v.doc.Blocks[i-1], v.doc.Blocks[i])
		if err != nil {
			return layout.Dimensions{}, err
		}
		top = gtx.Dp(gap)
	}
	bottom := 0
	if i == len(v.doc.Blocks)-1 {
		bottom = pad // the last element ends above the padding
	}

	b := v.doc.Blocks[i]
	ctx := blockCtx{text: v.palette, root: v.palette, shaper: v.shaper, links: v.links}
	bgtx := gtx
	bgtx.Constraints = layout.Constraints{Max: image.Pt(max(width-2*pad, 0), gtx.Constraints.Max.Y)}
	dims, err := at(gtx, tr, image.Pt(pad, top), func() (layout.Dimensions, error) {
		return layoutBlock(bgtx, ctx, b, tr)
	})
	if err != nil {
		return layout.Dimensions{}, err
	}
	return layout.Dimensions{Size: image.Pt(width, top+dims.Size.Y+bottom)}, nil
}

// Err is the first layout error, or nil. It is never cleared.
func (v *View) Err() error {
	return v.err
}

// Clicked reports the first link clicked since the last poll, and consumes its events (LINK-1).
// It is polled once per frame.
func (v *View) Clicked(gtx layout.Context) (doc.Target, bool) {
	return v.links.clicked(gtx)
}
