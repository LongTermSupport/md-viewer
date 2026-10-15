package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
	"github.com/LongTermSupport/md-viewer/internal/render"
)

// stepClock is a fake Clock that advances 1 ms per reading, so no test reads the wall clock.
type stepClock struct{ t time.Time }

func (c *stepClock) Now() time.Time {
	now := c.t
	c.t = c.t.Add(time.Millisecond)
	return now
}

// prepareRun is the outcome of one Prepare call.
type prepareRun struct {
	prep           Prepared
	code           ExitCode
	stdout, stderr string
}

// runPrepare calls Prepare with args and a fake clock, and captures both streams.
func runPrepare(t *testing.T, args ...string) prepareRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	clock := &stepClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	prep, code := Prepare(args, &stdout, &stderr, clock)
	return prepareRun{prep: prep, code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// checkRun asserts every observable result of a Prepare call. The literals at each call site
// are the expectations, so the test never reuses the constants it checks.
func checkRun(t *testing.T, r prepareRun, stdout, stderr string, code ExitCode, open bool) {
	t.Helper()
	if r.stdout != stdout {
		t.Errorf("stdout = %q, want %q", r.stdout, stdout)
	}
	if r.stderr != stderr {
		t.Errorf("stderr = %q, want %q", r.stderr, stderr)
	}
	if r.code != code {
		t.Errorf("exit code = %d, want %d", r.code, code)
	}
	if r.prep.OpenWindow != open {
		t.Errorf("OpenWindow = %v, want %v", r.prep.OpenWindow, open)
	}
}

// writeFile writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestPrepareArgs covers the output that moved from main into Prepare: help, version and
// usage errors (CLI-3, CLI-4, CLI-5, CLI-6, CLI-7).
func TestPrepareArgs(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		checkRun(t, runPrepare(t, "--help"), Usage, "", 0, false)
	})

	t.Run("version", func(t *testing.T) {
		r := runPrepare(t, "--version")
		if !strings.HasPrefix(r.stdout, "md-viewer ") {
			t.Errorf("stdout = %q, want prefix %q", r.stdout, "md-viewer ")
		}
		if strings.Count(r.stdout, "\n") != 1 || !strings.HasSuffix(r.stdout, "\n") {
			t.Errorf("stdout = %q, want one line ending in a newline", r.stdout)
		}
		if r.stderr != "" {
			t.Errorf("stderr = %q, want empty", r.stderr)
		}
		if r.code != 0 {
			t.Errorf("exit code = %d, want 0", r.code)
		}
		if r.prep.OpenWindow {
			t.Errorf("OpenWindow = true, want false")
		}
	})

	t.Run("no arguments", func(t *testing.T) {
		const stderr = "md-viewer: missing FILE argument\nRun 'md-viewer --help' for usage.\n"
		checkRun(t, runPrepare(t), "", stderr, 2, false)
	})

	// FILE is not read when --help is given, so a missing file is not an error.
	t.Run("help with a missing FILE", func(t *testing.T) {
		checkRun(t, runPrepare(t, "--help", "/no/such/file.md"), Usage, "", 0, false)
	})
}

// TestPrepareFileErrors checks CLI-8's four messages, byte for byte, with exit 1.
func TestPrepareFileErrors(t *testing.T) {
	tmp := t.TempDir()

	big := filepath.Join(tmp, "big.md")
	writeFile(t, big, "")
	if err := os.Truncate(big, 67108864+1); err != nil {
		t.Fatalf("truncate %s: %v", big, err)
	}
	nul := filepath.Join(tmp, "nul.md")
	writeFile(t, nul, "a\x00b")

	cases := []struct {
		name, path, stderr string
	}{
		{
			name:   "missing file",
			path:   filepath.Join(tmp, "nosuch.md"),
			stderr: "md-viewer: cannot read " + filepath.Join(tmp, "nosuch.md") + ": no such file or directory\n",
		},
		{
			// The path is as typed: a relative FILE is reported relative.
			name:   "relative missing file",
			path:   "./nosuch-m2.md",
			stderr: "md-viewer: cannot read ./nosuch-m2.md: no such file or directory\n",
		},
		{
			name:   "directory",
			path:   tmp,
			stderr: "md-viewer: cannot read " + tmp + ": is a directory\n",
		},
		{
			name:   "not a regular file",
			path:   "/dev/zero",
			stderr: "md-viewer: cannot read /dev/zero: not a regular file\n",
		},
		{
			name:   "too large",
			path:   big,
			stderr: "md-viewer: " + big + " is too large: 67108865 bytes, the limit is 67108864\n",
		},
		{
			name:   "NUL byte",
			path:   nul,
			stderr: "md-viewer: " + nul + " is not a text file: it contains a NUL byte\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkRun(t, runPrepare(t, c.path), "", c.stderr, 1, false)
		})
	}
}

// TestPrepareHTMLLine checks DOC-9's line: one for a file with HTML, none for a file without.
// A loaded file opens the window (OpenWindow) with exit 0.
func TestPrepareHTMLLine(t *testing.T) {
	tmp := t.TempDir()

	html := filepath.Join(tmp, "html.md")
	writeFile(t, html, "<div>x</div>\n\na <b>c</b>\n")
	checkRun(t, runPrepare(t, html), "",
		"md-viewer: 3 HTML fragment(s) not shown: HTML is not rendered\n", 0, true)

	plain := filepath.Join(tmp, "plain.md")
	writeFile(t, plain, "# a\n")
	checkRun(t, runPrepare(t, plain), "", "", 0, true)
}

// failWriter is a writer whose every write fails.
type failWriter struct{}

// Write fails.
func (failWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

// TestPrepareImages: after DOC-9's line, one stderr line for each image that is not drawn, in
// document order (STYLE-IMG-3, SPEED-1). The file loads with its images probed. A failed write to
// stderr is ExitFailure and opens no window (P12).
func TestPrepareImages(t *testing.T) {
	tmp := t.TempDir()
	var ok bytes.Buffer
	if err := png.Encode(&ok, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode ok.png: %v", err)
	}
	writeFile(t, filepath.Join(tmp, "ok.png"), ok.String())
	doc := filepath.Join(tmp, "doc.md")
	writeFile(t, doc, "<b>x</b>\n\n![A missing image](missing.png)\n\n![r](https://e.org/r.png)\n\n![ok](ok.png)\n")

	r := runPrepare(t, doc)
	const stderr = "md-viewer: 2 HTML fragment(s) not shown: HTML is not rendered\n" +
		"md-viewer: image not shown: missing.png: no such file or directory\n" +
		"md-viewer: image not shown: https://e.org/r.png: remote images are not loaded\n"
	checkRun(t, r, "", stderr, 0, true)
	if r.prep.images == nil {
		t.Errorf("Prepared.images is nil, want the document's images")
	}

	t.Run("a failed write to stderr", func(t *testing.T) {
		missing := filepath.Join(tmp, "missing.md")
		writeFile(t, missing, "![m](missing.png)\n")
		clock := &stepClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
		var stdout bytes.Buffer
		prep, code := Prepare([]string{missing}, &stdout, failWriter{}, clock)
		if code != ExitFailure || prep.OpenWindow {
			t.Fatalf("exit code %d, OpenWindow %t; want %d and false", code, prep.OpenWindow, ExitFailure)
		}
	})
}

// scriptedClock returns its readings in order. A read past the end returns the last reading and
// counts as an overrun, which a test asserts is 0, so no step reads the clock more often than the
// script says.
type scriptedClock struct {
	readings []time.Time
	next     int
	overruns int
}

// Now returns the next reading.
func (c *scriptedClock) Now() time.Time {
	if c.next >= len(c.readings) {
		c.overruns++
		return c.readings[len(c.readings)-1]
	}
	now := c.readings[c.next]
	c.next++
	return now
}

// TestPrepareTitleAndPalette: the window is titled with the file's base name (WIN-1), its canvas
// is the palette's canvas (STYLE-PAL-3), the viewer's text is the palette's Fg (STYLE-PAL-2), and
// the window is not closed after the first frame unless --time is given (CLI-6, SPEED-2). The
// colours are literals.
func TestPrepareTitleAndPalette(t *testing.T) {
	elements := filepath.Join(testdataDir(t), "elements.md")
	cases := []struct {
		name   string
		args   []string
		canvas color.NRGBA
		fg     color.NRGBA
		exit   bool
	}{
		{
			name:   "light",
			args:   []string{elements},
			canvas: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			fg:     color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff},
		},
		{
			name:   "dark",
			args:   []string{"--dark", elements},
			canvas: color.NRGBA{R: 0x0d, G: 0x11, B: 0x17, A: 0xff},
			fg:     color.NRGBA{R: 0xf0, G: 0xf6, B: 0xfc, A: 0xff},
		},
		{
			name:   "time",
			args:   []string{"--time", elements},
			canvas: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			fg:     color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff},
			exit:   true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runPrepare(t, c.args...)
			if r.code != 0 || !r.prep.OpenWindow {
				t.Fatalf("Prepare: exit code %d, OpenWindow %t; want 0 and true", r.code, r.prep.OpenWindow)
			}
			cfg := r.prep.Window(time.Time{})
			if cfg.Title != "elements.md - md-viewer" {
				t.Errorf("Title = %q, want %q", cfg.Title, "elements.md - md-viewer")
			}
			if cfg.Canvas != c.canvas {
				t.Errorf("Canvas = %v, want %v", cfg.Canvas, c.canvas)
			}
			if cfg.ExitAfterFirst != c.exit {
				t.Errorf("ExitAfterFirst = %t, want %t", cfg.ExitAfterFirst, c.exit)
			}
			if r.prep.viewer == nil {
				t.Fatal("the viewer is nil")
			}
			if got := r.prep.viewer.theme.Palette.Fg; got != c.fg {
				t.Errorf("the viewer's Fg = %v, want %v", got, c.fg)
			}
		})
	}

	t.Run("base name of a file in a directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "sub")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		path := filepath.Join(dir, "x y.md")
		writeFile(t, path, "# a\n")
		r := runPrepare(t, path)
		if !r.prep.OpenWindow {
			t.Fatalf("OpenWindow is false, want true")
		}
		if got := r.prep.Window(time.Time{}).Title; got != "x y.md - md-viewer" {
			t.Errorf("Title = %q, want %q", got, "x y.md - md-viewer")
		}
	})
}

// TestPrepareTimings: with a scripted clock the load takes three readings and the faces two
// (SPEED-2). OnFirst, set only with --time, takes the last reading and writes the --time line to
// stdout, and its write error is returned (CLI-6).
func TestPrepareTimings(t *testing.T) {
	elements := filepath.Join(testdataDir(t), "elements.md")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	script := func() *scriptedClock {
		return &scriptedClock{readings: []time.Time{
			t0,
			t0.Add(400 * time.Microsecond),
			t0.Add(12740 * time.Microsecond),
			t0.Add(20 * time.Millisecond),
			t0.Add(26100 * time.Microsecond),
			t0.Add(88 * time.Millisecond),
		}}
	}

	t.Run("the line", func(t *testing.T) {
		clock := script()
		var stdout, stderr bytes.Buffer
		prep, code := Prepare([]string{"--time", elements}, &stdout, &stderr, clock)
		if code != ExitOK || !prep.OpenWindow {
			t.Fatalf("Prepare: exit code %d, OpenWindow %t; want 0 and true", code, prep.OpenWindow)
		}
		if clock.next != 5 {
			t.Errorf("Prepare read the clock %d times, want 5 (three for the load, two for the faces)", clock.next)
		}
		cfg := prep.Window(t0)
		if cfg.OnFirst == nil {
			t.Fatal("OnFirst is nil with --time")
		}
		if err := cfg.OnFirst(); err != nil {
			t.Fatalf("OnFirst: %v", err)
		}
		const want = "read_ms=0.4 parse_ms=12.3 fonts_ms=6.1 first_frame_ms=88.0\n"
		if got := stdout.String(); got != want {
			t.Errorf("stdout = %q, want %q", got, want)
		}
		if clock.next != 6 || clock.overruns != 0 {
			t.Errorf("clock read %d times with %d overruns, want 6 and 0", clock.next, clock.overruns)
		}
	})

	t.Run("no --time", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		prep, code := Prepare([]string{elements}, &stdout, &stderr, script())
		if code != ExitOK || !prep.OpenWindow {
			t.Fatalf("Prepare: exit code %d, OpenWindow %t; want 0 and true", code, prep.OpenWindow)
		}
		cfg := prep.Window(t0)
		if cfg.OnFirst != nil {
			if err := cfg.OnFirst(); err != nil {
				t.Fatalf("OnFirst: %v", err)
			}
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want nothing without --time", stdout.String())
		}
	})

	t.Run("failed write of the line", func(t *testing.T) {
		var stderr bytes.Buffer
		prep, code := Prepare([]string{"--time", elements}, failWriter{}, &stderr, script())
		if code != ExitOK || !prep.OpenWindow {
			t.Fatalf("Prepare: exit code %d, OpenWindow %t; want 0 and true", code, prep.OpenWindow)
		}
		cfg := prep.Window(t0)
		if cfg.OnFirst == nil {
			t.Fatal("OnFirst is nil with --time")
		}
		if err := cfg.OnFirst(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Errorf("OnFirst returned %v, want the write error", err)
		}
	})
}

// TestPrepareBindsInvalidate: Bind reaches the images, so a frame that lays out a local image
// makes the window invalidate when the image's pixels are ready (SPEED-5). The frame returns no
// quit and no error. The wait ends on the invalidate, or fails when its context times out, so a
// missing invalidate fails the test rather than hanging it.
func TestPrepareBindsInvalidate(t *testing.T) {
	tmp := t.TempDir()
	var ok bytes.Buffer
	if err := png.Encode(&ok, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode ok.png: %v", err)
	}
	writeFile(t, filepath.Join(tmp, "ok.png"), ok.String())
	doc := filepath.Join(tmp, "doc.md")
	writeFile(t, doc, "![ok](ok.png)\n")

	r := runPrepare(t, doc)
	if !r.prep.OpenWindow {
		t.Fatalf("OpenWindow is false, want true")
	}
	cfg := r.prep.Window(time.Time{})
	if cfg.Bind == nil {
		t.Fatal("Bind is nil")
	}
	if cfg.Frame == nil {
		t.Fatal("Frame is nil")
	}
	invalidated := make(chan struct{}, 4)
	cfg.Bind(func() { invalidated <- struct{}{} })

	var ops op.Ops
	var router input.Router
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(980, 900)),
	}
	quit, err := cfg.Frame(gtx)
	if quit || err != nil {
		t.Fatalf("Frame returned quit %t, err %v; want false, nil", quit, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	select {
	case <-invalidated:
	case <-ctx.Done():
		t.Fatal("the window was not invalidated when the image was ready")
	}
}

// TestPrepareBindReachesViewer: Bind reaches the viewer, so a link whose opener fails and whose
// error line cannot be written invalidates the window at once (LINK-2). The spawned function runs
// in the test's own goroutine, so the count is read without a wait.
func TestPrepareBindReachesViewer(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "plain.md")
	writeFile(t, plain, "# a\n")
	r := runPrepare(t, plain)
	if !r.prep.OpenWindow {
		t.Fatalf("OpenWindow is false, want true")
	}
	v := r.prep.viewer
	v.opener = &fakeOpener{err: errors.New("no opener")}
	v.stderr = failWriter{}
	var spawned []func()
	v.spawn = func(f func()) { spawned = append(spawned, f) }
	invalidations := 0
	r.prep.bind(func() { invalidations++ })

	v.open(doc.Target{Kind: doc.External, Value: "https://e.org/"})
	if len(spawned) != 1 {
		t.Fatalf("spawned %d functions, want 1", len(spawned))
	}
	spawned[0]()
	if invalidations != 1 {
		t.Errorf("invalidations = %d, want 1", invalidations)
	}
	if err := v.takeAsyncErr(); err == nil {
		t.Errorf("the failed write was not kept for the next frame")
	}
}

// TestPrepareFontError: a font error is a preparation error: its line is printed to stderr with
// the program's prefix, the exit code is 1, and no window opens.
func TestPrepareFontError(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "plain.md")
	writeFile(t, plain, "# a\n")
	loadFaces := func() (render.Faces, error) {
		return render.Faces{}, errors.New("fonts broke")
	}
	var stdout, stderr bytes.Buffer
	clock := &stepClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	prep, code := prepareWith([]string{plain}, &stdout, &stderr, clock, loadFaces)
	checkRun(t, prepareRun{prep: prep, code: code, stdout: stdout.String(), stderr: stderr.String()},
		"", "md-viewer: fonts broke\n", 1, false)
}
