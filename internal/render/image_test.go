package render

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// envUpdateFixtures is the environment variable that makes TestImageFixtures write the fixtures
// (ARCHITECTURE "Fixtures in testdata/").
const envUpdateFixtures = "MDV_UPDATE_FIXTURES"

// fixtureDir is testdata/img, the images that the render tests share.
func fixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../../testdata/img")
	if err != nil {
		t.Fatalf("abs testdata/img: %v", err)
	}
	return dir
}

// TestImageFixtures writes the four fixtures when MDV_UPDATE_FIXTURES=1, and checks that they decode
// to the sizes and colours that ARCHITECTURE gives (STYLE-IMG-1).
func TestImageFixtures(t *testing.T) {
	dir := fixtureDir(t)
	if os.Getenv(envUpdateFixtures) == "1" {
		writeFixtures(t, dir)
	}
	checkFixtures(t, dir)
}

// writeFixtures writes the four fixture files into dir. Every encoder here is deterministic, so
// the files are the same on every run.
func writeFixtures(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	red := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for i := range red.Pix {
		if i%4 == 0 {
			red.Pix[i] = 0xff // R
		}
		if i%4 == 3 {
			red.Pix[i] = 0xff // A
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, red); err != nil {
		t.Fatalf("encode red-4x3.png: %v", err)
	}
	writeFile(t, filepath.Join(dir, "red-4x3.png"), pngBuf.Bytes())

	blue := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range blue.Pix {
		if i%4 == 2 || i%4 == 3 {
			blue.Pix[i] = 0xff // B and A
		}
	}
	var jpgBuf bytes.Buffer
	if err := jpeg.Encode(&jpgBuf, blue, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode blue-8x8.jpg: %v", err)
	}
	writeFile(t, filepath.Join(dir, "blue-8x8.jpg"), jpgBuf.Bytes())

	notes := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.White, color.Black})
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, notes, nil); err != nil {
		t.Fatalf("encode notes.gif: %v", err)
	}
	writeFile(t, filepath.Join(dir, "notes.gif"), gifBuf.Bytes())

	writeFile(t, filepath.Join(dir, "broken.png"), []byte("not a png"))
}

// writeFile writes data to path, failing the test on error.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// checkFixtures checks the four fixtures in dir: their sizes, their formats and their colours.
func checkFixtures(t *testing.T, dir string) {
	t.Helper()
	t.Run("red-4x3.png", func(t *testing.T) {
		img := decodeFixture(t, filepath.Join(dir, "red-4x3.png"), "png")
		checkBounds(t, img, 4, 3)
		for y := 0; y < 3; y++ {
			for x := 0; x < 4; x++ {
				if got := nrgba(t, img, x, y); got != (color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}) {
					t.Fatalf("pixel (%d,%d) is %v, want ff0000ff", x, y, got)
				}
			}
		}
	})

	t.Run("blue-8x8.jpg", func(t *testing.T) {
		img := decodeFixture(t, filepath.Join(dir, "blue-8x8.jpg"), "jpeg")
		checkBounds(t, img, 8, 8)
		// JPEG is lossy, so each channel is within 4 of 00 00 ff.
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				c := nrgba(t, img, x, y)
				if c.R > 4 || c.G > 4 || c.B < 0xff-4 {
					t.Fatalf("pixel (%d,%d) is %v, want each channel within 4 of 00 00 ff", x, y, c)
				}
			}
		}
	})

	t.Run("notes.gif", func(t *testing.T) {
		data := readFixture(t, filepath.Join(dir, "notes.gif"))
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode config of notes.gif: %v", err)
		}
		if format != "gif" || cfg.Width != 1 || cfg.Height != 1 {
			t.Fatalf("notes.gif is %s %dx%d, want a 1x1 gif", format, cfg.Width, cfg.Height)
		}
	})

	t.Run("broken.png", func(t *testing.T) {
		if got := string(readFixture(t, filepath.Join(dir, "broken.png"))); got != "not a png" {
			t.Fatalf("broken.png holds %q, want %q", got, "not a png")
		}
	})
}

// readFixture reads the fixture at path, failing the test on error.
func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// decodeFixture decodes the fixture at path, which must be of format, and fails the test otherwise.
func decodeFixture(t *testing.T, path, format string) image.Image {
	t.Helper()
	img, got, err := image.Decode(bytes.NewReader(readFixture(t, path)))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if got != format {
		t.Fatalf("%s decodes as %s, want %s", path, got, format)
	}
	return img
}

// checkBounds fails the test unless img is w by h pixels.
func checkBounds(t *testing.T, img image.Image, w, h int) {
	t.Helper()
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Fatalf("image is %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
}

// nrgba is the colour of pixel (x, y) of img, in NRGBA.
func nrgba(t *testing.T, img image.Image, x, y int) color.NRGBA {
	t.Helper()
	c, ok := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if !ok {
		t.Fatalf("pixel (%d,%d) does not convert to NRGBA", x, y)
	}
	return c
}

// pngSignature is the eight bytes that open every PNG file.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// pngChunk is one PNG chunk: its length, its type, its data and the CRC of its type and data.
func pngChunk(typ string, data []byte) []byte {
	var b bytes.Buffer
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(data)))
	b.Write(n[:])
	b.WriteString(typ)
	b.Write(data)
	binary.BigEndian.PutUint32(n[:], crc32.ChecksumIEEE(append([]byte(typ), data...)))
	b.Write(n[:])
	return b.Bytes()
}

// pngHeader is the signature and the IHDR of a w by h, 8-bit RGB PNG, and nothing after them: an
// image whose header is complete and whose pixels are absent.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], w)
	binary.BigEndian.PutUint32(ihdr[4:8], h)
	ihdr[8] = 8 // the bit depth
	ihdr[9] = 2 // the colour type: RGB
	var b bytes.Buffer
	b.Write(pngSignature)
	b.Write(pngChunk("IHDR", ihdr))
	return b.Bytes()
}

// pngEmptyIDAT is a 4 by 3 PNG whose header is valid and whose IDAT is a zlib stream of no bytes,
// so that the decoder reports "not enough pixel data" (png: invalid format).
func pngEmptyIDAT() []byte {
	var b bytes.Buffer
	b.Write(pngHeader(4, 3))
	b.Write(pngChunk("IDAT", []byte{0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01}))
	b.Write(pngChunk("IEND", nil))
	return b.Bytes()
}

// solidImage is a w by h image filled with c.
func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// writePNG writes an opaque black w by h PNG to path.
func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, solidImage(w, h, color.RGBA{A: 0xff})); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	writeFile(t, path, buf.Bytes())
}

// one is the single mark of kind in tr, and fails the test unless there is exactly one.
func one(t *testing.T, tr *Trace, kind MarkKind, name string) Mark {
	t.Helper()
	ms := marks(tr, kind)
	if len(ms) != 1 {
		t.Fatalf("%d %s marks, want 1", len(ms), name)
	}
	return ms[0]
}

// glyphSpan is the horizontal extent of the Glyphs marks gs: the left edge of the leftmost and the
// right edge of the rightmost. A single-line text's width is its span.
func glyphSpan(gs []Mark) (lo, hi int) {
	if len(gs) == 0 {
		return 0, 0
	}
	lo, hi = gs[0].Rect.Min.X, gs[0].Rect.Max.X
	for _, g := range gs {
		lo = min(lo, g.Rect.Min.X)
		hi = max(hi, g.Rect.Max.X)
	}
	return lo, hi
}

// laidWidth is the width of the text src lays out to, in a plain View at 800 px.
func laidWidth(t *testing.T, src string) int {
	t.Helper()
	e := newViewEnv(t, src, Light, metric1)
	_, tr := e.layout(0, 800)
	lo, hi := glyphSpan(needMarks(t, tr, MarkGlyphs, "Glyphs"))
	return hi - lo
}

// spawnRecorder is a spawn function that records each worker and starts none, so that a test
// decides when the worker runs (SPEED-5).
type spawnRecorder struct{ calls []func() }

// spawn records f.
func (s *spawnRecorder) spawn(f func()) {
	s.calls = append(s.calls, f)
}

// runWorker runs the one worker that sp recorded, and fails the test unless there is exactly one.
func runWorker(t *testing.T, sp *spawnRecorder) {
	t.Helper()
	if len(sp.calls) != 1 {
		t.Fatalf("%d workers started, want 1", len(sp.calls))
	}
	f := sp.calls[0]
	sp.calls = nil
	f()
}

// deferredEnv is a View of src in dir whose images record their worker and do not run it. The
// recorder is returned so that a test can see how many workers a layout started.
func deferredEnv(t *testing.T, src, dir string, pal Palette, metric unit.Metric) (*viewEnv, *spawnRecorder) {
	t.Helper()
	sp := &spawnRecorder{}
	d := parseIn(t, src, dir)
	e := viewWith(t, d, pal, metric, func(w io.Writer) (*Images, error) {
		return newImages(d, w, decodeFile, sp.spawn)
	})
	return e, sp
}

// newImagesOf is NewImages of d with the writer w, and fails the test on an error.
func newImagesOf(t *testing.T, d *doc.Document, w io.Writer) *Images {
	t.Helper()
	im, err := NewImages(d, w)
	if err != nil {
		t.Fatalf("NewImages: %v", err)
	}
	return im
}

// gatedDecoder is a decode function that the test holds: each call reports its path on started,
// then blocks until release is closed, then returns a 4 by 3 red image.
type gatedDecoder struct {
	calls   atomic.Int32
	started chan string
	release chan struct{}
}

// decode is the gated decode function.
func (g *gatedDecoder) decode(path string) (image.Image, error) {
	g.calls.Add(1)
	g.started <- path
	<-g.release
	return solidImage(4, 3, color.RGBA{R: 0xff, A: 0xff}), nil
}

// decodeLog is a decode function that records each path and returns a 1 by 1 image.
type decodeLog struct{ paths []string }

// decode records path.
func (l *decodeLog) decode(path string) (image.Image, error) {
	l.paths = append(l.paths, path)
	return solidImage(1, 1, color.RGBA{A: 0xff}), nil
}

// invalidateCounter counts the calls of its invalidate function. Each call also signals ch, without
// blocking, so that a test can wait for one.
type invalidateCounter struct {
	n  atomic.Int32
	ch chan struct{}
}

// newInvalidateCounter makes a counter whose channel holds one signal.
func newInvalidateCounter() *invalidateCounter {
	return &invalidateCounter{ch: make(chan struct{}, 1)}
}

// invalidate is the function a test binds to the Images.
func (c *invalidateCounter) invalidate() {
	c.n.Add(1)
	select {
	case c.ch <- struct{}{}:
	default:
	}
}

// awaitSignal waits for a signal on ch. The deadline only stops a test that would wait forever, so
// it is a failure and never a timing assumption.
func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Minute):
		t.Fatalf("no signal within a minute")
	}
}

// awaitPath waits for the decode function to start, and returns its path.
func awaitPath(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(time.Minute):
		t.Fatalf("no decode started within a minute")
		return ""
	}
}

// wantBases fails the test unless the base names of paths are want, in order.
func wantBases(t *testing.T, paths []string, want ...string) {
	t.Helper()
	var got []string
	for _, p := range paths {
		got = append(got, filepath.Base(p))
	}
	if !slices.Equal(got, want) {
		t.Fatalf("decoded %v, want %v", got, want)
	}
}

// failWriter is a writer whose every write fails.
type failWriter struct{}

// Write fails.
func (failWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

// TestImageBoxSize: an image is one dp per pixel, each side rounded to whole pixels once, and it is
// scaled down to the width available to it and never up (STYLE-IMG-2). The rows marked control pass
// on the stub: they pin the clamp at zero, and are not evidence of a red test.
func TestImageBoxSize(t *testing.T) {
	m15 := unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}
	tests := []struct {
		name    string
		size    image.Point
		metric  unit.Metric
		width   int
		want    image.Point
		control bool
	}{
		{"natural size", image.Pt(4, 3), metric1, 800, image.Pt(4, 3), false},
		{"two pixels per dp", image.Pt(4, 3), metric2, 800, image.Pt(8, 6), false},
		{"1.5 px per dp rounds 4.5 up", image.Pt(4, 3), m15, 800, image.Pt(6, 5), false},
		{"scaled to the width", image.Pt(1000, 500), metric1, 600, image.Pt(600, 300), false},
		{"as wide as the width is not scaled", image.Pt(600, 300), metric1, 600, image.Pt(600, 300), false},
		{"narrower than the width is not scaled up", image.Pt(300, 150), metric1, 600, image.Pt(300, 150), false},
		{"scaled at two pixels per dp", image.Pt(1000, 500), metric2, 1200, image.Pt(1200, 600), false},
		{"a side of 0.6 px rounds to 1", image.Pt(1000, 1), metric1, 600, image.Pt(600, 1), false},
		{"a tall image scaled to 2 px", image.Pt(3, 1000), metric1, 2, image.Pt(2, 667), false},
		{"a side of 0.1 px rounds to 0", image.Pt(1000, 1), metric1, 100, image.Pt(100, 0), true},
		{"zero width", image.Pt(4, 3), metric1, 0, image.Pt(0, 0), true},
		{"negative width", image.Pt(4, 3), metric1, -5, image.Pt(0, 0), true},
	}
	for _, tt := range tests {
		name := tt.name
		if tt.control {
			name = "control: " + name
		}
		t.Run(name, func(t *testing.T) {
			gtx := layout.Context{Metric: tt.metric, Constraints: layout.Constraints{Max: image.Pt(tt.width, 1<<20)}}
			if got := imageBox(gtx, tt.size); got != tt.want {
				t.Fatalf("box of %v at %d px is %v, want %v", tt.size, tt.width, got, tt.want)
			}
		})
	}
}

// TestProbeImage: the probe reads each file's size from its header, or gives the REASON that the
// placeholder shows (STYLE-IMG-1, STYLE-IMG-3). A directory and a non-regular file are not opened
// (Stat first). A missing file gives the operating system's error text, without the path.
func TestProbeImage(t *testing.T) {
	fix := fixtureDir(t)
	tmp := t.TempDir()
	cut := filepath.Join(tmp, "cut.png")
	writeFile(t, cut, pngHeader(4, 3)[:20])
	tests := []struct {
		name   string
		path   string
		size   image.Point
		reason string
	}{
		{"red PNG", filepath.Join(fix, "red-4x3.png"), image.Pt(4, 3), ""},
		{"blue JPEG", filepath.Join(fix, "blue-8x8.jpg"), image.Pt(8, 8), ""},
		{"GIF, which this test binary decodes", filepath.Join(fix, "notes.gif"), image.Point{}, "unsupported format"},
		{"not a PNG (the header sniff fails)", filepath.Join(fix, "broken.png"), image.Point{}, "unsupported format"},
		{"missing", filepath.Join(tmp, "nosuch.png"), image.Point{}, "no such file or directory"},
		{"directory", tmp, image.Point{}, "is a directory"},
		{"not a regular file", "/dev/null", image.Point{}, "not a regular file"},
		{"header cut short", cut, image.Point{}, "unexpected EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, reason := probeImage(tt.path)
			if reason != tt.reason {
				t.Fatalf("reason %q, want %q", reason, tt.reason)
			}
			if tt.reason == "" && size != tt.size {
				t.Fatalf("size %v, want %v", size, tt.size)
			}
		})
	}
}

// mustDecode is decodeFile of path, and fails the test on an error or a nil image.
func mustDecode(t *testing.T, path string) image.Image {
	t.Helper()
	img, err := decodeFile(path)
	if err != nil {
		t.Fatalf("decodeFile %s: %v", path, err)
	}
	if img == nil {
		t.Fatalf("decodeFile %s returned no image", path)
	}
	return img
}

// TestDecodeFile: a PNG or JPEG decodes to its pixels, and a file that fails to decode gives the
// decoder's error (STYLE-IMG-1, STYLE-IMG-3).
func TestDecodeFile(t *testing.T) {
	fix := fixtureDir(t)
	tmp := t.TempDir()
	empty := filepath.Join(tmp, "empty-idat.png")
	writeFile(t, empty, pngEmptyIDAT())
	t.Run("red PNG", func(t *testing.T) {
		img := mustDecode(t, filepath.Join(fix, "red-4x3.png"))
		checkBounds(t, img, 4, 3)
		if c := nrgba(t, img, 0, 0); c != (color.NRGBA{R: 0xff, A: 0xff}) {
			t.Fatalf("pixel (0,0) is %v, want red", c)
		}
	})
	t.Run("blue JPEG", func(t *testing.T) {
		checkBounds(t, mustDecode(t, filepath.Join(fix, "blue-8x8.jpg")), 8, 8)
	})
	t.Run("pixel data cut short", func(t *testing.T) {
		img, err := decodeFile(empty)
		if err == nil || img != nil || !strings.Contains(err.Error(), "not enough pixel data") {
			t.Fatalf("decodeFile gave image %v and error %v, want no image and an error containing %q", img != nil, err, "not enough pixel data")
		}
	})
	t.Run("GIF", func(t *testing.T) {
		img, err := decodeFile(filepath.Join(fix, "notes.gif"))
		if err == nil || img != nil {
			t.Fatalf("decodeFile of a GIF gave image %v and error %v, want an error (not png or jpeg)", img != nil, err)
		}
	})
}

// TestNewImagesWarnings: NewImages prints one stderr line for each image occurrence that is not
// drawn, in document order, and nothing for an image that is drawn, a heading or a table cell
// (STYLE-IMG-3, DOC-11).
func TestNewImagesWarnings(t *testing.T) {
	fix := fixtureDir(t)
	src := strings.Join([]string{
		"![A missing image](missing.png)",
		"",
		"> ![q](https://e.org/q.png)",
		"",
		"- ![](notes.gif)",
		"",
		"![ok](red-4x3.png)",
		"",
		"![ok again](red-4x3.png)",
		"",
		"# h ![x](missing.png)",
		"",
		"| c |",
		"| - |",
		"| ![y](missing.png) |",
	}, "\n")
	var warn bytes.Buffer
	newImagesOf(t, parseIn(t, src, fix), &warn)
	want := "md-viewer: image not shown: missing.png: no such file or directory\n" +
		"md-viewer: image not shown: https://e.org/q.png: remote images are not loaded\n" +
		"md-viewer: image not shown: notes.gif: unsupported format\n"
	if warn.String() != want {
		t.Errorf("stderr = %q, want %q", warn.String(), want)
	}

	tests := []struct {
		name, src, want string
	}{
		{"the same missing image twice gives two lines", "![m](missing.png)\n\n![m](missing.png)",
			"md-viewer: image not shown: missing.png: no such file or directory\n" +
				"md-viewer: image not shown: missing.png: no such file or directory\n"},
		{"an ftp destination is remote", "![r](ftp://e.org/r.png)",
			"md-viewer: image not shown: ftp://e.org/r.png: remote images are not loaded\n"},
		{"a fragment is not a local file", "![f](#x)", "md-viewer: image not shown: #x: not a local file\n"},
		{"a javascript destination is not a local file", "![j](javascript:x)",
			"md-viewer: image not shown: javascript:x: not a local file\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w bytes.Buffer
			newImagesOf(t, parseIn(t, tt.src, fix), &w)
			if w.String() != tt.want {
				t.Fatalf("stderr = %q, want %q", w.String(), tt.want)
			}
		})
	}
}

// TestNewImagesErrors: a failed write is an error (P12), and so is a block the image walk does not
// know, which names its type.
func TestNewImagesErrors(t *testing.T) {
	t.Run("a failed write", func(t *testing.T) {
		d := parseIn(t, "![m](missing.png)", fixtureDir(t))
		im, err := NewImages(d, failWriter{})
		if err == nil || im != nil {
			t.Fatalf("NewImages with a failing writer gave images %v and error %v, want an error", im != nil, err)
		}
	})
	t.Run("an unknown block", func(t *testing.T) {
		d := &doc.Document{Blocks: []doc.Block{nil}}
		im, err := NewImages(d, io.Discard)
		if err == nil || im != nil || !strings.Contains(err.Error(), "<nil>") {
			t.Fatalf("NewImages gave images %v and error %v, want an error naming <nil>", im != nil, err)
		}
	})
}

// TestImageNaturalSize: an image that is not yet decoded is drawn as a box at its final size, in
// BgMuted of its palette, and the element is the padding, the box and the padding (STYLE-IMG-2,
// STYLE-IMG-5). The worker is recorded and not run, so the box is the state throughout.
func TestImageNaturalSize(t *testing.T) {
	fix := fixtureDir(t)
	metric15 := unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}
	rows := []struct {
		name   string
		metric unit.Metric
		width  int
		box    image.Rectangle // at the padding of the window
	}{
		{"zoom 1", metric1, 800, image.Rect(45, 45, 49, 48)},
		{"zoom 2", metric2, 1600, image.Rect(90, 90, 98, 96)},
		{"zoom 1.5", metric15, 1200, image.Rect(68, 68, 74, 73)},
	}
	palettes := []struct {
		name string
		pal  Palette
	}{{"light", Light}, {"dark", Dark}}
	for _, p := range palettes {
		for _, tt := range rows {
			t.Run(p.name+" "+tt.name, func(t *testing.T) {
				e, sp := deferredEnv(t, "![a](red-4x3.png)", fix, p.pal, tt.metric)
				dims, tr := e.layout(0, tt.width)
				if len(sp.calls) != 1 {
					t.Fatalf("%d workers started by one layout, want 1", len(sp.calls))
				}
				box := one(t, tr, MarkImageBox, "ImageBox")
				if box.Rect != tt.box {
					t.Fatalf("box is %v, want %v", box.Rect, tt.box)
				}
				if box.Colour != p.pal.BgMuted {
					t.Fatalf("box is %v, want BgMuted %v", box.Colour, p.pal.BgMuted)
				}
				if n := len(marks(tr, MarkImagePixels)) + len(marks(tr, MarkPlaceholder)); n != 0 {
					t.Fatalf("%d pixel or placeholder marks before the decode, want none", n)
				}
				pad := tt.box.Min.Y
				if want := pad + tt.box.Dy() + pad; dims.Size.Y != want {
					t.Fatalf("element is %d px tall, want %d (padding %d, box %d, padding %d)", dims.Size.Y, want, pad, tt.box.Dy(), pad)
				}
			})
		}
	}
}

// TestImageScaledDown: an image wider than the width available to it is scaled down to that width,
// keeping its aspect ratio. The width is the window's content, less the list or quote around the
// image (STYLE-IMG-2). A 300 by 150 image is never scaled up.
func TestImageScaledDown(t *testing.T) {
	tmp := t.TempDir()
	writePNG(t, filepath.Join(tmp, "big.png"), 1000, 500)
	writePNG(t, filepath.Join(tmp, "small.png"), 300, 150)
	rows := []struct {
		name   string
		src    string
		metric unit.Metric
		width  int
		box    image.Rectangle
	}{
		{"top level", "![a](big.png)", metric1, 630, image.Rect(15, 15, 615, 315)},
		{"in a list item", "- ![a](big.png)", metric1, 630, image.Rect(47, 15, 615, 299)},
		{"in a quote", "> ![a](big.png)", metric1, 630, image.Rect(35, 15, 599, 297)},
		{"two pixels per dp", "![a](big.png)", metric2, 1260, image.Rect(30, 30, 1230, 630)},
		{"a smaller image is not scaled up", "![a](small.png)", metric1, 630, image.Rect(15, 15, 315, 165)},
	}
	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			e, sp := deferredEnv(t, tt.src, tmp, Light, tt.metric)
			_, tr := e.layout(0, tt.width)
			if len(sp.calls) != 1 {
				t.Fatalf("%d workers started by one layout, want 1", len(sp.calls))
			}
			box := one(t, tr, MarkImageBox, "ImageBox")
			if box.Rect != tt.box {
				t.Fatalf("box is %v, want %v", box.Rect, tt.box)
			}
		})
	}
}

// TestImagePlaceholders: an image that is not drawn is a placeholder: its alt text, or its
// destination when the alt is empty, in Italic FgMuted text in a 1 dp Border rectangle with 8 dp of
// padding. The rectangle hugs the text (STYLE-IMG-3), and it prints one stderr line of STYLE-IMG-3's
// form.
func TestImagePlaceholders(t *testing.T) {
	fix := fixtureDir(t)
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "cut.png"), pngHeader(4, 3)[:20])
	writeFile(t, filepath.Join(tmp, "huge.png"), pngHeader(8193, 1))
	const (
		inset = 9  // STYLE-IMG-3: the 8 dp padding and the 1 dp border, on each side
		line  = 24 // STYLE-TEXT-1: one body line
		edge  = 45 // DOC-LAYOUT-1: the padding of a window of 800 dp
	)
	rows := []struct {
		name, src, dir, text, stderr string
		italic                       bool // checked against Italic text: plain words only
	}{
		{"missing", "![A missing image](missing.png)", fix, "A missing image",
			"md-viewer: image not shown: missing.png: no such file or directory\n", true},
		{"a GIF", "![A gif](notes.gif)", fix, "A gif",
			"md-viewer: image not shown: notes.gif: unsupported format\n", true},
		{"not an image", "![A broken image](broken.png)", fix, "A broken image",
			"md-viewer: image not shown: broken.png: unsupported format\n", true},
		{"remote", "![A remote image](https://example.com/r.png)", fix, "A remote image",
			"md-viewer: image not shown: https://example.com/r.png: remote images are not loaded\n", true},
		{"remote with no alt text shows its destination", "![](https://example.com/r.png)", fix,
			"https://example.com/r.png",
			"md-viewer: image not shown: https://example.com/r.png: remote images are not loaded\n", false},
		{"ftp", "![A ftp image](ftp://e.org/r.png)", fix, "A ftp image",
			"md-viewer: image not shown: ftp://e.org/r.png: remote images are not loaded\n", true},
		{"fragment", "![A fragment](#x)", fix, "A fragment",
			"md-viewer: image not shown: #x: not a local file\n", true},
		{"a directory", "![d]()", fix, "d",
			"md-viewer: image not shown: : is a directory\n", false},
		{"header cut short", "![A bad header](cut.png)", tmp, "A bad header",
			"md-viewer: image not shown: cut.png: unexpected EOF\n", true},
		{"too large", "![A huge image](huge.png)", tmp, "A huge image",
			"md-viewer: image not shown: huge.png: image too large: 8193x1\n", true},
	}
	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			e := newViewEnvIn(t, tt.src, tt.dir, Light, metric1)
			dims, tr := e.layout(0, 800)
			ph := one(t, tr, MarkPlaceholder, "Placeholder")
			if ph.Text != tt.text {
				t.Fatalf("placeholder text %q, want %q", ph.Text, tt.text)
			}
			if ph.Colour != Light.Border {
				t.Fatalf("placeholder colour %v, want Border %v", ph.Colour, Light.Border)
			}
			if ph.Rect.Min != image.Pt(edge, edge) {
				t.Fatalf("placeholder starts at %v, want %v", ph.Rect.Min, image.Pt(edge, edge))
			}
			gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
			lo, hi := glyphSpan(gs)
			if want := (hi - lo) + 2*inset; ph.Rect.Dx() != want {
				t.Fatalf("placeholder is %d px wide, want %d (the text %d px plus %d)", ph.Rect.Dx(), want, hi-lo, 2*inset)
			}
			if ph.Rect.Dy() != line+2*inset {
				t.Fatalf("placeholder is %d px tall, want %d (one line plus %d)", ph.Rect.Dy(), line+2*inset, 2*inset)
			}
			text := ph.Rect.Inset(inset)
			for _, g := range gs {
				if !within(text, g.Rect) {
					t.Fatalf("glyphs %q at %v lie outside the text area %v", g.Text, g.Rect, text)
				}
				if g.Colour != Light.FgMuted {
					t.Fatalf("glyphs %q are %v, want FgMuted %v", g.Text, g.Colour, Light.FgMuted)
				}
			}
			if tt.italic {
				ital := laidWidth(t, "*"+tt.text+"*")
				plain := laidWidth(t, tt.text)
				if plain == ital {
					t.Fatalf("italic and plain %q are both %d px: the check cannot tell them apart", tt.text, ital)
				}
				if hi-lo != ital {
					t.Fatalf("text is %d px wide, want the italic width %d (plain is %d)", hi-lo, ital, plain)
				}
			}
			if n := len(marks(tr, MarkImageBox)) + len(marks(tr, MarkImagePixels)); n != 0 {
				t.Fatalf("%d box or pixel marks for a placeholder, want none", n)
			}
			if want := edge + line + 2*inset + edge; dims.Size.Y != want {
				t.Fatalf("element is %d px tall, want %d (padding, placeholder, padding)", dims.Size.Y, want)
			}
			if e.warn.String() != tt.stderr {
				t.Fatalf("stderr = %q, want %q", e.warn.String(), tt.stderr)
			}
		})
	}

	t.Run("in a quote the text and the border keep their muted colours", func(t *testing.T) {
		e := newViewEnvIn(t, "> ![A missing image](missing.png)", fix, Light, metric1)
		_, tr := e.layout(0, 800)
		ph := one(t, tr, MarkPlaceholder, "Placeholder")
		if ph.Colour != Light.Border {
			t.Fatalf("placeholder colour %v, want Border %v", ph.Colour, Light.Border)
		}
		for _, g := range needMarks(t, tr, MarkGlyphs, "Glyphs") {
			if g.Colour != Light.FgMuted {
				t.Fatalf("glyphs %q are %v, want FgMuted %v", g.Text, g.Colour, Light.FgMuted)
			}
		}
	})
}

// TestImageSizeCap: an image whose header declares more than 8192 px on a side, or more than 40
// million pixels, is a placeholder with REASON image too large: WxH (STYLE-IMG-1). The headers are
// generated, with no pixel data.
func TestImageSizeCap(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		w, h uint32
		ok   bool
	}{
		{8192, 1, true},
		{1, 8192, true},
		{8000, 5000, true}, // exactly 40 million pixels
		{8193, 1, false},
		{1, 8193, false},
		{8000, 5001, false},
		{8192, 8192, false},
	}
	for _, tt := range tests {
		path := filepath.Join(dir, fmt.Sprintf("%dx%d.png", tt.w, tt.h))
		writeFile(t, path, pngHeader(tt.w, tt.h))
		size, reason := probeImage(path)
		if tt.ok {
			if reason != "" || size != image.Pt(int(tt.w), int(tt.h)) {
				t.Fatalf("%dx%d: size %v reason %q, want drawn at %dx%d", tt.w, tt.h, size, reason, tt.w, tt.h)
			}
			continue
		}
		if want := fmt.Sprintf("image too large: %dx%d", tt.w, tt.h); reason != want {
			t.Fatalf("%dx%d: reason %q, want %q", tt.w, tt.h, reason, want)
		}
	}
}

// TestImageLazyDecode: before the block is laid out no pixels are decoded; the first layout starts
// the decode and draws the box; the bound invalidate is called once when the decode is done; the
// next layout draws the pixels in the same rectangle (SPEED-5, STYLE-IMG-5). A real goroutine runs
// the decode, and the test synchronises with channels only.
func TestImageLazyDecode(t *testing.T) {
	fix := fixtureDir(t)
	red := filepath.Join(fix, "red-4x3.png")
	gate := &gatedDecoder{started: make(chan string), release: make(chan struct{})}
	inv := newInvalidateCounter()
	d := parseIn(t, "![a](red-4x3.png)", fix)
	e := viewWith(t, d, Light, metric1, func(w io.Writer) (*Images, error) {
		return newImages(d, w, gate.decode, goSpawn)
	})
	e.images.Bind(inv.invalidate)
	if n := gate.calls.Load(); n != 0 {
		t.Fatalf("%d decodes before any layout, want none", n)
	}

	dims1, tr1 := e.layout(0, 800)
	box := one(t, tr1, MarkImageBox, "ImageBox")
	if box.Rect != image.Rect(45, 45, 49, 48) || box.Colour != Light.BgMuted {
		t.Fatalf("first layout box %v in %v, want (45,45)-(49,48) in BgMuted", box.Rect, box.Colour)
	}
	if n := len(marks(tr1, MarkImagePixels)); n != 0 {
		t.Fatalf("%d pixel marks before the decode is done, want none", n)
	}

	if got := awaitPath(t, gate.started); got != red {
		t.Fatalf("decode started for %q, want %q", got, red)
	}

	dims2, tr2 := e.layout(0, 800)
	if box := one(t, tr2, MarkImageBox, "ImageBox"); box.Rect != image.Rect(45, 45, 49, 48) {
		t.Fatalf("box while the decode is blocked is %v, want (45,45)-(49,48)", box.Rect)
	}
	if n := len(marks(tr2, MarkImagePixels)); n != 0 || dims2 != dims1 {
		t.Fatalf("while the decode is blocked: %d pixel marks, dims %+v want %+v", n, dims2, dims1)
	}
	if n := gate.calls.Load(); n != 1 {
		t.Fatalf("%d decodes started, want 1", n)
	}

	close(gate.release)
	awaitSignal(t, inv.ch)
	if n := inv.n.Load(); n != 1 {
		t.Fatalf("invalidate called %d times, want 1", n)
	}

	dims3, tr3 := e.layout(0, 800)
	if px := one(t, tr3, MarkImagePixels, "ImagePixels"); px.Rect != image.Rect(45, 45, 49, 48) {
		t.Fatalf("pixels at %v, want (45,45)-(49,48)", px.Rect)
	}
	if n := len(marks(tr3, MarkImageBox)); n != 0 || dims3 != dims1 {
		t.Fatalf("after the decode: %d box marks, dims %+v want %+v", n, dims3, dims1)
	}

	dims4, tr4 := e.layout(0, 800)
	if px := one(t, tr4, MarkImagePixels, "ImagePixels"); px.Rect != image.Rect(45, 45, 49, 48) {
		t.Fatalf("pixels again at %v, want (45,45)-(49,48)", px.Rect)
	}
	if gate.calls.Load() != 1 || inv.n.Load() != 1 || dims4 != dims1 {
		t.Fatalf("a later layout decoded or invalidated again: decodes %d, invalidates %d", gate.calls.Load(), inv.n.Load())
	}
	if e.warn.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing", e.warn.String())
	}
}

// TestImageDecodeOneAtATime: one worker decodes the images in the order they are first laid out,
// one at a time (SPEED-5). A second worker starts only when the first has exited, and a request for
// an image that is already decoded starts nothing. The workers run on the test goroutine.
func TestImageDecodeOneAtATime(t *testing.T) {
	tmp := t.TempDir()
	writePNG(t, filepath.Join(tmp, "a.png"), 2, 2)
	writePNG(t, filepath.Join(tmp, "b.png"), 3, 3)
	writePNG(t, filepath.Join(tmp, "c.png"), 4, 4)
	d := parseIn(t, "![a](a.png)\n\n![b](b.png)\n\n![c](c.png)", tmp)
	log := &decodeLog{}
	sp := &spawnRecorder{}
	inv := newInvalidateCounter()
	e := viewWith(t, d, Light, metric1, func(w io.Writer) (*Images, error) {
		return newImages(d, w, log.decode, sp.spawn)
	})
	e.images.Bind(inv.invalidate)

	e.layout(0, 800)
	e.layout(1, 800)
	if len(sp.calls) != 1 {
		t.Fatalf("%d workers started by two layouts, want 1", len(sp.calls))
	}
	if len(log.paths) != 0 {
		t.Fatalf("%d decodes before the worker runs, want none", len(log.paths))
	}

	runWorker(t, sp)
	wantBases(t, log.paths, "a.png", "b.png")
	if n := inv.n.Load(); n != 2 {
		t.Fatalf("invalidate called %d times, want 2", n)
	}
	for i := 0; i < 2; i++ {
		_, tr := e.layout(i, 800)
		one(t, tr, MarkImagePixels, "ImagePixels")
	}

	e.layout(2, 800)
	if len(sp.calls) != 1 {
		t.Fatalf("%d workers started by a third image, want 1 (the first exited)", len(sp.calls))
	}
	runWorker(t, sp)
	wantBases(t, log.paths, "a.png", "b.png", "c.png")

	_, tr := e.layout(0, 800)
	one(t, tr, MarkImagePixels, "ImagePixels")
	if len(sp.calls) != 0 {
		t.Fatalf("laying out a decoded image started %d workers, want none", len(sp.calls))
	}
	wantBases(t, log.paths, "a.png", "b.png", "c.png")
}

// TestImageDecodeFailure: an image whose header is valid and whose pixels are cut short is a box,
// then a placeholder once the decode has failed. Its stderr line is printed once, when the
// placeholder is first laid out, because a decode can fail after the window has opened.
func TestImageDecodeFailure(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, "bad.png"), pngEmptyIDAT())
	sp := &spawnRecorder{}
	d := parseIn(t, "![bad image](bad.png)", tmp)
	e := viewWith(t, d, Light, metric1, func(w io.Writer) (*Images, error) {
		return newImages(d, w, decodeFile, sp.spawn)
	})

	_, tr1 := e.layout(0, 800)
	one(t, tr1, MarkImageBox, "ImageBox")
	if e.warn.Len() != 0 {
		t.Fatalf("stderr before the decode fails = %q, want nothing", e.warn.String())
	}

	runWorker(t, sp)

	_, tr2 := e.layout(0, 800)
	if ph := one(t, tr2, MarkPlaceholder, "Placeholder"); ph.Text != "bad image" {
		t.Fatalf("placeholder text %q, want %q", ph.Text, "bad image")
	}
	const want = "md-viewer: image not shown: bad.png: png: invalid format: not enough pixel data\n"
	if e.warn.String() != want {
		t.Fatalf("stderr = %q, want %q", e.warn.String(), want)
	}

	e.layout(0, 800)
	if e.warn.String() != want {
		t.Fatalf("a second layout printed again: stderr = %q", e.warn.String())
	}
	if len(sp.calls) != 0 {
		t.Fatalf("a failed image started %d more workers, want none", len(sp.calls))
	}
}

// TestImageError: a doc.Image with no Images is an error that names the image, and a local image
// that the View's Images did not probe is an error that names its path.
func TestImageError(t *testing.T) {
	t.Run("no Images", func(t *testing.T) {
		e := newViewEnv(t, "", Light, metric1)
		ctx := blockCtx{text: Light, root: Light}
		img := doc.Image{Alt: "x", Dest: "x.png", Target: doc.Target{Kind: doc.LocalFile, Value: "/x/x.png", Dest: "x.png"}}
		if _, err := layoutImage(e.context(800), ctx, img, nil); err == nil || !strings.Contains(err.Error(), "image") {
			t.Fatalf("layoutImage error %v, want one naming the image", err)
		}
	})
	t.Run("a path that was not probed", func(t *testing.T) {
		d := parseIn(t, "![a](/x/y.png)", "/doc")
		e := viewWith(t, d, Light, metric1, func(w io.Writer) (*Images, error) {
			return NewImages(&doc.Document{}, w)
		})
		e.view.Layout(e.context(800), 0, nil)
		if err := e.view.Err(); err == nil || !strings.Contains(err.Error(), "/x/y.png") {
			t.Fatalf("Err() = %v, want an error naming /x/y.png", err)
		}
	})
}

// TestLinkedImage: an image that is a link's content has a LinkArea over its whole box, its pixels
// or its placeholder, and the hand cursor over it (STYLE-IMG-4, LINK-1). The placeholder's text
// keeps FgMuted, not Accent, because it is not a text link (STYLE-IMG-3).
func TestLinkedImage(t *testing.T) {
	fix := fixtureDir(t)
	want := doc.Target{Kind: doc.External, Value: "https://e.org/", Dest: "https://e.org/"}
	const linked = "[![a](red-4x3.png)](https://e.org/)"

	t.Run("the box", func(t *testing.T) {
		e, sp := deferredEnv(t, linked, fix, Light, metric1)
		_, tr := e.layout(0, 800)
		if len(sp.calls) != 1 {
			t.Fatalf("%d workers started by one layout, want 1", len(sp.calls))
		}
		box := one(t, tr, MarkImageBox, "ImageBox")
		la := one(t, tr, MarkLinkArea, "LinkArea")
		if la.Rect != box.Rect {
			t.Fatalf("link area %v, want the box %v", la.Rect, box.Rect)
		}
		if la.Link == nil || *la.Link != want {
			t.Fatalf("link target %+v, want %+v", la.Link, want)
		}
	})

	t.Run("the pixels", func(t *testing.T) {
		e, sp := deferredEnv(t, linked, fix, Light, metric1)
		e.layout(0, 800)
		runWorker(t, sp)
		_, tr := e.layout(0, 800)
		px := one(t, tr, MarkImagePixels, "ImagePixels")
		la := one(t, tr, MarkLinkArea, "LinkArea")
		if la.Rect != px.Rect {
			t.Fatalf("link area %v, want the pixels %v", la.Rect, px.Rect)
		}
	})

	t.Run("a placeholder", func(t *testing.T) {
		e := newViewEnvIn(t, "[![A missing image](missing.png)](https://e.org/)", fix, Light, metric1)
		_, tr := e.layout(0, 800)
		ph := one(t, tr, MarkPlaceholder, "Placeholder")
		la := one(t, tr, MarkLinkArea, "LinkArea")
		if la.Rect != ph.Rect {
			t.Fatalf("link area %v, want the placeholder %v", la.Rect, ph.Rect)
		}
		for _, g := range needMarks(t, tr, MarkGlyphs, "Glyphs") {
			if g.Colour != Light.FgMuted {
				t.Fatalf("linked placeholder glyphs %q are %v, want FgMuted %v", g.Text, g.Colour, Light.FgMuted)
			}
		}
	})

	t.Run("click", func(t *testing.T) {
		e, sp := deferredEnv(t, linked, fix, Light, metric1)
		// Frame 1 lays out, which makes the link's gesture. Frame 2 polls before it lays out, as
		// TestViewClicked does.
		e.frame(0, 800)
		if len(sp.calls) != 1 {
			t.Fatalf("%d workers started by one frame, want 1", len(sp.calls))
		}
		if early, polled := e.poll(); polled {
			t.Fatalf("a poll with no input reported %+v", early)
		}
		tr := e.frame(0, 800)
		la := one(t, tr, MarkLinkArea, "LinkArea")
		e.router.Queue(move(centre(la.Rect)), press(centre(la.Rect)), release(centre(la.Rect)))
		if got, ok := e.poll(); !ok || got != want {
			t.Fatalf("poll after the click = %+v, %t; want %+v and true", got, ok, want)
		}
		if again, more := e.poll(); more {
			t.Fatalf("second poll in the same frame reported %+v again", again)
		}
	})

	t.Run("cursor", func(t *testing.T) {
		e, sp := deferredEnv(t, linked, fix, Light, metric1)
		tr := e.frame(0, 800)
		if len(sp.calls) != 1 {
			t.Fatalf("%d workers started by one frame, want 1", len(sp.calls))
		}
		la := one(t, tr, MarkLinkArea, "LinkArea")
		e.router.Queue(move(centre(la.Rect)))
		e.frame(0, 800)
		if c := e.router.Cursor(); c != pointer.CursorPointer {
			t.Fatalf("over a linked image the cursor is %v, want CursorPointer", c)
		}
	})
}
