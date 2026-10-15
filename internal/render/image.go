package render

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder (STYLE-IMG-1)
	_ "image/png"  // registers the PNG decoder (STYLE-IMG-1)
	"io"
	"math"
	"os"

	"gioui.org/layout"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// Formats that are drawn (STYLE-IMG-1), as image.DecodeConfig and image.Decode name them.
const (
	formatPNG  = "png"  // STYLE-IMG-1
	formatJPEG = "jpeg" // STYLE-IMG-1
)

// The size cap (STYLE-IMG-1). A texture larger than the GPU accepts makes Gio panic, and a huge
// header would allocate gigabytes, so the header is judged before any pixel is decoded.
const (
	maxImageSide   = 8192       // STYLE-IMG-1: pixels along each side
	maxImagePixels = 40_000_000 // STYLE-IMG-1: pixels in all
)

// REASONs of a placeholder, printed after the image's DEST (STYLE-IMG-3).
const (
	reasonRemote    = "remote images are not loaded" // STYLE-IMG-3
	reasonFormat    = "unsupported format"           // STYLE-IMG-3
	reasonNotLocal  = "not a local file"             // STYLE-IMG-3
	reasonDirectory = "is a directory"               // STYLE-IMG-3
	reasonNotFile   = "not a regular file"           // STYLE-IMG-3
	// reasonTooLargeFmt is the REASON of an image past the size cap, with its size (STYLE-IMG-1).
	reasonTooLargeFmt = "image too large: %dx%d" // STYLE-IMG-1, STYLE-IMG-3
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

// probeImage reads the size of the image at path from its header (SPEED-1). A file that cannot be
// drawn gives the REASON its placeholder shows (STYLE-IMG-3), and a size of zero. A directory and a
// non-regular file are judged by Stat and never opened, so a FIFO cannot block the probe (CLI-8).
func probeImage(path string) (image.Point, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return image.Point{}, pathReason(err)
	}
	switch {
	case fi.IsDir():
		return image.Point{}, reasonDirectory
	case !fi.Mode().IsRegular():
		return image.Point{}, reasonNotFile
	}
	f, err := os.Open(path)
	if err != nil {
		return image.Point{}, pathReason(err)
	}
	cfg, format, derr := image.DecodeConfig(f)
	cerr := f.Close()
	switch {
	case errors.Is(derr, image.ErrFormat):
		return image.Point{}, reasonFormat
	case derr != nil:
		return image.Point{}, derr.Error()
	case cerr != nil:
		return image.Point{}, cerr.Error()
	case !supportedFormat(format):
		return image.Point{}, reasonFormat
	case tooLarge(cfg):
		return image.Point{}, fmt.Sprintf(reasonTooLargeFmt, cfg.Width, cfg.Height)
	}
	return image.Pt(cfg.Width, cfg.Height), ""
}

// decodeFile decodes the pixels of the image at path. It runs in the decode worker, never on the
// frame path (SPEED-5). Its error is the decoder's, or the operating system's for a file that
// cannot be opened, and a format other than PNG or JPEG is an error too (STYLE-IMG-1).
func decodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New(pathReason(err))
	}
	img, format, derr := image.Decode(f)
	cerr := f.Close()
	switch {
	case derr != nil:
		return nil, derr
	case cerr != nil:
		return nil, cerr
	case !supportedFormat(format):
		return nil, errors.New(reasonFormat)
	case img.Bounds().Empty():
		return nil, fmt.Errorf("render: %s decodes to no pixels", path)
	}
	return img, nil
}

// supportedFormat is whether a decoded format is one md-viewer draws (STYLE-IMG-1).
func supportedFormat(format string) bool {
	return format == formatPNG || format == formatJPEG
}

// tooLarge is whether an image of this header is past the size cap (STYLE-IMG-1): it is checked
// before any pixels are decoded, so that a huge header allocates nothing.
func tooLarge(cfg image.Config) bool {
	return cfg.Width > maxImageSide || cfg.Height > maxImageSide ||
		int64(cfg.Width)*int64(cfg.Height) > maxImagePixels
}

// pathReason is the text of the operating system's error for a path, without the path, as CLI-8
// words it (STYLE-IMG-3).
func pathReason(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
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
