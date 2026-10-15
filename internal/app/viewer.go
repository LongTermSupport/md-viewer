package app

import (
	"io"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/LongTermSupport/md-viewer/internal/render"
)

// Viewer is the per-window state: the list position and the zoom (ARCHITECTURE "internal/app").
type Viewer struct {
	view  *render.View
	theme *material.Theme
	list  widget.List
	zoom  Zoom
}

// NewViewer makes the viewer of v, painted in p, whose links open with o and whose diagnostics go
// to stderr.
func NewViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer) *Viewer {
	return newViewer(v, p, o, stderr, goSpawn)
}

// newViewer is NewViewer with the function that runs the opener's wait. spawn is goSpawn in the
// window; the tests pass one that records the functions. The theme is material's, with the
// scrollbar's indicator in the palette's Fg (STYLE-TEXT-1, SCROLL-4).
func newViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer, spawn func(func())) *Viewer {
	th := material.NewTheme()
	th.Palette.Fg = p.Fg
	return &Viewer{
		view:  v,
		theme: th,
		list:  widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

// goSpawn runs f in its own goroutine (LINK-2: the viewer does not wait for the opener).
func goSpawn(f func()) {
	go f()
}

// Frame handles the input of the frame, lays out the document, and reports quit and the error
// that ends the window (CLI-6). For now it only lays out.
func (vw *Viewer) Frame(gtx layout.Context) (bool, error) {
	material.List(vw.theme, &vw.list).Layout(gtx, vw.view.Len(), vw.element)
	return false, vw.view.Err()
}

// element lays out block i at the zoom (STYLE-ZOOM-3). The list has given it the width less the
// scrollbar gutter, which is not zoomed, so only the document's own metric changes.
func (vw *Viewer) element(gtx layout.Context, i int) layout.Dimensions {
	gtx.Metric = vw.zoom.Metric(gtx.Metric)
	return vw.view.Layout(gtx, i, nil)
}
