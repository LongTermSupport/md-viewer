package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
	"math"
	"os"
	"sync"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/LongTermSupport/md-viewer/internal/doc"
	"github.com/LongTermSupport/md-viewer/internal/render"
)

// Messages of the link actions. Each ends with a newline (LINK-1, LINK-2).
const (
	msgLinkNotFound   = progName + ": link target not found: %s\n"
	msgLinkFragment   = progName + ": in-page links are not followed: %s\n"
	msgLinkRefused    = progName + ": link not opened: %s: only http, https, mailto and local files are allowed\n"
	msgLinkOpenFailed = progName + ": cannot open link: %s: %s\n"
)

// Distances of the keys, in dp at the zoom (SCROLL-2). A line is lineStep. A page is the
// viewport less pageKeep, and at least pageKeep.
const (
	lineStep = 40
	pageKeep = 40
)

// The wheel. wheelZoomStep is the accumulated Ctrl+wheel distance, in px, that changes the zoom by
// one level (ZOOM-4). scrollRangeMax is the largest wheel distance the wheel area takes, so it
// takes every vertical event (ARCHITECTURE "Input routing").
const (
	wheelZoomStep  = 10
	scrollRangeMax = 1 << 30
)

// Viewer is the per-window state: the list position, the zoom, the wheel accumulator and the key
// filters (ARCHITECTURE "internal/app").
type Viewer struct {
	view     *render.View
	theme    *material.Theme
	list     widget.List
	zoom     Zoom
	wheelAcc float32 // the Ctrl+wheel distance not yet zoomed by, in px (ZOOM-4)
	keys     []event.Filter
	opener   Opener
	stderr   io.Writer
	spawn    func(func())

	// stderrMu serialises the viewer's writes to stderr: the frame and the opener's goroutine both
	// write there.
	stderrMu sync.Mutex

	// asyncMu guards asyncErr, the error of a write made by a spawned function, which the next
	// frame returns, and invalidate, the window's Invalidate, which that function calls after it
	// records the error.
	asyncMu    sync.Mutex
	asyncErr   error
	invalidate func()
}

// NewViewer makes the viewer of v, painted in p, whose links open with o and whose diagnostics go
// to stderr. The viewer serialises its own writes, from the frame and from the opener's goroutine.
// stderr is also written by render.Images from layout, on the frame goroutine, so stderr must be
// safe for concurrent use: *os.File, which main passes, is.
func NewViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer) *Viewer {
	return newViewer(v, p, o, stderr, goSpawn)
}

// newViewer is NewViewer with the function that runs the opener's wait. spawn is goSpawn in the
// window; the tests pass one that records the functions. The theme is material's, with the
// scrollbar's indicator in the palette's Fg (SCROLL-4). Until Bind, invalidate does nothing.
func newViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer, spawn func(func())) *Viewer {
	th := material.NewTheme()
	th.Palette.Fg = p.Fg
	return &Viewer{
		view:       v,
		theme:      th,
		list:       widget.List{List: layout.List{Axis: layout.Vertical}},
		keys:       keyFilters(),
		opener:     o,
		stderr:     stderr,
		spawn:      spawn,
		invalidate: func() {},
	}
}

// Bind sets the function that a failed write from a spawned function calls, so that the next
// frame comes at once (LINK-2). It is the window's Invalidate.
func (vw *Viewer) Bind(invalidate func()) {
	vw.asyncMu.Lock()
	defer vw.asyncMu.Unlock()
	vw.invalidate = invalidate
}

// goSpawn runs f in its own goroutine (LINK-2: the viewer does not wait for the opener).
func goSpawn(f func()) {
	go f()
}

// Frame handles the input of the frame, lays out the document, and reports quit and the error
// that ends the window (CLI-6). The input comes first, so the layout shows its result.
func (vw *Viewer) Frame(gtx layout.Context) (bool, error) {
	if err := vw.takeAsyncErr(); err != nil {
		return false, err
	}
	quit, err := vw.keyInput(gtx)
	if err != nil {
		return false, err
	}
	vw.wheelInput(gtx)
	linkErr := vw.linkInput(gtx)
	material.List(vw.theme, &vw.list).Layout(gtx, vw.view.Len(), vw.element)
	vw.wheelArea(gtx)
	return quit, frameErrors(linkErr, vw.view.Err())
}

// frameErrors is the error of a frame, from a link write error and a layout error. Both can
// happen in one frame. It is one line, joined by "; ", so main's "md-viewer: " prefix (CLI-6)
// covers both; errors.Join would put the second error on a line of its own.
func frameErrors(linkErr, layoutErr error) error {
	switch {
	case linkErr == nil:
		return layoutErr
	case layoutErr == nil:
		return linkErr
	default:
		return fmt.Errorf("%w; %w", linkErr, layoutErr)
	}
}

// linkInput acts on the link clicked since the last frame, if any (LINK-1). The opener itself
// runs in a spawned function, so the frame never waits for it.
func (vw *Viewer) linkInput(gtx layout.Context) error {
	target, ok := vw.view.Clicked(gtx)
	if !ok {
		return nil
	}
	return vw.followLink(target)
}

// followLink does what LINK-1 says for a clicked target. A message that cannot be written is
// returned.
func (vw *Viewer) followLink(t doc.Target) error {
	switch t.Kind {
	case doc.External:
		vw.open(t)
		return nil
	case doc.LocalFile:
		// Stat runs in the frame: LINK-1 needs the target to exist before the opener starts. A slow
		// file system (a hung network mount, say) delays the frame by as long.
		_, err := os.Stat(t.Value)
		switch {
		case err == nil:
			vw.open(t)
			return nil
		case errors.Is(err, fs.ErrNotExist):
			return vw.printf(msgLinkNotFound, t.Value)
		default:
			return vw.printf(msgLinkOpenFailed, t.Value, statReason(err))
		}
	case doc.Fragment:
		return vw.printf(msgLinkFragment, t.Dest)
	case doc.Refused:
		return vw.printf(msgLinkRefused, t.Dest)
	default:
		return fmt.Errorf("app: no action for a link of kind %d", uint8(t.Kind))
	}
}

// open runs the opener on t in a spawned function (LINK-2). The opener's error, if any, is
// printed from that function. A failed write there is kept for the next frame, and the window is
// invalidated so that frame comes at once.
func (vw *Viewer) open(t doc.Target) {
	vw.spawn(func() {
		err := vw.opener.Open(context.Background(), t.Value)
		if err == nil {
			return
		}
		werr := vw.printf(msgLinkOpenFailed, t.Value, err.Error())
		if werr == nil {
			return
		}
		vw.asyncMu.Lock()
		vw.asyncErr = errors.Join(vw.asyncErr, werr)
		invalidate := vw.invalidate
		vw.asyncMu.Unlock()
		invalidate()
	})
}

// takeAsyncErr returns the error kept from a spawned function, and clears it. The spawned function
// invalidates the window after it keeps the error, so the frame that returns it comes at once
// (LINK-2); a window that closes first never sees it.
func (vw *Viewer) takeAsyncErr() error {
	vw.asyncMu.Lock()
	defer vw.asyncMu.Unlock()
	err := vw.asyncErr
	vw.asyncErr = nil
	return err
}

// printf writes a diagnostic to stderr and returns the write's error.
func (vw *Viewer) printf(format string, args ...any) error {
	vw.stderrMu.Lock()
	defer vw.stderrMu.Unlock()
	_, err := fmt.Fprintf(vw.stderr, format, args...)
	return err
}

// statReason is the operating-system text of a stat error, without the path it names.
func statReason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// wheelInput applies the vertical wheel events of the frame, in order. Without Ctrl a scroll
// moves the list by its pixels (SCROLL-3). With Ctrl it is a zoom step (ZOOM-4), and never scrolls.
func (vw *Viewer) wheelInput(gtx layout.Context) {
	filter := pointer.Filter{
		Target:  vw,
		Kinds:   pointer.Scroll,
		ScrollY: pointer.ScrollRange{Min: -scrollRangeMax, Max: scrollRangeMax},
	}
	for {
		ev, ok := gtx.Event(filter)
		if !ok {
			return
		}
		// Only pointer events match the wheel filter, so the switch has one case.
		switch e := ev.(type) {
		case pointer.Event:
			if e.Modifiers.Contain(key.ModCtrl) {
				vw.wheelZoom(e.Scroll.Y)
			} else {
				vw.list.Position.Offset += int(math.Round(float64(e.Scroll.Y)))
			}
		}
	}
}

// wheelZoom adds the Ctrl+wheel distance dy, in px, to the accumulator. When the sum reaches
// -wheelZoomStep or +wheelZoomStep the zoom goes one level in or out, and the sum is reset to 0,
// even at a limit where the zoom does not change (ZOOM-4).
func (vw *Viewer) wheelZoom(dy float32) {
	vw.wheelAcc += dy
	switch {
	case vw.wheelAcc <= -wheelZoomStep:
		vw.setZoom(vw.zoom.In())
		vw.wheelAcc = 0
	case vw.wheelAcc >= wheelZoomStep:
		vw.setZoom(vw.zoom.Out())
		vw.wheelAcc = 0
	}
}

// wheelArea takes every vertical wheel event of the window (ARCHITECTURE "Input routing"). It is
// added after the list, so it is on top of it. It is inside a pointer.PassOp, so presses still
// reach the scrollbar and the links below it (SCROLL-4, STYLE-TEXT-3).
func (vw *Viewer) wheelArea(gtx layout.Context) {
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, vw)
	area.Pop()
	pass.Pop()
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
