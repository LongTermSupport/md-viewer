package app

import (
	"io"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/render"
)

// Viewer is the per-window state: the list position, the zoom, the wheel accumulator and the
// opener (ARCHITECTURE "internal/app"). Stub: the list is a layout.List until the scrollbar's
// widget is added, which needs the module's go.sum to name its dependency.
type Viewer struct {
	list layout.List
	zoom Zoom
}

// NewViewer makes the viewer of v, painted in p, whose links open with o and whose diagnostics go
// to stderr.
func NewViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer) *Viewer {
	return newViewer(v, p, o, stderr, goSpawn)
}

// newViewer is NewViewer with the function that runs the opener's wait. spawn is goSpawn in the
// window; the tests pass one that records the functions. Stub: an empty viewer.
func newViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer, spawn func(func())) *Viewer {
	return &Viewer{}
}

// goSpawn runs f in its own goroutine (LINK-2: the viewer does not wait for the opener).
func goSpawn(f func()) {
	go f()
}

// Frame handles the input of the frame, lays out the document, and reports quit and the error
// that ends the window (CLI-6). Stub: lays out nothing and reports no input.
func (vw *Viewer) Frame(gtx layout.Context) (bool, error) {
	return false, nil
}
