package render

import (
	"image"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// The viewer's list at zoom 100 (SPEC WIN-2, SCROLL-4): the 980 × 900 dp window less the 10 dp
// scrollbar gutter gives a list 970 dp wide and 900 dp tall. One dp is one pixel here.
const (
	listWidth  = 970
	listHeight = 900
)

// benchTestdata is the shared fixtures directory, relative to this package's directory.
const benchTestdata = "../../testdata"

// loadLarge parses large.md, as the viewer does, and probes its images once. That is the
// preparation SPEED-1 times before the window opens, so the benchmark keeps it out of its loop.
func loadLarge(tb testing.TB) (*doc.Document, *Images, Faces) {
	tb.Helper()
	src, err := os.ReadFile(filepath.Join(benchTestdata, "large.md"))
	if err != nil {
		tb.Fatalf("read large.md: %v", err)
	}
	dir, err := filepath.Abs(benchTestdata)
	if err != nil {
		tb.Fatalf("Abs of the testdata directory: %v", err)
	}
	d, err := doc.Parse(src, dir)
	if err != nil {
		tb.Fatalf("Parse(large.md): %v", err)
	}
	img, err := NewImages(d, io.Discard)
	if err != nil {
		tb.Fatalf("NewImages(large.md): %v", err)
	}
	faces, err := LoadFaces()
	if err != nil {
		tb.Fatalf("LoadFaces: %v", err)
	}
	return d, img, faces
}

// viewportContext is the layout context of the viewer's list: the list's constraints at one pixel
// per dp.
func viewportContext(ops *op.Ops, r *input.Router) layout.Context {
	return layout.Context{
		Ops:         ops,
		Source:      r.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(listWidth, listHeight)),
	}
}

// firstViewport lays out v's blocks in a vertical layout.List under gtx, as the viewer's list
// does (SCROLL-1). It returns how many blocks were laid out and the sum of their heights.
func firstViewport(v *View, gtx layout.Context) (calls, height int) {
	var list layout.List
	list.Axis = layout.Vertical
	list.Layout(gtx, v.Len(), func(gtx layout.Context, i int) layout.Dimensions {
		calls++
		dims := v.Layout(gtx, i, nil)
		height += dims.Size.Y
		return dims
	})
	return calls, height
}

// TestFirstViewportLaysOutFewBlocks: the blocks that fill the viewport of large.md are laid out,
// and only those. The layout asks for fewer than 60 blocks, and the blocks it lays out cover the
// list's 900 px height (SCROLL-1). The benchmark uses the same helper, so this pins its scope too.
func TestFirstViewportLaysOutFewBlocks(t *testing.T) {
	d, img, faces := loadLarge(t)
	v := NewView(d, NewShaper(faces), Light, img)
	var ops op.Ops
	var r input.Router
	calls, height := firstViewport(v, viewportContext(&ops, &r))
	if err := v.Err(); err != nil {
		t.Fatalf("layout of large.md failed: %v", err)
	}
	if calls <= 0 || calls >= 60 {
		t.Errorf("first viewport laid out %d blocks, want more than 0 and fewer than 60", calls)
	}
	if height < listHeight {
		t.Errorf("the blocks laid out are %d px tall, want at least %d to cover the viewport", height, listHeight)
	}
}

// BenchmarkFirstViewportLarge measures the first frame's layout of large.md. Each iteration makes
// a fresh shaper, as the first frame has (a shared one would keep its layout cache), and a fresh
// View, then lays out the blocks that fill the list. The parse, the fonts and the image probes
// are set up outside the loop. Its numbers are reported, never asserted.
func BenchmarkFirstViewportLarge(b *testing.B) {
	d, img, faces := loadLarge(b)
	b.ReportAllocs()
	var calls, height int
	for b.Loop() {
		var ops op.Ops
		var r input.Router
		v := NewView(d, NewShaper(faces), Light, img)
		calls, height = firstViewport(v, viewportContext(&ops, &r))
		if err := v.Err(); err != nil {
			b.Fatalf("layout of large.md failed: %v", err)
		}
	}
	b.ReportMetric(float64(calls), "blocks/op")
	b.ReportMetric(float64(height), "px/op")
}
