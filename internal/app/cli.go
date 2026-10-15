package app

import (
	"fmt"
	"strings"
)

// ExitCode is a process exit status (CLI-6). Only main passes one to os.Exit.
type ExitCode int

const (
	ExitOK      ExitCode = 0 // CLI-6: success
	ExitFailure ExitCode = 1 // CLI-6: runtime failure
	ExitUsage   ExitCode = 2 // CLI-6: usage error
)

// progName is the program name in every message and the version line (CLI-5, CLI-7).
const progName = "md-viewer"

// Flags and operand markers (CLI-2, CLI-3).
const (
	flagDark      = "--dark"
	flagTime      = "--time"
	flagHelp      = "--help"
	flagHelpShort = "-h"
	flagVersion   = "--version"
	argEndOfFlags = "--"
	argDash       = "-"
)

// Usage error messages (CLI-7). msgExtraFile takes the count of FILE operands.
const (
	msgMissingFile = progName + ": missing FILE argument"
	msgExtraFile   = progName + ": expected one FILE, got %d"
	msgUnknownFlag = progName + ": unknown flag: "
)

// UsageHint is the second line of every usage error (CLI-7).
const UsageHint = "Run 'md-viewer --help' for usage."

// Options is what the command line asked for. Path is the FILE operand, as typed (CLI-1).
type Options struct {
	Path                      string
	Dark, Time, Help, Version bool
}

// UsageError is a CLI-7 usage error. Msg is the first line of the message, without the hint.
type UsageError struct {
	Msg string
}

func (e *UsageError) Error() string {
	return e.Msg
}

// ParseArgs reads the arguments after the program name (CLI-1 to CLI-3, CLI-7).
//
// Precedence: the first unknown flag is always an error. Otherwise --help and --version
// skip the FILE count (Path is set only when exactly one operand is given). Otherwise
// zero operands, or more than one, is an error.
func ParseArgs(args []string) (Options, error) {
	var opts Options
	var operands []string
	unknown := ""
	endOfFlags := false
	for _, arg := range args {
		switch {
		case endOfFlags:
			operands = append(operands, arg)
		case arg == argEndOfFlags:
			endOfFlags = true
		case arg == flagDark:
			opts.Dark = true
		case arg == flagTime:
			opts.Time = true
		case arg == flagHelp, arg == flagHelpShort:
			opts.Help = true
		case arg == flagVersion:
			opts.Version = true
		case arg == argDash:
			operands = append(operands, arg)
		case strings.HasPrefix(arg, "-"):
			if unknown == "" {
				unknown = arg
			}
		default:
			operands = append(operands, arg)
		}
	}
	if unknown != "" {
		return Options{}, &UsageError{Msg: msgUnknownFlag + unknown}
	}
	if opts.Help || opts.Version {
		if len(operands) == 1 {
			opts.Path = operands[0]
		}
		return opts, nil
	}
	switch n := len(operands); n {
	case 0:
		return Options{}, &UsageError{Msg: msgMissingFile}
	case 1:
		opts.Path = operands[0]
		return opts, nil
	default:
		return Options{}, &UsageError{Msg: fmt.Sprintf(msgExtraFile, n)}
	}
}

// Usage is the text of --help (CLI-4), byte for byte. It ends with one newline.
const Usage = `Usage: md-viewer [--dark] [--time] FILE
       md-viewer --help | --version

Shows one Markdown file, rendered, in one window.

  --dark     use the dark palette
  --time     print startup timings after the first frame, then exit
  --help     show this help
  --version  show the version

Scroll: wheel, arrows, Page Up/Down, Space, Home/End.
Zoom: Ctrl and + or -, Ctrl+0 to reset, Ctrl and the wheel.
Quit: q or Esc.
`
