package render

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
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
