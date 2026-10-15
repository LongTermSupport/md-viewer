package app

import (
	"fmt"
	"io"
	"math"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/LongTermSupport/md-viewer/internal/render"
)

// Distances of the keys, in dp at the zoom (SCROLL-2). A line is lineStep. A page is the
// viewport less pageKeep, and at least pageKeep.
const (
	lineStep = 40
	pageKeep = 40
)

// Viewer is the per-window state: the list position, the zoom and the key filters
// (ARCHITECTURE "internal/app").
type Viewer struct {
	view  *render.View
	theme *material.Theme
	list  widget.List
	zoom  Zoom
	keys  []event.Filter
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
		keys:  keyFilters(),
	}
}

// goSpawn runs f in its own goroutine (LINK-2: the viewer does not wait for the opener).
func goSpawn(f func()) {
	go f()
}

// Frame handles the input of the frame, lays out the document, and reports quit and the error
// that ends the window (CLI-6). The input comes first, so the layout shows its result.
func (vw *Viewer) Frame(gtx layout.Context) (bool, error) {
	quit, err := vw.keyInput(gtx)
	if err != nil {
		return false, err
	}
	material.List(vw.theme, &vw.list).Layout(gtx, vw.view.Len(), vw.element)
	if err := vw.view.Err(); err != nil {
		return quit, err
	}
	return quit, nil
}

// keyInput applies the key presses of the frame in the order they arrived, and reports quit.
// One call with every filter keeps that order; a filter at a time would not.
func (vw *Viewer) keyInput(gtx layout.Context) (bool, error) {
	quit := false
	for {
		ev, ok := gtx.Event(vw.keys...)
		if !ok {
			return quit, nil
		}
		// Only key events match the key filters, so the switch has one case.
		switch e := ev.(type) {
		case key.Event:
			q, err := vw.act(gtx, ActionFor(e))
			if err != nil {
				return false, err
			}
			quit = quit || q
		}
	}
}

// act does one action. It reports quit for ActionQuit. The distances of the scroll keys are dp at
// the zoom, so they are read from a context with the zoomed metric.
func (vw *Viewer) act(gtx layout.Context, a Action) (bool, error) {
	zg := gtx
	zg.Metric = vw.zoom.Metric(gtx.Metric)
	page := max(gtx.Constraints.Max.Y-zg.Dp(pageKeep), zg.Dp(pageKeep))
	switch a {
	case ActionNone:
	case ActionLineDown:
		vw.list.Position.Offset += zg.Dp(lineStep)
	case ActionLineUp:
		vw.list.Position.Offset -= zg.Dp(lineStep)
	case ActionPageDown:
		vw.list.Position.Offset += page
	case ActionPageUp:
		vw.list.Position.Offset -= page
	case ActionTop:
		vw.list.Position = layout.Position{}
	case ActionBottom:
		vw.list.Position = layout.Position{First: vw.view.Len()}
	case ActionZoomIn:
		vw.setZoom(vw.zoom.In())
	case ActionZoomOut:
		vw.setZoom(vw.zoom.Out())
	case ActionZoomReset:
		vw.setZoom(vw.zoom.Reset())
	case ActionQuit:
		return true, nil
	default:
		return false, fmt.Errorf("app: no handler for %v", a)
	}
	return false, nil
}

// setZoom changes the zoom and keeps the block at the top of the view there. The offset into that
// block is scaled by the ratio of the new zoom to the old, rounded to the nearest pixel (ZOOM-5).
// The list's limits apply on the next layout.
func (vw *Viewer) setZoom(z Zoom) {
	oldPct, newPct := vw.zoom.Percent(), z.Percent()
	vw.list.Position.Offset = int(math.Round(float64(vw.list.Position.Offset) * float64(newPct) / float64(oldPct)))
	vw.zoom = z
}

// element lays out block i at the zoom (STYLE-ZOOM-3). The list has given it the width less the
// scrollbar gutter, which is not zoomed, so only the document's own metric changes.
func (vw *Viewer) element(gtx layout.Context, i int) layout.Dimensions {
	gtx.Metric = vw.zoom.Metric(gtx.Metric)
	return vw.view.Layout(gtx, i, nil)
}
