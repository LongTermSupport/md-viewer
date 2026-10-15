package app

import (
	"errors"
	"fmt"
	"testing"
)

// CLI-11 (ParseArgs reads only its arguments, so main reads no environment or stdin) holds
// by construction and has no test.

func TestUsageText(t *testing.T) {
	// CLI-4, copied literally: the constant must equal this text byte for byte.
	const want = `Usage: md-viewer [--dark] [--time] FILE
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
	if Usage != want {
		t.Errorf("Usage = %q, want %q", Usage, want)
	}
}

func checkParse(t *testing.T, args []string, want Options) {
	t.Helper()
	got, err := ParseArgs(args)
	if err != nil {
		t.Errorf("ParseArgs(%q) error = %v, want nil", args, err)
		return
	}
	if got != want {
		t.Errorf("ParseArgs(%q) = %#v, want %#v", args, got, want)
	}
}

func TestParseArgsFile(t *testing.T) {
	cases := []struct {
		args []string
		want Options
	}{
		{[]string{"a.md"}, Options{Path: "a.md"}},
		{[]string{"--dark", "a.md"}, Options{Path: "a.md", Dark: true}},
		{[]string{"a.md", "--dark"}, Options{Path: "a.md", Dark: true}},
		{[]string{"--time", "a.md", "--dark"}, Options{Path: "a.md", Dark: true, Time: true}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.args), func(t *testing.T) {
			checkParse(t, c.args, c.want)
		})
	}
}

func TestParseArgsDoubleDash(t *testing.T) {
	cases := []struct {
		args []string
		want Options
		msg  string // the usage error message; empty when ParseArgs must succeed
	}{
		{[]string{"--", "--dark"}, Options{Path: "--dark"}, ""},
		{[]string{"-"}, Options{Path: "-"}, ""},
		// After --, an unknown-looking flag is an operand, not an error (CLI-2).
		{[]string{"--", "--nope"}, Options{Path: "--nope"}, ""},
		{[]string{"--dark", "--", "-x"}, Options{Path: "-x", Dark: true}, ""},
		// A second -- after the first is an operand.
		{[]string{"--", "--"}, Options{Path: "--"}, ""},
		// Operands after -- still count toward the FILE count.
		{[]string{"a.md", "--", "--dark"}, Options{}, "md-viewer: expected one FILE, got 2"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.args), func(t *testing.T) {
			if c.msg == "" {
				checkParse(t, c.args, c.want)
				return
			}
			_, err := ParseArgs(c.args)
			var ue *UsageError
			if !errors.As(err, &ue) {
				t.Errorf("ParseArgs(%q) error = %v, want a *UsageError", c.args, err)
				return
			}
			if ue.Msg != c.msg {
				t.Errorf("ParseArgs(%q) message = %q, want %q", c.args, ue.Msg, c.msg)
			}
		})
	}
}

func TestParseArgsHelpVersion(t *testing.T) {
	cases := []struct {
		args []string
		want Options
	}{
		{[]string{"--help"}, Options{Help: true}},
		{[]string{"-h"}, Options{Help: true}},
		{[]string{"--version"}, Options{Version: true}},
		// Both flags are recorded; main checks Help first (CLI-3).
		{[]string{"--help", "--version"}, Options{Help: true, Version: true}},
		// A FILE is not needed, but one given is recorded.
		{[]string{"--help", "a.md"}, Options{Path: "a.md", Help: true}},
		// Extra operands with --version are not a FILE count error (decision 4).
		{[]string{"--version", "a", "b"}, Options{Version: true}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.args), func(t *testing.T) {
			checkParse(t, c.args, c.want)
		})
	}
}

func TestParseArgsRepeated(t *testing.T) {
	checkParse(t, []string{"--dark", "--dark", "a.md"}, Options{Path: "a.md", Dark: true})
}

func TestParseArgsUsageErrors(t *testing.T) {
	cases := []struct {
		args []string
		msg  string
	}{
		{[]string{}, "md-viewer: missing FILE argument"},
		{[]string{"a", "b"}, "md-viewer: expected one FILE, got 2"},
		{[]string{"--nope", "a"}, "md-viewer: unknown flag: --nope"},
		{[]string{"-dark", "a"}, "md-viewer: unknown flag: -dark"},
		{[]string{"--dark=1", "a"}, "md-viewer: unknown flag: --dark=1"},
		{[]string{"--x", "a", "b"}, "md-viewer: unknown flag: --x"},
		// An unknown flag is an error even with --help (decision 4).
		{[]string{"--help", "--nope"}, "md-viewer: unknown flag: --nope"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.args), func(t *testing.T) {
			_, err := ParseArgs(c.args)
			var ue *UsageError
			if !errors.As(err, &ue) {
				t.Errorf("ParseArgs(%q) error = %v, want a *UsageError", c.args, err)
				return
			}
			if ue.Msg != c.msg {
				t.Errorf("ParseArgs(%q) message = %q, want %q", c.args, ue.Msg, c.msg)
			}
		})
	}
}
