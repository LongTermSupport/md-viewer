// Command md-viewer shows one Markdown file, rendered, in one window (SPEC.md).
package main

import (
	"errors"
	"io"
	"os"
	"runtime/debug"

	"github.com/LongTermSupport/md-viewer/internal/app"
)

// notImplemented is printed for a valid FILE until M2 and M10 replace it (MILESTONES M0).
const notImplemented = "md-viewer: not implemented yet\n"

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		// Best effort: if stderr is the writer that failed, the exit code still says so.
		if werr := write(os.Stderr, "md-viewer: "+err.Error()+"\n"); werr != nil {
			code = app.ExitFailure
		}
	}
	os.Exit(int(code))
}

// run handles the arguments and returns the exit code (CLI-6). A non-nil error is a failure
// to write output, or an argument error that is not a usage error; main reports it.
func run(args []string, stdout, stderr io.Writer) (app.ExitCode, error) {
	opts, err := app.ParseArgs(args)
	if err != nil {
		var ue *app.UsageError
		if !errors.As(err, &ue) {
			return app.ExitFailure, err
		}
		if werr := write(stderr, ue.Msg+"\n"+app.UsageHint+"\n"); werr != nil {
			return app.ExitFailure, werr
		}
		return app.ExitUsage, nil
	}
	switch {
	case opts.Help:
		if werr := write(stdout, app.Usage); werr != nil {
			return app.ExitFailure, werr
		}
		return app.ExitOK, nil
	case opts.Version:
		info, ok := debug.ReadBuildInfo()
		if werr := write(stdout, app.VersionLine(info, ok)+"\n"); werr != nil {
			return app.ExitFailure, werr
		}
		return app.ExitOK, nil
	default:
		if werr := write(stderr, notImplemented); werr != nil {
			return app.ExitFailure, werr
		}
		return app.ExitFailure, nil
	}
}

// write writes s to w and returns the write error, if any.
func write(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
