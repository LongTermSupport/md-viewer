package render

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder (STYLE-IMG-1)
	_ "image/png"  // registers the PNG decoder (STYLE-IMG-1)
	"io"
	"math"
	"net/url"
	"os"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// The placeholder's spacing (STYLE-IMG-3): 8 dp of padding and a 1 dp border.
const (
	placeholderPadding unit.Dp = 8 // STYLE-IMG-3
	placeholderBorder  unit.Dp = 1 // STYLE-IMG-3
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
	reasonNoPixels  = "decodes to no pixels"         // STYLE-IMG-3
	// reasonTooLargeFmt is the REASON of an image past the size cap, with its size (STYLE-IMG-1).
	reasonTooLargeFmt = "image too large: %dx%d" // STYLE-IMG-1, STYLE-IMG-3
)

// msgImageNotShown is the stderr line of an image that is not drawn (STYLE-IMG-3). It ends with a
// newline.
const msgImageNotShown = "md-viewer: image not shown: %s: %s\n" // STYLE-IMG-3

// Images is the document's images (SPEED-1, SPEED-5). The headers are probed before the window and
// the pixels are decoded by one worker. The frame goroutine and the worker share the entries, so
// every field below mu is read and written under mu. warn is written only before the window
// opens, and by layout on the frame goroutine.
type Images struct {
	warn   io.Writer
	decode func(path string) (image.Image, error)
	spawn  func(func())

	mu         sync.Mutex
	entries    map[string]*imageEntry // by the path of each LocalFile image
	queue      []*imageEntry          // waiting for the worker, first come first served (SPEED-5)
	running    bool                   // a worker is alive: it exits when the queue is empty (SPEED-6)
	invalidate func()                 // the window's invalidate, once Bind has set it
}

// imageState is where an image is in its life: probed and not drawn, probed and waiting for its
// pixels, queued for the worker, decoded, or failed to decode (STYLE-IMG-5). The zero value is
// stateNotShown.
type imageState uint8

const (
	stateNotShown imageState = iota // probed and not drawn: reason says why (STYLE-IMG-3)
	stateIdle                       // probed, its pixels not yet asked for
	stateQueued                     // asked for, and waiting for the worker
	stateReady                      // decoded: pixels holds them
	stateFailed                     // the decode failed: reason says why (STYLE-IMG-3)
)

// imageEntry is one local image file, probed once however many times the document names it.
type imageEntry struct {
	path     string
	size     image.Point   // the probed size: the box's size at natural size
	state    imageState    // guarded by Images.mu
	reason   string        // when not shown or failed: STYLE-IMG-3's REASON
	pixels   paint.ImageOp // when ready
	reported bool          // the line of a failed decode has been printed (STYLE-IMG-3)
}

// NewImages probes the headers of d's local images and prints a line to warn for each image
// occurrence that is not drawn (STYLE-IMG-3, SPEED-1). An error is a failed write, or a block that
// the walk does not know (fail fast).
func NewImages(d *doc.Document, warn io.Writer) (*Images, error) {
	return newImages(d, warn, decodeFile, goSpawn)
}

// newImages is NewImages with the decode function and the spawn function given, so that tests
// control the worker. spawn starts the worker; goSpawn starts it in a goroutine.
func newImages(d *doc.Document, warn io.Writer, decode func(path string) (image.Image, error), spawn func(func())) (*Images, error) {
	im := &Images{
		warn:       warn,
		decode:     decode,
		spawn:      spawn,
		entries:    make(map[string]*imageEntry),
		invalidate: func() {},
	}
	err := walkImages(d.Blocks, im.note)
	if err != nil {
		return nil, err
	}
	return im, nil
}

// walkImages calls visit for each image of blocks, in document order, through quotes and list items
// (DOC-11). A table or a heading holds no image block. A block type the walk does not know is an
// error that names it.
func walkImages(blocks []doc.Block, visit func(doc.Image) error) error {
	for _, b := range blocks {
		switch b := b.(type) {
		case doc.Image:
			if err := visit(b); err != nil {
				return err
			}
		case doc.Quote:
			if err := walkImages(b.Blocks, visit); err != nil {
				return err
			}
		case doc.List:
			for _, item := range b.Items {
				if err := walkImages(item.Blocks, visit); err != nil {
					return err
				}
			}
		case doc.Heading, doc.Paragraph, doc.CodeBlock, doc.Rule, doc.Table:
			// These hold no image block (DOC-11).
		default:
			return fmt.Errorf("render: no image walk for block %T", b)
		}
	}
	return nil
}

// note records one occurrence of an image: a local file is probed the first time its path is seen,
// and the line for an image that is not drawn is written once per occurrence (STYLE-IMG-3).
func (im *Images) note(img doc.Image) error {
	var reason string
	switch img.Target.Kind {
	case doc.External:
		reason = reasonRemote
	case doc.LocalFile:
		state, why := im.probe(img.Target.Value)
		if state != stateNotShown {
			return nil
		}
		reason = why
	case doc.Fragment:
		reason = reasonNotLocal
	case doc.Refused:
		reason = refusedReason(img.Target.Value)
	default:
		return fmt.Errorf("render: no image target for kind %d", img.Target.Kind)
	}
	_, err := fmt.Fprintf(im.warn, msgImageNotShown, img.Dest, reason)
	return err
}

// probe returns the state and the REASON of the local image at path, probing it on first use.
func (im *Images) probe(path string) (imageState, string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	e, ok := im.entries[path]
	if !ok {
		size, reason := probeImage(path)
		e = &imageEntry{path: path, size: size, state: stateIdle}
		if reason != "" {
			e.state = stateNotShown
			e.reason = reason
		}
		im.entries[path] = e
	}
	return e.state, e.reason
}

// refusedReason is the REASON for a destination that md-viewer does not follow (DOC-5): one with a
// host, such as ftp://, file://host/ or //host/, is a remote image; any other is not a local file
// (STYLE-IMG-3).
func refusedReason(dest string) string {
	u, err := url.Parse(dest)
	if err == nil && u.Host != "" {
		return reasonRemote
	}
	return reasonNotLocal
}

// Bind sets the function that the worker calls when an image's pixels are ready: the window's
// invalidate. Until it is bound, the calls do nothing, so layout without a window is valid. An image
// decoded before Bind is drawn on the next frame only, so Bind must precede the first layout.
func (im *Images) Bind(invalidate func()) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.invalidate = invalidate
}

// imageView is one image's state as layout sees it, copied out under mu.
type imageView struct {
	state  imageState
	size   image.Point   // the probed size
	reason string        // when not shown or failed
	pixels paint.ImageOp // when ready
	report bool          // a failed decode whose line is due now (STYLE-IMG-3)
}

// view is the state of the local image at path. An idle image is queued here, for the worker, and
// the worker is started when none is running (SPEED-5). A failed image is due its line once. A
// path that NewImages did not probe is an error that names it.
func (im *Images) view(path string) (imageView, error) {
	im.mu.Lock()
	e, ok := im.entries[path]
	if !ok {
		im.mu.Unlock()
		return imageView{}, fmt.Errorf("render: image %s was not probed by NewImages", path)
	}
	v := imageView{state: e.state, size: e.size, reason: e.reason}
	start := false
	switch e.state {
	case stateIdle:
		e.state = stateQueued
		im.queue = append(im.queue, e)
		v.state = stateQueued
		if !im.running {
			im.running = true
			start = true
		}
	case stateReady:
		v.pixels = e.pixels
	case stateFailed:
		v.report = !e.reported
		e.reported = true
	case stateQueued, stateNotShown:
		// Nothing to do: the image is waiting, or it is not drawn.
	default:
		im.mu.Unlock()
		return imageView{}, fmt.Errorf("render: no image state for value %d", e.state)
	}
	im.mu.Unlock()
	if start {
		im.spawn(im.work)
	}
	return v, nil
}

// work decodes the queued images one at a time, in the order they were asked for, and returns when
// the queue is empty (SPEED-5, SPEED-6). It runs in one goroutine at a time, because view starts it
// only when no worker is running.
func (im *Images) work() {
	for {
		e := im.next()
		if e == nil {
			return
		}
		img, err := im.decode(e.path)
		var px paint.ImageOp
		if err == nil {
			px = paint.NewImageOp(img)
		}
		im.finish(e, px, err)
	}
}

// next takes the head of the queue. When the queue is empty it records that no worker is running,
// and returns nil.
func (im *Images) next() *imageEntry {
	im.mu.Lock()
	defer im.mu.Unlock()
	if len(im.queue) == 0 {
		im.running = false
		return nil
	}
	e := im.queue[0]
	im.queue = im.queue[1:]
	return e
}

// finish records the outcome of e's decode, and invalidates the window so that the pixels are drawn
// (SPEED-5). A failed decode keeps its error text as the REASON (STYLE-IMG-3). The invalidate is
// called without mu held.
func (im *Images) finish(e *imageEntry, px paint.ImageOp, err error) {
	im.mu.Lock()
	if err != nil {
		e.state = stateFailed
		e.reason = err.Error()
	} else {
		e.state = stateReady
		e.pixels = px
	}
	invalidate := im.invalidate
	im.mu.Unlock()
	invalidate()
}

// layoutImage lays out one image as a block (DOC-11): its box while its pixels are pending, its
// pixels, or its placeholder (STYLE-IMG-1 to STYLE-IMG-5). A linked image is clickable over its
// whole rectangle (STYLE-IMG-4). It records its marks in tr, which may be nil.
func layoutImage(gtx layout.Context, ctx blockCtx, img doc.Image, tr *Trace) (layout.Dimensions, error) {
	if ctx.images == nil {
		return layout.Dimensions{}, fmt.Errorf("render: image %s has no Images", img.Dest)
	}
	if img.Link != nil && ctx.links == nil {
		return layout.Dimensions{}, fmt.Errorf("render: linked image %s with no link state", img.Dest)
	}
	v := imageView{state: stateNotShown}
	if img.Target.Kind == doc.LocalFile {
		var err error
		v, err = ctx.images.view(img.Target.Value)
		if err != nil {
			return layout.Dimensions{}, err
		}
	}
	if v.report {
		if _, err := fmt.Fprintf(ctx.images.warn, msgImageNotShown, img.Dest, v.reason); err != nil {
			return layout.Dimensions{}, err
		}
	}

	var r image.Rectangle
	switch v.state {
	case stateNotShown, stateFailed:
		rect, err := layoutPlaceholder(gtx, ctx, img, tr)
		if err != nil {
			return layout.Dimensions{}, err
		}
		r = rect
	case stateQueued:
		r = image.Rectangle{Max: imageBox(gtx, v.size)}
		fillRect(gtx, r, ctx.root.BgMuted)
		tr.add(Mark{Kind: MarkImageBox, Rect: r, Colour: ctx.root.BgMuted})
	case stateReady:
		r = image.Rectangle{Max: imageBox(gtx, v.size)}
		drawPixels(gtx, r, v.pixels)
		tr.add(Mark{Kind: MarkImagePixels, Rect: r})
	case stateIdle:
		return layout.Dimensions{}, fmt.Errorf("render: image %s was not queued", img.Dest)
	default:
		return layout.Dimensions{}, fmt.Errorf("render: no image state for value %d", v.state)
	}
	if img.Link != nil {
		linkArea(gtx, ctx.links, img.Link, r, tr)
	}
	// Baseline 0: an image has no text line (STYLE-LIST-2, leadsWithText).
	return layout.Dimensions{Size: image.Pt(r.Dx(), r.Dy())}, nil
}

// drawPixels draws the decoded pixels of an image in the box r, scaled from their own size to the
// box (STYLE-IMG-2). The box is the probed size, so a file that changed since the probe still fills
// the box the layout reserved. Only the box is painted.
func drawPixels(gtx layout.Context, r image.Rectangle, px paint.ImageOp) {
	iw, ih := px.Size().X, px.Size().Y
	sx := float32(r.Dx()) / float32(iw)
	sy := float32(r.Dy()) / float32(ih)
	clipStack := clip.Rect(r).Push(gtx.Ops)
	scale := op.Affine(f32.AffineId().Scale(f32.Pt(0, 0), f32.Pt(sx, sy))).Push(gtx.Ops)
	px.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	scale.Pop()
	clipStack.Pop()
}

// layoutPlaceholder draws img's alt text, or its destination when the alt is empty, as a placeholder:
// Italic FgMuted body text inside a 1 dp Border rectangle, with 8 dp of padding between the border
// and the text. The rectangle hugs the text (STYLE-IMG-3). It returns the rectangle, which is also
// its mark.
func layoutPlaceholder(gtx layout.Context, ctx blockCtx, img doc.Image, tr *Trace) (image.Rectangle, error) {
	text := img.Alt
	if text == "" {
		text = img.Dest
	}
	inset := gtx.Dp(placeholderPadding + placeholderBorder)
	muted := ctx.text
	muted.Fg = ctx.root.FgMuted
	mctx := ctx
	mctx.text = muted
	fp := mctx.flow(BodyStyle(muted), false, wrapProse)
	runs := []doc.Run{{Kind: doc.TextRun, Text: text, Style: doc.Emphasis}}
	fgtx := gtx
	fgtx.Constraints = layout.Constraints{Max: image.Pt(max(gtx.Constraints.Max.X-2*inset, 0), gtx.Constraints.Max.Y)}
	dims, err := at(gtx, tr, image.Pt(inset, inset), func() (layout.Dimensions, error) {
		return layoutFlow(fgtx, fp, runs, tr)
	})
	if err != nil {
		return image.Rectangle{}, err
	}
	r := image.Rect(0, 0, dims.Size.X+2*inset, dims.Size.Y+2*inset)
	border := gtx.Dp(placeholderBorder)
	fillRect(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+border), ctx.root.Border)
	fillRect(gtx, image.Rect(r.Min.X, r.Max.Y-border, r.Max.X, r.Max.Y), ctx.root.Border)
	fillRect(gtx, image.Rect(r.Min.X, r.Min.Y, r.Min.X+border, r.Max.Y), ctx.root.Border)
	fillRect(gtx, image.Rect(r.Max.X-border, r.Min.Y, r.Max.X, r.Max.Y), ctx.root.Border)
	tr.add(Mark{Kind: MarkPlaceholder, Rect: r, Colour: ctx.root.Border, Text: text})
	return r, nil
}

// openRegular opens the regular file at path. A directory and a non-regular file are judged by Stat
// and never opened, so a FIFO cannot block the caller (CLI-8). For a file that is not opened, the
// REASON is returned instead, and it names no path (STYLE-IMG-3).
func openRegular(path string) (*os.File, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, pathReason(err)
	}
	switch {
	case fi.IsDir():
		return nil, reasonDirectory
	case !fi.Mode().IsRegular():
		return nil, reasonNotFile
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, pathReason(err)
	}
	return f, ""
}

// probeImage reads the size of the image at path from its header (SPEED-1). A file that cannot be
// drawn gives the REASON its placeholder shows (STYLE-IMG-3), and a size of zero.
func probeImage(path string) (image.Point, string) {
	f, why := openRegular(path)
	if why != "" {
		return image.Point{}, why
	}
	cfg, format, err := image.DecodeConfig(f)
	cerr := f.Close()
	if why := headerReason(cfg, format, err); why != "" {
		return image.Point{}, why
	}
	if cerr != nil {
		return image.Point{}, pathReason(cerr)
	}
	return image.Pt(cfg.Width, cfg.Height), ""
}

// headerReason is the REASON of an image whose header image.DecodeConfig read as cfg, format and err,
// or "" when the image is drawn. A format that is not PNG or JPEG, and an image past the size cap, are
// judged here (STYLE-IMG-1, STYLE-IMG-3). The text names no path (CLI-8).
func headerReason(cfg image.Config, format string, err error) string {
	switch {
	case errors.Is(err, image.ErrFormat):
		return reasonFormat
	case err != nil:
		return pathReason(err)
	case !supportedFormat(format):
		return reasonFormat
	case tooLarge(cfg):
		return fmt.Sprintf(reasonTooLargeFmt, cfg.Width, cfg.Height)
	}
	return ""
}

// decodeFile decodes the pixels of the image at path. It runs in the decode worker, never on the
// frame path (SPEED-5). The file may have changed since the probe, so it is judged again before any
// pixel is decoded (STYLE-IMG-1). Its error text is a REASON, and names no path (CLI-8).
func decodeFile(path string) (image.Image, error) {
	f, why := openRegular(path)
	if why != "" {
		return nil, errors.New(why)
	}
	img, err := decodeOpened(f)
	cerr := f.Close()
	if err != nil {
		return nil, err
	}
	if cerr != nil {
		return nil, errors.New(pathReason(cerr))
	}
	return img, nil
}

// decodeOpened decodes the open file f: its header is judged first, and then its pixels are decoded
// from the start of the file.
func decodeOpened(f *os.File) (image.Image, error) {
	cfg, format, err := image.DecodeConfig(f)
	if why := headerReason(cfg, format, err); why != "" {
		return nil, errors.New(why)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New(pathReason(err))
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, errors.New(pathReason(err))
	}
	if why := noPixels(img); why != "" {
		return nil, errors.New(why)
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

// noPixels is the REASON of a decoded image with no pixels, or "" when it has some (STYLE-IMG-3).
func noPixels(img image.Image) string {
	if img.Bounds().Empty() {
		return reasonNoPixels
	}
	return ""
}

// goSpawn starts f in a goroutine.
func goSpawn(f func()) {
	go f()
}
