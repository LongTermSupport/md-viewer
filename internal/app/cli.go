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
	flagTheme     = "--theme"
	flagFontSize  = "--font-size"
	flagTime      = "--time"
	flagHelp      = "--help"
	flagHelpShort = "-h"
	flagVersion   = "--version"
	argEndOfFlags = "--"
	argDash       = "-"
)

// Usage error messages (CLI-7). msgExtraFile takes the count of FILE operands; msgNeedsValue takes
// the flag; msgInvalidValue takes the flag and the value as typed.
const (
	msgMissingFile  = progName + ": missing FILE argument"
	msgExtraFile    = progName + ": expected one FILE, got %d"
	msgUnknownFlag  = progName + ": unknown flag: "
	msgNeedsValue   = progName + ": %s needs a value"
	msgInvalidValue = progName + ": invalid %s value: %s"
)

// UsageHint is the second line of every usage error (CLI-7).
const UsageHint = "Run '" + progName + " --help' for usage."

// Options is what the command line asked for. Path is the FILE operand, as typed (CLI-1).
// Settings holds the theme and font size the flags gave (SET-1); an unset field is not given.
type Options struct {
	Path     string
	Settings Settings
	Time     bool
	Help     bool
	Version  bool
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
// Precedence: the first unknown flag, missing value or invalid value, in argument order, is always
// an error. Otherwise --help and --version skip the FILE count (Path is set only when exactly one
// operand is given). Otherwise zero operands, or more than one, is an error. --theme and
// --font-size take the next argument as their value, whatever it is (CLI-3).
func ParseArgs(args []string) (Options, error) {
	var opts Options
	var operands []string
	var problem error
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case endOfFlags:
			operands = append(operands, arg)
		case arg == argEndOfFlags:
			endOfFlags = true
		case arg == flagDark:
			opts.Settings.Theme = ThemeDark
		case arg == flagTheme:
			if i+1 >= len(args) {
				problem = firstProblem(problem, &UsageError{Msg: fmt.Sprintf(msgNeedsValue, arg)})
				break
			}
			i++
			t, ok := ParseTheme(args[i])
			if !ok {
				problem = firstProblem(problem, &UsageError{Msg: fmt.Sprintf(msgInvalidValue, arg, args[i])})
				break
			}
			opts.Settings.Theme = t
		case arg == flagFontSize:
			if i+1 >= len(args) {
				problem = firstProblem(problem, &UsageError{Msg: fmt.Sprintf(msgNeedsValue, arg)})
				break
			}
			i++
			f, ok := ParseFontSize(args[i])
			if !ok {
				problem = firstProblem(problem, &UsageError{Msg: fmt.Sprintf(msgInvalidValue, arg, args[i])})
				break
			}
			opts.Settings.FontSize = f
		case arg == flagTime:
			opts.Time = true
		case arg == flagHelp, arg == flagHelpShort:
			opts.Help = true
		case arg == flagVersion:
			opts.Version = true
		case arg == argDash:
			operands = append(operands, arg)
		case strings.HasPrefix(arg, argDash):
			problem = firstProblem(problem, &UsageError{Msg: msgUnknownFlag + arg})
		default:
			operands = append(operands, arg)
		}
	}
	if problem != nil {
		return Options{}, problem
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

// firstProblem keeps the earlier of two usage problems. ParseArgs reports the first in argument
// order (CLI-7), so a later one never replaces an earlier one.
func firstProblem(first, next error) error {
	if first != nil {
		return first
	}
	return next
}

// Usage is the text of --help (CLI-4), byte for byte. It ends with one newline.
const Usage = `Usage: md-viewer [--theme light|dark|auto] [--font-size N] [--time] FILE
       md-viewer --help | --version

Shows one Markdown file, rendered, in one window.

  --theme MODE    light, dark, or auto to follow the desktop (default auto)
  --dark          the same as --theme dark
  --font-size N   body text size, 10 to 32 (default 16)
  --time          print startup timings after the first frame, then exit
  --help          show this help
  --version       show the version

Settings file: $XDG_CONFIG_HOME/md-viewer/config (theme = ..., font-size = ...).
Scroll: wheel, arrows, Page Up/Down, Space, Home/End.
Zoom: Ctrl and + or -, Ctrl+0 to reset, Ctrl and the wheel.
Quit: q or Esc.
`
