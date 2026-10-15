package render

import (
	"image"
	"io"
	"math"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// Images is the document's images. Stub: step 1 of M8, to be replaced.
type Images struct{}

// NewImages is a stub.
func NewImages(d *doc.Document, warn io.Writer) (*Images, error) {
	return newImages(d, warn, decodeFile, goSpawn)
}

// newImages is a stub.
func newImages(d *doc.Document, warn io.Writer, decode func(string) (image.Image, error), spawn func(func())) (*Images, error) {
	return &Images{}, nil
}

// Bind is a stub.
func (im *Images) Bind(invalidate func()) {}

// probeImage is a stub.
func probeImage(path string) (image.Point, string) {
	return image.Point{}, ""
}

// decodeFile is a stub.
func decodeFile(path string) (image.Image, error) {
	return nil, nil
}

// imageBox is the box an image of size (one pixel per dp at natural size) takes in the width gtx
// gives it: its natural size, or scaled down to that width keeping its aspect ratio (STYLE-IMG-2).
// Each side is rounded to whole pixels once, so a box may be 0 px tall.
func imageBox(gtx layout.Context, size image.Point) image.Point {
	w := max(gtx.Constraints.Max.X, 0)
	fw := float64(size.X) * float64(gtx.Metric.PxPerDp)
	fh := float64(size.Y) * float64(gtx.Metric.PxPerDp)
	if fw > float64(w) {
		return image.Pt(w, int(math.Round(fh*float64(w)/fw)))
	}
	return image.Pt(int(math.Round(fw)), int(math.Round(fh)))
}

// layoutImage is a stub: height 0 at the width given.
func layoutImage(gtx layout.Context, ctx blockCtx, img doc.Image, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}, nil
}

// goSpawn starts f in a goroutine.
func goSpawn(f func()) {
	go f()
}
