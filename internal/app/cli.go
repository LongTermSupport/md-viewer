package app

// Stub: zero-value behaviour, written so the M0 tests compile and fail on their assertions.

type ExitCode int

const (
	ExitOK      ExitCode = 0 // CLI-6
	ExitFailure ExitCode = 1 // CLI-6
	ExitUsage   ExitCode = 2 // CLI-6
)

type Options struct {
	Path                      string
	Dark, Time, Help, Version bool
}

type UsageError struct {
	Msg string
}

func (e *UsageError) Error() string {
	return e.Msg
}

func ParseArgs(args []string) (Options, error) {
	return Options{}, nil
}

const Usage = ""
