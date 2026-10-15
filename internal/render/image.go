package render

import (
	"image"
	"io"

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

// imageBox is a stub.
func imageBox(gtx layout.Context, size image.Point) image.Point {
	return image.Point{}
}

// layoutImage is a stub: height 0 at the width given.
func layoutImage(gtx layout.Context, ctx blockCtx, img doc.Image, tr *Trace) (layout.Dimensions, error) {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}, nil
}

// goSpawn starts f in a goroutine.
func goSpawn(f func()) {
	go f()
}
