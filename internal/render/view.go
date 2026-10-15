package render

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// View lays out one document (ARCHITECTURE "internal/render"). It owns the per-link click state,
// so it lives as long as the window. Stub: not yet implemented.
type View struct {
	doc     *doc.Document
	shaper  *text.Shaper
	palette Palette
	links   *links
	err     error
}

// NewView makes a View of d, shaped by sh and painted in palette p. Stub: not yet implemented.
func NewView(d *doc.Document, sh *text.Shaper, p Palette) *View {
	return &View{}
}

// Len is the number of top-level blocks. Stub: not yet implemented.
func (v *View) Len() int {
	return 0
}

// Layout draws block i as a layout.ListElement. Stub: not yet implemented.
func (v *View) Layout(gtx layout.Context, i int, tr *Trace) layout.Dimensions {
	return layout.Dimensions{}
}

// Err is the first layout error. Stub: not yet implemented.
func (v *View) Err() error {
	return nil
}

// Clicked reports the first link clicked since the last poll. Stub: not yet implemented.
func (v *View) Clicked(gtx layout.Context) (doc.Target, bool) {
	return doc.Target{}, false
}
