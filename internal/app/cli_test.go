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
	const want = `Usage: md-viewer [--theme light|dark|auto] [--font-size N] [--time] FILE
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
	dark := Settings{Theme: ThemeDark}
	cases := []struct {
		args []string
		want Options
	}{
		{[]string{"a.md"}, Options{Path: "a.md"}},
		{[]string{"--dark", "a.md"}, Options{Path: "a.md", Settings: dark}},
		{[]string{"a.md", "--dark"}, Options{Path: "a.md", Settings: dark}},
		{[]string{"--time", "a.md", "--dark"}, Options{Path: "a.md", Settings: dark, Time: true}},
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
		{[]string{"--dark", "--", "-x"}, Options{Path: "-x", Settings: Settings{Theme: ThemeDark}}, ""},
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
	checkParse(t, []string{"--dark", "--dark", "a.md"}, Options{Path: "a.md", Settings: Settings{Theme: ThemeDark}})
}

// TestParseArgsSettings (CLI-2, CLI-3): --theme and --font-size take the next argument; --dark is
// --theme dark; when theme flags repeat, the last one wins, and the same holds for --font-size.
func TestParseArgsSettings(t *testing.T) {
	cases := []struct {
		args []string
		want Options
	}{
		{[]string{"--theme", "dark", "a.md"}, Options{Path: "a.md", Settings: Settings{Theme: ThemeDark}}},
		{[]string{"a.md", "--font-size", "20"}, Options{Path: "a.md", Settings: Settings{FontSize: 20}}},
		{[]string{"--dark", "--theme", "light", "a.md"}, Options{Path: "a.md", Settings: Settings{Theme: ThemeLight}}},
		{[]string{"--theme", "light", "--dark", "a.md"}, Options{Path: "a.md", Settings: Settings{Theme: ThemeDark}}},
		{[]string{"--font-size", "12", "--font-size", "14", "a.md"}, Options{Path: "a.md", Settings: Settings{FontSize: 14}}},
		{[]string{"--theme", "auto", "a.md"}, Options{Path: "a.md", Settings: Settings{Theme: ThemeAuto}}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.args), func(t *testing.T) {
			checkParse(t, c.args, c.want)
		})
	}
}

// TestParseArgsSettingsErrors (CLI-7): a valued flag with no value, or a value that is not valid,
// is a usage error that names the flag and the value as typed. The first unknown flag, missing
// value or invalid value in argument order is the one reported, and these are reported even with
// --help.
func TestParseArgsSettingsErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"theme with no value", []string{"a.md", "--theme"}, "md-viewer: --theme needs a value"},
		{"font size with no value", []string{"a.md", "--font-size"}, "md-viewer: --font-size needs a value"},
		{"unknown theme", []string{"--theme", "blue", "a.md"}, "md-viewer: invalid --theme value: blue"},
		{"font size out of range", []string{"--font-size", "40", "a.md"}, "md-viewer: invalid --font-size value: 40"},
		{"font size with a sign", []string{"--font-size", "+16", "a.md"}, "md-viewer: invalid --font-size value: +16"},
		{"a flag is taken as a value", []string{"--theme", "--help"}, "md-viewer: invalid --theme value: --help"},
		{"an equals form is unknown", []string{"--theme=dark", "a.md"}, "md-viewer: unknown flag: --theme=dark"},
		{"an unknown flag before a bad value", []string{"--x", "--theme", "blue", "a"}, "md-viewer: unknown flag: --x"},
		{"a bad value before an unknown flag", []string{"--theme", "blue", "--x"}, "md-viewer: invalid --theme value: blue"},
		{"a bad value with --help", []string{"--theme", "blue", "--help"}, "md-viewer: invalid --theme value: blue"},
		{"--dark is not a value", []string{"--font-size", "--dark", "a.md"}, "md-viewer: invalid --font-size value: --dark"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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
