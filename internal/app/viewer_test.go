package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
	"github.com/LongTermSupport/md-viewer/internal/render"
)

// defaultSize is the window in px for most viewer tests, at the metric 1 px per dp.
var defaultSize = image.Pt(800, 600)

// fakeOpener records the targets it is asked to open, and returns a scripted error.
type fakeOpener struct {
	calls []string
	err   error
}

func (f *fakeOpener) Open(_ context.Context, target string) error {
	f.calls = append(f.calls, target)
	return f.err
}

// sharedShaper loads the embedded faces once for the whole package's tests.
var sharedShaper = sync.OnceValues(func() (*text.Shaper, error) {
	f, err := render.LoadFaces()
	if err != nil {
		return nil, err
	}
	return render.NewShaper(f), nil
})

func shaper(t *testing.T) *text.Shaper {
	t.Helper()
	sh, err := sharedShaper()
	if err != nil {
		t.Fatalf("LoadFaces: %v", err)
	}
	return sh
}

// testdataDir is the absolute path of the repository's testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata"))
	if err != nil {
		t.Fatalf("Abs of testdata: %v", err)
	}
	return dir
}

// parse makes the document of src, whose relative links resolve in dir.
func parse(t *testing.T, src, dir string) *doc.Document {
	t.Helper()
	d, err := doc.Parse([]byte(src), dir)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

// longDoc is 60 one-line paragraphs. At zoom 100 in a 600 px high view it is about 2474 px tall.
func longDoc(t *testing.T) *doc.Document {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "Line %d.\n\n", i)
	}
	return parse(t, b.String(), t.TempDir())
}

// stroke is one key event: a press, or a release when up is set.
type stroke struct {
	name key.Name
	mods key.Modifiers
	up   bool
}

// event is the key event the window sends for s.
func (s stroke) event() key.Event {
	state := key.Press
	if s.up {
		state = key.Release
	}
	return key.Event{Name: s.name, Modifiers: s.mods, State: state}
}

// viewerEnv runs a Viewer the way the window does: a frame, then the router's frame.
type viewerEnv struct {
	t       *testing.T
	ops     op.Ops
	router  input.Router
	doc     *doc.Document
	images  *render.Images
	view    *render.View
	vw      *Viewer
	opener  *fakeOpener
	size    image.Point
	metric  unit.Metric
	spawned []func()
}

// newEnv makes a viewer of d in a window of size px, whose stderr is stderr. The viewer's spawn
// records its functions; runSpawned runs them.
func newEnv(t *testing.T, d *doc.Document, size image.Point, stderr io.Writer) *viewerEnv {
	t.Helper()
	images, err := render.NewImages(d, io.Discard)
	if err != nil {
		t.Fatalf("NewImages: %v", err)
	}
	e := &viewerEnv{
		t:      t,
		doc:    d,
		images: images,
		opener: &fakeOpener{},
		size:   size,
		metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	e.view = render.NewView(d, shaper(t), render.Light, images)
	e.vw = newViewer(e.view, render.Light, e.opener, stderr, func(f func()) {
		e.spawned = append(e.spawned, f)
	})
	return e
}

// step runs one frame: the viewer's Frame, then the router's Frame. A frame that fails ends
// there, as the window does.
func (e *viewerEnv) step() (bool, error) {
	e.ops.Reset()
	gtx := layout.Context{
		Ops:         &e.ops,
		Source:      e.router.Source(),
		Metric:      e.metric,
		Constraints: layout.Exact(e.size),
	}
	quit, err := e.vw.Frame(gtx)
	if err != nil {
		return quit, err
	}
	e.router.Frame(&e.ops)
	return quit, nil
}

// frame runs one frame, and fails the test if it returns an error. It returns quit.
func (e *viewerEnv) frame() bool {
	e.t.Helper()
	quit, err := e.step()
	if err != nil {
		e.t.Fatalf("Frame: %v", err)
	}
	return quit
}

// press sends each stroke, one frame apart. It returns quit from the last frame.
func (e *viewerEnv) press(strokes ...stroke) bool {
	e.t.Helper()
	quit := false
	for _, s := range strokes {
		e.router.Queue(s.event())
		quit = e.frame()
	}
	return quit
}

// centre is the middle of the window.
func (e *viewerEnv) centre() f32.Point {
	return f32.Pt(float32(e.size.X)/2, float32(e.size.Y)/2)
}

// wheel queues a vertical scroll of y px at the window's centre, with mods held.
func (e *viewerEnv) wheel(y float32, mods key.Modifiers) {
	e.scroll(e.centre(), 0, y, mods)
}

// scroll queues a scroll of (x, y) px at p, with mods held.
func (e *viewerEnv) scroll(p f32.Point, x, y float32, mods key.Modifiers) {
	e.router.Queue(pointer.Event{
		Kind:      pointer.Scroll,
		Position:  p,
		Scroll:    f32.Pt(x, y),
		Source:    pointer.Mouse,
		Modifiers: mods,
	})
}

// click is a primary click at p (LINK-1). The pointer moves onto p and a frame runs, as it does in
// the window, so the press is hit-tested where the pointer hovers. Then the press and the release
// are queued for the next frame to poll.
func (e *viewerEnv) click(p f32.Point) {
	e.t.Helper()
	e.router.Queue(pointer.Event{Kind: pointer.Move, Position: p, Source: pointer.Mouse})
	e.frame()
	e.router.Queue(
		pointer.Event{Kind: pointer.Press, Position: p, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: p, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
}

// runSpawned runs the functions the viewer spawned, in order, and clears them.
func (e *viewerEnv) runSpawned() {
	fs := e.spawned
	e.spawned = nil
	for _, f := range fs {
		f()
	}
}

// absY is the absolute scroll position: the heights of the blocks above the first visible one,
// plus Offset. The heights come from a separate View of the same document, laid out at the
// window's width less the 10 px scrollbar gutter (SCROLL-4), at the viewer's zoom.
func (e *viewerEnv) absY() int {
	e.t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      e.vw.zoom.Metric(e.metric),
		Constraints: layout.Constraints{Max: image.Pt(e.size.X-10, e.size.Y)},
	}
	ref := render.NewView(e.doc, shaper(e.t), render.Light, e.images)
	sum := 0
	for i := range e.vw.list.Position.First {
		sum += ref.Layout(gtx, i, nil).Size.Y
	}
	if err := ref.Err(); err != nil {
		e.t.Fatalf("layout of the reference view: %v", err)
	}
	return sum + e.vw.list.Position.Offset
}

// linkCentre is the centre of the first link area of block 0 of d, laid out at width px. The
// viewer lays block 0 at the top-left of the list, so this is where a click must go.
func linkCentre(t *testing.T, d *doc.Document, images *render.Images, width int) f32.Point {
	t.Helper()
	v := render.NewView(d, shaper(t), render.Light, images)
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(width, 1<<20)},
	}
	var tr render.Trace
	v.Layout(gtx, 0, &tr)
	for _, m := range tr.Marks {
		if m.Kind == render.MarkLinkArea {
			return f32.Pt(
				float32(m.Rect.Min.X+m.Rect.Dx()/2),
				float32(m.Rect.Min.Y+m.Rect.Dy()/2),
			)
		}
	}
	t.Fatal("block 0 has no link area")
	return f32.Point{}
}

// linkDoc is one paragraph holding the link src, first in the document.
func linkDoc(t *testing.T, src string) *doc.Document {
	t.Helper()
	return parse(t, src+"\n", t.TempDir())
}

// imagesOf is the images of d, as a window would make them.
func imagesOf(t *testing.T, d *doc.Document) *render.Images {
	t.Helper()
	images, err := render.NewImages(d, io.Discard)
	if err != nil {
		t.Fatalf("NewImages: %v", err)
	}
	return images
}

// The keys of the scroll tests.
var (
	keyDown     = stroke{name: key.NameDownArrow}
	keyUp       = stroke{name: key.NameUpArrow}
	keyPageDown = stroke{name: key.NamePageDown}
	keyPageUp   = stroke{name: key.NamePageUp}
	keySpace    = stroke{name: key.NameSpace}
	keyHome     = stroke{name: key.NameHome}
	keyEnd      = stroke{name: key.NameEnd}
	keyZoomIn   = stroke{name: "=", mods: key.ModCtrl}
	keyZoomOut  = stroke{name: "-", mods: key.ModCtrl}
	keyZoom0    = stroke{name: "0", mods: key.ModCtrl}
)

// TestViewerKeysScroll checks SCROLL-2 on a long document: the distances in dp, the minimum
// page, and the limits at the top and the bottom.
func TestViewerKeysScroll(t *testing.T) {
	tests := []struct {
		name string
		size image.Point
		keys []stroke
		want int
	}{
		{"down moves 40 px at zoom 100", defaultSize, []stroke{keyDown}, 40},
		{"down moves 80 px at zoom 200", defaultSize,
			[]stroke{keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyDown}, 80},
		{"down moves 27 px at zoom 67", defaultSize,
			[]stroke{keyZoomOut, keyZoomOut, keyZoomOut, keyZoomOut, keyDown}, 27},
		{"down, down, up ends at 40", defaultSize, []stroke{keyDown, keyDown, keyUp}, 40},
		{"page down moves the viewport less 40 dp", defaultSize, []stroke{keyPageDown}, 560},
		{"page down, page down, page up ends at 560", defaultSize,
			[]stroke{keyPageDown, keyPageDown, keyPageUp}, 560},
		{"space moves as page down", defaultSize, []stroke{keySpace}, 560},
		{"space, space, shift+space ends at 560", defaultSize,
			[]stroke{keySpace, keySpace, {name: key.NameSpace, mods: key.ModShift}}, 560},
		{"page down is at least 40 dp in a short window", image.Pt(800, 60),
			[]stroke{keyPageDown}, 40},
		{"home after page down goes to the top", defaultSize, []stroke{keyPageDown, keyHome}, 0},
		{"up at the top does not move", defaultSize, []stroke{keyUp}, 0},
		{"release of down does not move", defaultSize, []stroke{{name: key.NameDownArrow, up: true}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, longDoc(t), tt.size, io.Discard)
			e.frame()
			e.requireLong()
			e.press(tt.keys...)
			if got := e.absY(); got != tt.want {
				t.Errorf("absolute position is %d, want %d", got, tt.want)
			}
		})
	}
}

// requireLong fails the test unless the document is long enough for the scroll tests to mean
// something: more than three view heights of 600 px at zoom 100. It measures the blocks with a
// reference View, not the viewer's list, so the check does not depend on the code under test.
func (e *viewerEnv) requireLong() {
	e.t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      e.metric,
		Constraints: layout.Constraints{Max: image.Pt(e.size.X-10, e.size.Y)},
	}
	ref := render.NewView(e.doc, shaper(e.t), render.Light, e.images)
	total := 0
	for i := range ref.Len() {
		total += ref.Layout(gtx, i, nil).Size.Y
	}
	if total <= 3*600 {
		e.t.Fatalf("document is %d px tall, want more than %d", total, 3*600)
	}
}

// TestViewerHomeEnd checks the two limits: Home is the top, and End puts the last block at the
// bottom. Down at the bottom does not move past it.
func TestViewerHomeEnd(t *testing.T) {
	e := newEnv(t, longDoc(t), defaultSize, io.Discard)
	e.frame()
	e.requireLong()

	e.press(keyEnd)
	pos := e.vw.list.Position
	if pos.First+pos.Count != 60 || pos.OffsetLast != 0 {
		t.Errorf("after End: First %d Count %d OffsetLast %d, want the last block at the bottom",
			pos.First, pos.Count, pos.OffsetLast)
	}
	bottom := e.absY()
	e.press(keyDown)
	if got := e.absY(); got != bottom {
		t.Errorf("down at the bottom moved from %d to %d", bottom, got)
	}

	e.press(keyHome)
	pos = e.vw.list.Position
	if pos.First != 0 || pos.Offset != 0 {
		t.Errorf("after Home: First %d Offset %d, want 0 and 0", pos.First, pos.Offset)
	}
}

// TestViewerEveryKeyRoutes checks that each key of the action table reaches the viewer through
// the router, and has its effect. A key whose effect needs a prior state presses its setup first.
func TestViewerEveryKeyRoutes(t *testing.T) {
	type effect uint8
	const (
		scrolls effect = iota
		zooms
		quits
	)
	tests := []struct {
		name   string
		setup  []stroke
		press  stroke
		effect effect
	}{
		{"down", nil, keyDown, scrolls},
		{"up", []stroke{keyPageDown}, keyUp, scrolls},
		{"page down", nil, keyPageDown, scrolls},
		{"page up", []stroke{keyPageDown}, keyPageUp, scrolls},
		{"space", nil, keySpace, scrolls},
		{"shift space", []stroke{keyPageDown}, stroke{name: key.NameSpace, mods: key.ModShift}, scrolls},
		{"home", []stroke{keyPageDown}, keyHome, scrolls},
		{"end", nil, keyEnd, scrolls},
		{"ctrl equals", nil, keyZoomIn, zooms},
		{"ctrl plus", nil, stroke{name: "+", mods: key.ModCtrl}, zooms},
		{"ctrl minus", []stroke{keyZoomIn}, keyZoomOut, zooms},
		{"ctrl zero", []stroke{keyZoomIn}, keyZoom0, zooms},
		{"q", nil, stroke{name: "Q"}, quits},
		{"shift q", nil, stroke{name: "Q", mods: key.ModShift}, quits},
		{"escape", nil, stroke{name: key.NameEscape}, quits},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, longDoc(t), defaultSize, io.Discard)
			e.frame()
			e.requireLong()
			e.press(tt.setup...)
			before := e.absY()
			zoomBefore := e.vw.zoom.Percent()
			quit := e.press(tt.press)
			switch tt.effect {
			case scrolls:
				if e.absY() == before {
					t.Errorf("key %q did not move the view from %d", tt.press.name, before)
				}
			case zooms:
				if e.vw.zoom.Percent() == zoomBefore {
					t.Errorf("key %q did not change the zoom from %d%%", tt.press.name, zoomBefore)
				}
			case quits:
				if !quit {
					t.Errorf("key %q did not quit", tt.press.name)
				}
			}
		})
	}
}

// TestViewerZoomKeys checks ZOOM-2 with the keys: each Ctrl key moves the zoom through ZOOM-1's
// levels, and the limits hold at 300 and at 50.
func TestViewerZoomKeys(t *testing.T) {
	e := newEnv(t, longDoc(t), defaultSize, io.Discard)
	e.frame()
	steps := []struct {
		name string
		key  stroke
		want int
	}{
		{"ctrl equals", keyZoomIn, 110},
		{"ctrl shift plus", stroke{name: "+", mods: key.ModCtrl | key.ModShift}, 125},
		{"ctrl minus", keyZoomOut, 110},
		{"ctrl zero", keyZoom0, 100},
	}
	for _, s := range steps {
		e.press(s.key)
		if got := e.vw.zoom.Percent(); got != s.want {
			t.Errorf("after %s the zoom is %d%%, want %d%%", s.name, got, s.want)
		}
	}
	for range 13 {
		e.press(keyZoomIn)
	}
	if got := e.vw.zoom.Percent(); got != 300 {
		t.Errorf("after many ctrl+= the zoom is %d%%, want 300%%", got)
	}
	for range 15 {
		e.press(keyZoomOut)
	}
	if got := e.vw.zoom.Percent(); got != 50 {
		t.Errorf("after many ctrl+- the zoom is %d%%, want 50%%", got)
	}
}

// TestViewerZoomScalesContent checks DOC-LAYOUT-1 and STYLE-ZOOM-3 through the frame height of a
// rule-only document: the padding is 45 dp from a 768 dp list and 15 dp below it, and the
// padding and the rule are zoomed. Window widths are in px at zoom 100, the list is the window
// less the 10 px gutter.
func TestViewerZoomScalesContent(t *testing.T) {
	tests := []struct {
		name  string
		width int
		zoom  int // 100 or 200
		want  int
	}{
		{"1000 px at zoom 100 gets 45 dp", 1000, 100, 94},
		{"778 px at zoom 100 gets 45 dp (768 dp after the gutter)", 778, 100, 94},
		{"777 px at zoom 100 gets 15 dp", 777, 100, 34},
		{"1600 px at zoom 200 gets 45 dp", 1600, 200, 188},
		{"1000 px at zoom 200 gets 15 dp", 1000, 200, 68},
		{"1546 px at zoom 200 gets 45 dp (the gutter is not zoomed)", 1546, 200, 188},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, parse(t, "---\n", t.TempDir()), image.Pt(tt.width, 600), io.Discard)
			e.frame() // the window has laid out a frame before any key arrives
			if tt.zoom == 200 {
				// Five Ctrl+= presses: 110, 125, 150, 175, 200 (ZOOM-1).
				e.press(keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn)
			}
			e.frame()
			if got := e.vw.list.Position.Length; got != tt.want {
				t.Errorf("frame height is %d px, want %d px", got, tt.want)
			}
		})
	}
}

// TestViewerWheel checks SCROLL-3 and ZOOM-4's boundary: a vertical wheel scroll without Ctrl
// moves the view by its pixels, including over the scrollbar, and Ctrl and horizontal scrolls
// do not scroll.
func TestViewerWheel(t *testing.T) {
	scrollbar := f32.Pt(795, 300)
	tests := []struct {
		name  string
		steps func(e *viewerEnv)
		want  int
	}{
		{"wheel 100 moves 100 px", func(e *viewerEnv) {
			e.wheel(100, 0)
		}, 100},
		{"wheel 100 then -30 ends at 70", func(e *viewerEnv) {
			e.wheel(100, 0)
			e.frame()
			e.wheel(-30, 0)
		}, 70},
		{"one X11 notch of 10 px moves 10", func(e *viewerEnv) {
			e.wheel(10, 0)
		}, 10},
		{"wheel over the scrollbar scrolls", func(e *viewerEnv) {
			e.scroll(scrollbar, 0, 100, 0)
		}, 100},
		{"ctrl wheel does not scroll", func(e *viewerEnv) {
			e.wheel(100, key.ModCtrl)
		}, 0},
		{"ctrl shift wheel does not scroll", func(e *viewerEnv) {
			e.wheel(100, key.ModCtrl|key.ModShift)
		}, 0},
		{"horizontal scroll does not move", func(e *viewerEnv) {
			e.scroll(e.centre(), 50, 0, 0)
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, longDoc(t), defaultSize, io.Discard)
			e.frame()
			e.requireLong()
			tt.steps(e)
			e.frame()
			if got := e.absY(); got != tt.want {
				t.Errorf("absolute position is %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("wheel past the bottom clamps", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		e.requireLong()
		e.wheel(5000, 0)
		e.frame()
		if e.absY() == 0 {
			t.Error("a wheel scroll of 5000 px did not move the view")
		}
		if got := e.vw.list.Position.OffsetLast; got != 0 {
			t.Errorf("OffsetLast is %d after a large scroll, want 0 (the last block at the bottom)", got)
		}
	})
	t.Run("horizontal ctrl scroll does not zoom", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		before := e.vw.zoom.Percent()
		e.scroll(e.centre(), 50, 0, key.ModCtrl)
		e.frame()
		if got := e.vw.zoom.Percent(); got != before {
			t.Errorf("zoom went from %d%% to %d%% after a horizontal ctrl scroll", before, got)
		}
	})
}

// TestViewerCtrlWheelZoom checks ZOOM-4: Ctrl+wheel accumulates, zooms one level at a time when
// the sum reaches -10 or +10 px, resets the sum, and never scrolls.
func TestViewerCtrlWheelZoom(t *testing.T) {
	// to300 is seven Ctrl+= presses from 100: 110, 125, 150, 175, 200, 250, 300.
	to300 := []stroke{keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn}
	tests := []struct {
		name   string
		setup  []stroke
		events []float32
		want   int
	}{
		{"-100 zooms in one level", nil, []float32{-100}, 110},
		{"-4 twice does nothing, a third zooms in", nil, []float32{-4, -4}, 100},
		{"-4 three times zooms in", nil, []float32{-4, -4, -4}, 110},
		{"+100 zooms out from 110", nil, []float32{-100, 100}, 100},
		{"exactly -10 zooms in", nil, []float32{-10}, 110},
		{"-9 alone does nothing", nil, []float32{-9}, 100},
		{"-9 then -1 zooms in", nil, []float32{-9, -1}, 110},
		{"-6, +6, -6 gives no change", nil, []float32{-6, 6, -6}, 100},
		{"+10 zooms out", nil, []float32{10}, 90},
		{"-100 at 300 stays at 300", to300, []float32{-100}, 300},
		{"at 300, two -4 do not zoom in", to300, []float32{-100, -4, -4}, 300},
		{"at 300, two +4 do not zoom out", to300, []float32{-100, 4, 4}, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, longDoc(t), defaultSize, io.Discard)
			e.frame()
			e.press(tt.setup...)
			for _, y := range tt.events {
				e.wheel(y, key.ModCtrl)
				e.frame()
			}
			if got := e.vw.zoom.Percent(); got != tt.want {
				t.Errorf("zoom is %d%%, want %d%%", got, tt.want)
			}
		})
	}
}

// TestViewerCtrlWheelKeepsFirst checks that Ctrl+wheel zoom keeps the block at the top: First
// does not change, and the offset is scaled (ZOOM-5).
func TestViewerCtrlWheelKeepsFirst(t *testing.T) {
	e := newEnv(t, longDoc(t), defaultSize, io.Discard)
	e.frame()
	e.requireLong()
	e.wheel(300, 0)
	e.frame()
	first := e.vw.list.Position.First
	e.wheel(-100, key.ModCtrl)
	e.frame()
	if got := e.vw.zoom.Percent(); got != 110 {
		t.Errorf("zoom is %d%% after Ctrl+wheel -100, want 110%%", got)
	}
	if got := e.vw.list.Position.First; got != first {
		t.Errorf("First changed from %d to %d on a Ctrl+wheel zoom", first, got)
	}
}

// TestViewerZoomKeepsPosition checks ZOOM-5: a zoom change keeps First and scales Offset by the
// ratio of the zooms, rounded to the nearest pixel.
func TestViewerZoomKeepsPosition(t *testing.T) {
	setPosition := func(e *viewerEnv, first, offset int) {
		e.vw.list.Position = layout.Position{First: first, Offset: offset}
		e.frame()
	}
	t.Run("ctrl equals scales 20 to 22", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		setPosition(e, 10, 20)
		e.press(keyZoomIn)
		assertPosition(t, e, 10, 22)
	})
	t.Run("ctrl equals then ctrl minus returns 22 to 20", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		setPosition(e, 10, 20)
		e.press(keyZoomIn, keyZoomOut)
		assertPosition(t, e, 10, 20)
	})
	t.Run("ctrl zero from 200 scales 30 to 15", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		e.press(keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn, keyZoomIn)
		setPosition(e, 10, 30)
		e.press(keyZoom0)
		assertPosition(t, e, 10, 15)
	})
	t.Run("ctrl wheel -100 scales 20 to 22", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		setPosition(e, 10, 20)
		e.wheel(-100, key.ModCtrl)
		e.frame()
		assertPosition(t, e, 10, 22)
	})
}

// assertPosition fails the test unless the list's First and Offset are first and offset.
func assertPosition(t *testing.T, e *viewerEnv, first, offset int) {
	t.Helper()
	pos := e.vw.list.Position
	if pos.First != first || pos.Offset != offset {
		t.Errorf("position is First %d Offset %d, want First %d Offset %d",
			pos.First, pos.Offset, first, offset)
	}
}

// TestViewerScrollbarDrag checks SCROLL-4: a press on the indicator and a drag scroll the list.
// The first drag event only anchors, so the test sends two moves. A press and a release without
// a move do not scroll.
func TestViewerScrollbarDrag(t *testing.T) {
	indicator := f32.Pt(795, 10)
	t.Run("drag scrolls", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		e.requireLong()
		e.router.Queue(
			pointer.Event{Kind: pointer.Move, Position: indicator, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Press, Position: indicator, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		)
		e.frame()
		e.router.Queue(
			pointer.Event{Kind: pointer.Move, Position: f32.Pt(795, 60), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Move, Position: f32.Pt(795, 110), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		)
		e.frame()
		e.router.Queue(pointer.Event{Kind: pointer.Release, Position: f32.Pt(795, 110), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse})
		e.frame()
		e.frame()
		if got := e.absY(); got <= 0 {
			t.Errorf("absolute position is %d after the drag, want more than 0", got)
		}
	})
	t.Run("press and release without a move do not scroll", func(t *testing.T) {
		e := newEnv(t, longDoc(t), defaultSize, io.Discard)
		e.frame()
		e.router.Queue(
			pointer.Event{Kind: pointer.Move, Position: indicator, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Press, Position: indicator, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
			pointer.Event{Kind: pointer.Release, Position: indicator, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		)
		e.frame()
		e.frame()
		if got := e.absY(); got != 0 {
			t.Errorf("absolute position is %d, want 0", got)
		}
	})
}

// TestViewerLinks checks LINK-1 and LINK-2 for each kind of link: the opener's calls and the
// stderr line, byte for byte. Each row clicks block 0's link once.
func TestViewerLinks(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		file     string // created in the document's directory, when set
		testdata bool   // the document is in testdata, not a temporary directory
		calls    func(dir string) []string
		stderr   func(dir string) string
	}{
		{
			name:  "external link",
			src:   "[e](https://e.org/x)",
			calls: func(string) []string { return []string{"https://e.org/x"} },
		},
		{
			name:  "mailto link",
			src:   "[m](mailto:a@b.c)",
			calls: func(string) []string { return []string{"mailto:a@b.c"} },
		},
		{
			name:  "existing local file",
			src:   "[f](f.txt)",
			file:  "f.txt",
			calls: func(dir string) []string { return []string{filepath.Join(dir, "f.txt")} },
		},
		{
			name:  "existing file with a fragment",
			src:   "[o](other.md#part)",
			file:  "other.md",
			calls: func(dir string) []string { return []string{filepath.Join(dir, "other.md")} },
		},
		{
			name:  "empty destination is the document's directory",
			src:   "[d]()",
			calls: func(dir string) []string { return []string{dir} },
		},
		{
			name: "missing file",
			src:  "[n](nope.md)",
			stderr: func(dir string) string {
				return "md-viewer: link target not found: " + filepath.Join(dir, "nope.md") + "\n"
			},
		},
		{
			name:   "fragment",
			src:    "[t](#top)",
			stderr: func(string) string { return "md-viewer: in-page links are not followed: #top\n" },
		},
		{
			name: "refused scheme",
			src:  "[j](javascript:alert(1))",
			stderr: func(string) string {
				return "md-viewer: link not opened: javascript:alert(1): only http, https, mailto and local files are allowed\n"
			},
		},
		{
			name:     "image that is a link (STYLE-IMG-4)",
			src:      "[![a](img/red-4x3.png)](https://e.org/)",
			testdata: true,
			calls:    func(string) []string { return []string{"https://e.org/"} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.testdata {
				dir = testdataDir(t)
			}
			if tt.file != "" {
				if err := os.WriteFile(filepath.Join(dir, tt.file), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			d := parse(t, tt.src+"\n", dir)
			var stderr bytes.Buffer
			e := newEnv(t, d, defaultSize, &stderr)
			e.frame()
			e.click(linkCentre(t, d, e.images, defaultSize.X-10))
			quit, err := e.step()
			if quit || err != nil {
				t.Fatalf("click frame returned quit %v, err %v; want false, nil", quit, err)
			}
			e.runSpawned()

			var wantCalls []string
			if tt.calls != nil {
				wantCalls = tt.calls(dir)
			}
			if !slices.Equal(e.opener.calls, wantCalls) {
				t.Errorf("opener was called with %q, want %q", e.opener.calls, wantCalls)
			}
			wantStderr := ""
			if tt.stderr != nil {
				wantStderr = tt.stderr(dir)
			}
			if got := stderr.String(); got != wantStderr {
				t.Errorf("stderr is %q, want %q", got, wantStderr)
			}
		})
	}
}

// TestViewerLinkControls checks that a link is not opened without a click: a hover over it, a
// frame with no input, and a press on the link released away from it (LINK-1: press and release
// on the same link). The press-and-release-away sequence settles the layout first, and sends the
// pointer's moves a frame apart, as the window does; Gio then cancels the gesture.
func TestViewerLinkControls(t *testing.T) {
	d := linkDoc(t, "[e](https://e.org/x)")
	p := linkCentre(t, d, imagesOf(t, d), defaultSize.X-10)
	away := f32.Pt(p.X+200, p.Y)
	t.Run("press on the link and release away", func(t *testing.T) {
		var stderr bytes.Buffer
		e := newEnv(t, d, defaultSize, &stderr)
		e.frame()
		e.frame()
		e.router.Queue(pointer.Event{Kind: pointer.Move, Position: p, Source: pointer.Mouse})
		e.frame()
		e.router.Queue(pointer.Event{Kind: pointer.Press, Position: p, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse})
		e.frame()
		e.router.Queue(pointer.Event{Kind: pointer.Move, Position: away, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse})
		e.frame()
		e.router.Queue(pointer.Event{Kind: pointer.Release, Position: away, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse})
		e.frame()
		e.runSpawned()
		if len(e.opener.calls) != 0 || stderr.Len() != 0 {
			t.Errorf("opener calls %q and stderr %q, want none", e.opener.calls, stderr.String())
		}
	})
	t.Run("hover over the link", func(t *testing.T) {
		var stderr bytes.Buffer
		e := newEnv(t, d, defaultSize, &stderr)
		e.frame()
		e.router.Queue(pointer.Event{Kind: pointer.Move, Position: p, Source: pointer.Mouse})
		e.frame()
		e.frame()
		e.runSpawned()
		if len(e.opener.calls) != 0 || stderr.Len() != 0 {
			t.Errorf("opener calls %q and stderr %q, want none", e.opener.calls, stderr.String())
		}
	})
	t.Run("no input", func(t *testing.T) {
		var stderr bytes.Buffer
		e := newEnv(t, d, defaultSize, &stderr)
		e.frame()
		e.frame()
		e.runSpawned()
		if len(e.opener.calls) != 0 || stderr.Len() != 0 {
			t.Errorf("opener calls %q and stderr %q, want none", e.opener.calls, stderr.String())
		}
	})
}

// TestViewerLinkCursor checks STYLE-TEXT-3: the hand cursor over a link, even under the wheel
// area, and the default cursor over plain text.
func TestViewerLinkCursor(t *testing.T) {
	d := linkDoc(t, "[e](https://e.org/x) plain")
	images := imagesOf(t, d)
	p := linkCentre(t, d, images, defaultSize.X-10)
	e := newEnv(t, d, defaultSize, io.Discard)
	e.frame()
	e.router.Queue(pointer.Event{Kind: pointer.Move, Position: p, Source: pointer.Mouse})
	e.frame()
	if got := e.router.Cursor(); got != pointer.CursorPointer {
		t.Errorf("cursor over the link is %v, want the hand", got)
	}
	e.router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(p.X+200, p.Y), Source: pointer.Mouse})
	e.frame()
	if got := e.router.Cursor(); got != pointer.CursorDefault {
		t.Errorf("cursor over plain text is %v, want the default", got)
	}
}

// TestViewerOpensOffFrame checks LINK-2: the frame that sees the click starts nothing; the
// opener runs only in the function the viewer spawned.
func TestViewerOpensOffFrame(t *testing.T) {
	d := linkDoc(t, "[e](https://e.org/x)")
	e := newEnv(t, d, defaultSize, io.Discard)
	e.frame()
	e.click(linkCentre(t, d, e.images, defaultSize.X-10))
	e.frame()
	if len(e.opener.calls) != 0 {
		t.Errorf("opener was called %d times in the frame, want 0", len(e.opener.calls))
	}
	if len(e.spawned) != 1 {
		t.Errorf("%d functions spawned, want 1", len(e.spawned))
	}
	e.runSpawned()
	if len(e.opener.calls) != 1 {
		t.Errorf("opener was called %d times after the spawned function, want 1", len(e.opener.calls))
	}
}

// TestOpenerFailureReported checks LINK-2: an opener that fails prints its line, and the window
// goes on: the next frame neither quits nor fails. A successful opener prints nothing.
func TestOpenerFailureReported(t *testing.T) {
	d := linkDoc(t, "[e](https://e.org/x)")
	t.Run("failure prints the line", func(t *testing.T) {
		var stderr bytes.Buffer
		e := newEnv(t, d, defaultSize, &stderr)
		e.opener.err = errors.New("exit status 3")
		e.frame()
		e.click(linkCentre(t, d, e.images, defaultSize.X-10))
		e.frame()
		e.runSpawned()
		if got, want := stderr.String(), "md-viewer: cannot open link: https://e.org/x: exit status 3\n"; got != want {
			t.Errorf("stderr is %q, want %q", got, want)
		}
		quit, err := e.step()
		if quit || err != nil {
			t.Errorf("frame after the failure returned quit %v, err %v; want false, nil", quit, err)
		}
	})
	t.Run("success prints nothing", func(t *testing.T) {
		var stderr bytes.Buffer
		e := newEnv(t, d, defaultSize, &stderr)
		e.frame()
		e.click(linkCentre(t, d, e.images, defaultSize.X-10))
		e.frame()
		e.runSpawned()
		if stderr.Len() != 0 {
			t.Errorf("stderr is %q, want nothing", stderr.String())
		}
	})
}

// TestViewerQuit checks WIN-4: q and Esc quit, with the modifiers the spec allows, and nothing
// else does.
func TestViewerQuit(t *testing.T) {
	tests := []struct {
		name  string
		keys  []stroke
		quits bool
	}{
		{"q", []stroke{{name: "Q"}}, true},
		{"shift q", []stroke{{name: "Q", mods: key.ModShift}}, true},
		{"escape", []stroke{{name: key.NameEscape}}, true},
		{"ctrl q", []stroke{{name: "Q", mods: key.ModCtrl}}, false},
		{"alt q", []stroke{{name: "Q", mods: key.ModAlt}}, false},
		{"super q", []stroke{{name: "Q", mods: key.ModSuper}}, false},
		{"release of q", []stroke{{name: "Q", up: true}}, false},
		{"no key", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, longDoc(t), defaultSize, io.Discard)
			e.frame()
			quit := e.press(tt.keys...)
			if len(tt.keys) == 0 {
				quit = e.frame()
			}
			if quit != tt.quits {
				t.Errorf("Frame returned quit %v, want %v", quit, tt.quits)
			}
		})
	}
}

// TestViewerNoEditing pins WIN-5 (control only: nothing here can fail on a viewer that does not
// edit). Ctrl+C, Ctrl+A, Ctrl+V and text input change nothing: no clipboard request, no text
// input state, no scroll and no zoom.
func TestViewerNoEditing(t *testing.T) {
	e := newEnv(t, longDoc(t), defaultSize, io.Discard)
	e.frame()
	before, zoomBefore := e.absY(), e.vw.zoom.Percent()
	e.router.Queue(
		key.Event{Name: "C", Modifiers: key.ModCtrl, State: key.Press},
		key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press},
		key.Event{Name: "V", Modifiers: key.ModCtrl, State: key.Press},
		key.EditEvent{Text: "x"},
	)
	e.frame()
	if e.router.ClipboardRequested() {
		t.Error("the viewer requested the clipboard")
	}
	if _, _, ok := e.router.WriteClipboard(); ok {
		t.Error("the viewer wrote to the clipboard")
	}
	if got := e.router.TextInputState(); got != input.TextInputKeep {
		t.Errorf("text input state is %v, want Keep", got)
	}
	if got := e.absY(); got != before {
		t.Errorf("absolute position moved from %d to %d", before, got)
	}
	if got := e.vw.zoom.Percent(); got != zoomBefore {
		t.Errorf("zoom changed from %d%% to %d%%", zoomBefore, got)
	}
}

// TestViewerEmptyDocument checks that a document with no blocks takes every input without an
// error or a panic.
func TestViewerEmptyDocument(t *testing.T) {
	e := newEnv(t, parse(t, "", t.TempDir()), defaultSize, io.Discard)
	e.frame()
	e.press(keyEnd, keyHome, keyDown, keyZoomIn)
	e.wheel(100, 0)
	e.frame()
}

// TestViewerElements lays out every element of testdata/elements.md with input: Home, End and a
// wheel scroll, at 980 × 900. The document has images, whose decoding is not waited for here.
func TestViewerElements(t *testing.T) {
	dir := testdataDir(t)
	src, err := os.ReadFile(filepath.Join(dir, "elements.md"))
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, parse(t, string(src), dir), image.Pt(980, 900), io.Discard)
	e.frame()
	e.press(keyEnd, keyHome)
	e.wheel(50, 0)
	e.frame()
	if got := e.vw.list.Position.Length; got <= 0 {
		t.Errorf("list length is %d, want more than 0", got)
	}
}

// TestViewerRewraps checks WRAP-2: the same paragraph is taller when the window is narrower.
func TestViewerRewraps(t *testing.T) {
	words := strings.TrimSpace(strings.Repeat("word ", 60))
	e := newEnv(t, parse(t, words+"\n", t.TempDir()), defaultSize, io.Discard)
	e.frame()
	wide := e.vw.list.Position.Length
	e.size = image.Pt(300, 600)
	e.frame()
	narrow := e.vw.list.Position.Length
	if wide <= 0 || narrow <= wide {
		t.Errorf("paragraph is %d px at 800 px and %d px at 300 px, want 0 < 800 px height < 300 px height",
			wide, narrow)
	}
}

// failingLayoutDoc is a document View cannot lay out: a heading of level 9 (STYLE-TEXT-2).
func failingLayoutDoc() *doc.Document {
	return &doc.Document{Blocks: []doc.Block{doc.Heading{Level: 9}}}
}

// TestViewerLayoutError checks that a frame fails with the layout error, and with a failed write
// to stderr, and that a document that lays out gives no error.
func TestViewerLayoutError(t *testing.T) {
	t.Run("layout error ends the frame", func(t *testing.T) {
		e := newEnv(t, failingLayoutDoc(), defaultSize, io.Discard)
		_, err := e.step()
		if err == nil {
			t.Fatal("Frame returned no error for a heading of level 9")
		}
		if !errors.Is(err, e.view.Err()) {
			t.Errorf("Frame error %v is not the view's error %v", err, e.view.Err())
		}
	})
	t.Run("failed stderr write ends the frame", func(t *testing.T) {
		d := linkDoc(t, "[t](#top)")
		e := newEnv(t, d, defaultSize, failWriter{})
		e.frame()
		e.click(linkCentre(t, d, e.images, defaultSize.X-10))
		if _, err := e.step(); err == nil {
			t.Error("Frame returned no error after a failed stderr write")
		}
	})
	t.Run("ordinary document gives no error", func(t *testing.T) {
		dir := testdataDir(t)
		src, err := os.ReadFile(filepath.Join(dir, "elements.md"))
		if err != nil {
			t.Fatal(err)
		}
		e := newEnv(t, parse(t, string(src), dir), image.Pt(980, 900), io.Discard)
		if _, err := e.step(); err != nil {
			t.Errorf("Frame returned %v for elements.md", err)
		}
	})
}
