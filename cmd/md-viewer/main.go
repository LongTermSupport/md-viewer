// Command md-viewer shows one Markdown file, rendered, in one window (SPEC.md).
package main

import (
	"io"
	"os"

	"github.com/LongTermSupport/md-viewer/internal/app"
)

// notImplemented is printed for a file that loads, until M10 opens the window (MILESTONES M0, P13).
const notImplemented = "md-viewer: not implemented yet\n"

func main() {
	prep, code := app.Prepare(os.Args[1:], os.Stdout, os.Stderr, app.SystemClock{})
	if !prep.OpenWindow {
		os.Exit(int(code))
	}
	// A failed write changes nothing: the exit code is ExitFailure either way.
	if werr := write(os.Stderr, notImplemented); werr != nil {
		os.Exit(int(app.ExitFailure))
	}
	os.Exit(int(app.ExitFailure))
}

// write writes s to w and returns the write error, if any.
func write(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
