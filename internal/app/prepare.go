package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime/debug"
	"time"

	"github.com/LongTermSupport/md-viewer/internal/desktop"
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

// Env is what Prepare reads from outside the process: the environment's variables (SET-2,
// THEME-2) and the colour-scheme query (THEME-2). SystemEnv gives the real ones; a test gives its own.
type Env struct {
	Getenv      func(string) string
	ColorScheme func(context.Context, func(string) string) (desktop.Scheme, error)
}

// SystemEnv is the process's environment and the desktop's colour-scheme query.
func SystemEnv() Env {
	return Env{Getenv: os.Getenv, ColorScheme: desktop.ColorScheme}
}

// Messages printed before the window opens. Each ends with a newline.
const (
	msgReadFailed  = progName + ": cannot read %s: %v\n"                                  // CLI-8
	msgIsDirectory = progName + ": cannot read %s: is a directory\n"                      // CLI-8
	msgTooLarge    = progName + ": %s is too large: %d bytes, the limit is %d\n"          // CLI-8
	msgNotText     = progName + ": %s is not a text file: it contains a NUL byte\n"       // CLI-8
	msgHTMLDropped = progName + ": %d HTML fragment(s) not shown: HTML is not rendered\n" // DOC-9
	msgLoadOther   = progName + ": %v\n"                                                  // DOC-13
	msgSchemeRead  = progName + ": cannot read the desktop colour scheme: %v\n"           // THEME-3
	msgConfigLine  = progName + ": %s:%d: %s\n"                                           // SET-5
	msgConfigFile  = progName + ": %s %s\n"                                               // SET-5
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

// Prepare handles the command line, the configuration file and the document before any window
// opens (SPEED-1). A usage error prints its two lines to stderr (CLI-7) and gives ExitUsage.
// --help and --version print to stdout and give ExitOK, without reading the configuration file.
// A configuration file that cannot be read or is refused prints its line and gives ExitFailure
// (SET-5). A document that cannot be loaded prints its CLI-8 line and gives ExitFailure. A
// document that loads prints DOC-9's line when HTML was dropped, and gives OpenWindow with
// ExitOK. A failed write gives ExitFailure.
func Prepare(args []string, env Env, stdout, stderr io.Writer, c Clock) (Prepared, ExitCode) {
	return prepareWith(args, env, stdout, stderr, c, render.LoadFaces)
}

// prepareWith is Prepare with the font loader given, so that a test can make the load fail.
func prepareWith(args []string, env Env, stdout, stderr io.Writer, c Clock, loadFaces func() (render.Faces, error)) (Prepared, ExitCode) {
	p, code, err := prepare(args, env, stdout, stderr, c, loadFaces)
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
func prepare(args []string, env Env, stdout, stderr io.Writer, c Clock, loadFaces func() (render.Faces, error)) (Prepared, ExitCode, error) {
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
	// The file is read before the document, so a refused file is reported even when FILE is
	// missing (SET-5). Flags win over the file, and the rest is the default (SET-1).
	file, err := loadSettings(env.Getenv)
	if err != nil {
		return Prepared{}, ExitFailure, write(stderr, configMessage(err))
	}
	settings := opts.Settings.Over(file).Resolved()
	d, stats, err := doc.Load(opts.Path, c.Now)
	if err != nil {
		return Prepared{}, ExitFailure, write(stderr, loadMessage(err))
	}
	if d.DroppedHTML > 0 {
		if werr := write(stderr, fmt.Sprintf(msgHTMLDropped, d.DroppedHTML)); werr != nil {
			return Prepared{}, ExitFailure, werr
		}
	}
	// The desktop is asked only for auto (THEME-2). It runs alongside the image probes and the font
	// load, and the deferred close joins it on every path after this point, so it never outlives
	// Prepare.
	var query *schemeQuery
	if settings.Theme == ThemeAuto {
		query = startScheme(env.ColorScheme, env.Getenv)
	}
	defer query.close()
	// The image lines follow DOC-9's line, in document order (STYLE-IMG-3). Their write error is a
	// failed write like any other.
	images, err := render.NewImages(d, stderr)
	if err != nil {
		return Prepared{}, ExitFailure, err
	}
	// The faces and the shaper are timed from two readings, which is what fonts_ms measures (SPEC
	// SPEED-2).
	fontsStart := c.Now()
	faces, err := loadFaces()
	if err != nil {
		return Prepared{}, ExitFailure, err
	}
	sh := render.NewShaper(faces)
	fonts := c.Now().Sub(fontsStart)
	// The desktop's answer is joined here. A failed query prints its line after the image lines,
	// and the window opens with light (THEME-3).
	answer := query.wait()
	if answer.err != nil {
		if werr := write(stderr, fmt.Sprintf(msgSchemeRead, answer.err)); werr != nil {
			return Prepared{}, ExitFailure, werr
		}
	}
	// The palette is chosen here and nowhere else (STYLE-PAL-2).
	pal := paletteFor(settings.Theme, answer.scheme)
	view := render.NewView(d, sh, pal, images)
	return Prepared{
		OpenWindow: true,
		opts:       opts,
		document:   d,
		images:     images,
		palette:    pal,
		viewer:     NewViewer(view, pal, settings.FontSize, SystemOpener(), stderr),
		timings:    Timings{Read: stats.Read, Parse: stats.Parse, Fonts: fonts},
		clock:      c,
		stdout:     stdout,
	}, ExitOK, nil
}

// paletteFor is the palette of a resolved theme (THEME-1). Auto is dark only when the desktop
// prefers dark; no preference is light.
func paletteFor(t Theme, s desktop.Scheme) render.Palette {
	switch t {
	case ThemeDark:
		return render.Dark
	case ThemeAuto:
		if s == desktop.SchemeDark {
			return render.Dark
		}
		return render.Light
	case ThemeUnset, ThemeLight:
		// ThemeUnset never reaches here: Resolved gives every unset theme its default.
		return render.Light
	default:
		return render.Light
	}
}

// schemeAnswer is what the colour-scheme query gave.
type schemeAnswer struct {
	scheme desktop.Scheme
	err    error
}

// schemeQuery is one colour-scheme query, run in its own goroutine and joined once (THEME-2).
type schemeQuery struct {
	ch     chan schemeAnswer // one buffered slot: the goroutine never blocks on the answer
	answer schemeAnswer
	joined bool
}

// startScheme starts the query with the deadline of THEME-3. The query gets its own context, so it
// is bounded whatever the caller does.
func startScheme(query func(context.Context, func(string) string) (desktop.Scheme, error), getenv func(string) string) *schemeQuery {
	q := &schemeQuery{ch: make(chan schemeAnswer, 1)}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), schemeTimeout)
		defer cancel()
		scheme, err := query(ctx, getenv)
		q.ch <- schemeAnswer{scheme: scheme, err: err}
	}()
	return q
}

// wait joins the query and returns its answer, once. A nil query, which a theme other than auto
// has, gives no preference and no error.
func (q *schemeQuery) wait() schemeAnswer {
	if q == nil {
		return schemeAnswer{scheme: desktop.SchemeNoPreference}
	}
	if !q.joined {
		q.answer = <-q.ch
		q.joined = true
	}
	return q.answer
}

// close joins a query that the preparation did not reach, so that its goroutine ends with Prepare.
func (q *schemeQuery) close() {
	q.wait()
}

// configFileError is a configuration file that cannot be read or is refused: its path, and the
// error. The error is a *fs.PathError for a file that cannot be opened, or a *ConfigError for its
// content.
type configFileError struct {
	path string
	err  error
}

// Error is the path and the error.
func (e *configFileError) Error() string {
	return e.path + ": " + e.err.Error()
}

// Unwrap is the error, for errors.As.
func (e *configFileError) Unwrap() error {
	return e.err
}

// loadSettings reads the configuration file, if there is one (SET-2). An absent file, or one whose
// directory is absent, gives no settings and no message (SET-4). The file is opened through
// doc.OpenRegular, so no open can block on a FIFO (CLI-8), and it is read up to one byte past the
// limit, so ParseConfig can refuse a larger file (SET-5).
func loadSettings(getenv func(string) string) (Settings, error) {
	path, ok := ConfigPath(getenv)
	if !ok {
		return Settings{}, nil
	}
	f, _, err := doc.OpenRegular(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Settings{}, nil
		}
		return Settings{}, &configFileError{path: path, err: err}
	}
	data, readErr := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return Settings{}, &configFileError{path: path, err: err}
	}
	s, err := ParseConfig(data)
	if err != nil {
		return Settings{}, &configFileError{path: path, err: err}
	}
	return s, nil
}

// configMessage is the stderr line for a configuration file that is refused (SET-5): the file and
// its line when the error is in a line, the file and its problem when the error is in the whole file,
// and CLI-8's wording when the file cannot be read.
func configMessage(err error) string {
	var fe *configFileError
	if !errors.As(err, &fe) {
		return fmt.Sprintf(msgLoadOther, err)
	}
	var ce *ConfigError
	if errors.As(fe.err, &ce) {
		if ce.Line == 0 {
			return fmt.Sprintf(msgConfigFile, fe.path, ce.Msg)
		}
		return fmt.Sprintf(msgConfigLine, fe.path, ce.Line, ce.Msg)
	}
	var pe *fs.PathError
	if errors.As(fe.err, &pe) {
		return fmt.Sprintf(msgReadFailed, fe.path, pe.Err)
	}
	return fmt.Sprintf(msgReadFailed, fe.path, fe.err)
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
