package app

import (
	"io"
	"time"
)

// Clock gives the current time. Load takes its readings from it, so tests use a fake clock.
type Clock interface{ Now() time.Time }

// SystemClock is the wall clock: the only reader of time.Now outside main.
type SystemClock struct{}

// Now is a stub until the implementation step.
func (SystemClock) Now() time.Time { return time.Time{} }

// Prepared is what Prepare leaves for the window. OpenWindow is set when a file loaded.
type Prepared struct {
	OpenWindow bool
}

// Prepare is a stub until the implementation step.
func Prepare(args []string, stdout, stderr io.Writer, c Clock) (Prepared, ExitCode) {
	return Prepared{}, ExitOK
}
