// Command md-viewer shows one Markdown file, rendered, in one window (SPEC.md).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/LongTermSupport/md-viewer/internal/app"
	"github.com/LongTermSupport/md-viewer/internal/window"
	"github.com/LongTermSupport/md-viewer/internal/window/gioapp"
)

// The stderr lines of a window that fails: a frame failure prints its own error (CLI-6), and a
// window that cannot be opened prints CLI-9's line. Each ends with a newline.
const (
	msgFrameFailed = "md-viewer: %v\n"                       // CLI-6
	msgCannotOpen  = "md-viewer: cannot open a window: %v\n" // CLI-9
)

func main() {
	start := time.Now()
	prep, code := app.Prepare(os.Args[1:], app.SystemEnv(), os.Stdout, os.Stderr, app.SystemClock{})
	if !prep.OpenWindow {
		os.Exit(int(code))
	}
	// The window runs on its own goroutine, because gioapp.Main must hold the main goroutine.
	go func() {
		os.Exit(int(exitCode(gioapp.Run(prep.Window(start)), os.Stderr)))
	}()
	gioapp.Main()
}

// exitCode is the exit code of the window's result, and writes the stderr line for an error: a
// *window.FrameError prints its error (CLI-6), any other error prints CLI-9's line. A failed
// write to stderr still gives app.ExitFailure.
func exitCode(err error, stderr io.Writer) app.ExitCode {
	if err == nil {
		return app.ExitOK
	}
	line := fmt.Sprintf(msgCannotOpen, err)
	var fe *window.FrameError
	if errors.As(err, &fe) {
		line = fmt.Sprintf(msgFrameFailed, fe.Err)
	}
	// A failed write has no stream left to report on, so its error is not reported. The exit code
	// is ExitFailure whether or not the line lands.
	write(stderr, line)
	return app.ExitFailure
}

// write writes s to w and returns the write error, if any.
func write(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
