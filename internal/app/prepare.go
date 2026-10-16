package app

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/LongTermSupport/md-viewer/internal/doc"
	"github.com/LongTermSupport/md-viewer/internal/render"
	"github.com/LongTermSupport/md-viewer/internal/window"
)

// Clock gives the current time. Load takes its readings from it, so tests use a fake clock.
type Clock interface{ Now() time.Time }

// SystemClock is the wall clock: the only reader of time.Now outside main.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// Messages printed before the window opens. Each ends with a newline.
const (
	msgReadFailed  = progName + ": cannot read %s: %v\n"                                  // CLI-8
	msgIsDirectory = progName + ": cannot read %s: is a directory\n"                      // CLI-8
	msgTooLarge    = progName + ": %s is too large: %d bytes, the limit is %d\n"          // CLI-8
	msgNotText     = progName + ": %s is not a text file: it contains a NUL byte\n"       // CLI-8
	msgHTMLDropped = progName + ": %d HTML fragment(s) not shown: HTML is not rendered\n" // DOC-9
	msgLoadOther   = progName + ": %v\n"                                                  // DOC-13
)

// Prepared is what Prepare leaves for the window. OpenWindow is set when a file loaded; the
// other fields are what the window needs.
type Prepared struct {
	OpenWindow bool
	opts       Options
	document   *doc.Document
	images     *render.Images
	palette    render.Palette
	viewer     *Viewer
	timings    Timings // Read, Parse and Fonts; FirstFrame is taken in OnFirst
	clock      Clock
	stdout     io.Writer
}

// titleSuffix ends the window's title: NAME - md-viewer (WIN-1).
const titleSuffix = " - " + progName

// Window is the window's configuration (WIN-1, STYLE-PAL-3, SPEED-2). start is the first statement
// of main. OnFirst is set only with --time, and then writes the timings line to stdout after the
// first frame; its error is a frame failure (CLI-6).
func (p Prepared) Window(start time.Time) window.Config {
	cfg := window.Config{
		Title:          p.document.Name + titleSuffix,
		Canvas:         p.palette.Canvas,
		Frame:          p.viewer.Frame,
		Bind:           p.bind,
		ExitAfterFirst: p.opts.Time,
	}
	if p.opts.Time {
		cfg.OnFirst = func() error {
			t := p.timings
			t.FirstFrame = p.clock.Now().Sub(start)
			return write(p.stdout, t.Line()+"\n")
		}
	}
	return cfg
}

// bind passes the window's invalidate to the images and to the viewer (SPEED-5, LINK-2).
func (p Prepared) bind(invalidate func()) {
	p.images.Bind(invalidate)
	p.viewer.Bind(invalidate)
}

// Prepare handles the command line and the file before any window opens (SPEED-1). A usage
// error prints its two lines to stderr (CLI-7) and gives ExitUsage. --help and --version print to
// stdout and give ExitOK. A file that cannot be loaded prints its CLI-8 line and gives
// ExitFailure. A file that loads prints DOC-9's line when HTML was dropped, and gives
// OpenWindow with ExitOK. A failed write gives ExitFailure.
func Prepare(args []string, stdout, stderr io.Writer, c Clock) (Prepared, ExitCode) {
	return prepareWith(args, stdout, stderr, c, render.LoadFaces)
}

// prepareWith is Prepare with the font loader given, so that a test can make the load fail.
func prepareWith(args []string, stdout, stderr io.Writer, c Clock, loadFaces func() (render.Faces, error)) (Prepared, ExitCode) {
	p, code, err := prepare(args, stdout, stderr, c, loadFaces)
	if err != nil {
		// Best effort. A write error cannot be reported on the stream that failed, and the
		// exit code is ExitFailure whether or not this line lands.
		write(stderr, progName+": "+err.Error()+"\n")
		return Prepared{}, ExitFailure
	}
	return p, code
}

// prepare does the work of Prepare. Its error is a failed write, a font error, or an argument error
// that is not a usage error; the code is ignored when the error is not nil. ParseArgs returns only
// a *UsageError, so the other case is a fallback for a later change to ParseArgs.
func prepare(args []string, stdout, stderr io.Writer, c Clock, loadFaces func() (render.Faces, error)) (Prepared, ExitCode, error) {
	opts, err := ParseArgs(args)
	if err != nil {
		var ue *UsageError
		if !errors.As(err, &ue) {
			return Prepared{}, ExitFailure, err
		}
		if werr := write(stderr, ue.Msg+"\n"+UsageHint+"\n"); werr != nil {
			return Prepared{}, ExitFailure, werr
		}
		return Prepared{}, ExitUsage, nil
	}
	switch {
	case opts.Help:
		return Prepared{}, ExitOK, write(stdout, Usage)
	case opts.Version:
		info, ok := debug.ReadBuildInfo()
		return Prepared{}, ExitOK, write(stdout, VersionLine(info, ok)+"\n")
	}
	d, stats, err := doc.Load(opts.Path, c.Now)
	if err != nil {
		return Prepared{}, ExitFailure, write(stderr, loadMessage(err))
	}
	if d.DroppedHTML > 0 {
		if werr := write(stderr, fmt.Sprintf(msgHTMLDropped, d.DroppedHTML)); werr != nil {
			return Prepared{}, ExitFailure, werr
		}
	}
	// The image lines follow DOC-9's line, in document order (STYLE-IMG-3). Their write error is a
	// failed write like any other.
	images, err := render.NewImages(d, stderr)
	if err != nil {
		return Prepared{}, ExitFailure, err
	}
	// The palette is chosen here and nowhere else (STYLE-PAL-2). The faces and the shaper are timed
	// from two readings, which is what fonts_ms measures (SPEC SPEED-2).
	pal := render.Light
	if opts.Settings.Theme == ThemeDark {
		pal = render.Dark
	}
	fontsStart := c.Now()
	faces, err := loadFaces()
	if err != nil {
		return Prepared{}, ExitFailure, err
	}
	sh := render.NewShaper(faces)
	fonts := c.Now().Sub(fontsStart)
	view := render.NewView(d, sh, pal, images)
	return Prepared{
		OpenWindow: true,
		opts:       opts,
		document:   d,
		images:     images,
		palette:    pal,
		viewer:     NewViewer(view, pal, SystemOpener(), stderr),
		timings:    Timings{Read: stats.Read, Parse: stats.Parse, Fonts: fonts},
		clock:      c,
		stdout:     stdout,
	}, ExitOK, nil
}

// loadMessage is the stderr line for a file that cannot be loaded: CLI-8's wording for each typed
// error, and md-viewer: followed by the error for any other.
func loadMessage(err error) string {
	var re *doc.ReadError
	var de *doc.DirectoryError
	var te *doc.TooLargeError
	var be *doc.BinaryError
	switch {
	case errors.As(err, &re):
		return fmt.Sprintf(msgReadFailed, re.Path, re.Err)
	case errors.As(err, &de):
		return fmt.Sprintf(msgIsDirectory, de.Path)
	case errors.As(err, &te):
		return fmt.Sprintf(msgTooLarge, te.Path, te.Size, doc.MaxFileSize)
	case errors.As(err, &be):
		return fmt.Sprintf(msgNotText, be.Path)
	default:
		return fmt.Sprintf(msgLoadOther, err)
	}
}

// write writes s to w and returns the write error, if any.
func write(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
