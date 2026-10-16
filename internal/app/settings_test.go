package app

import (
	"errors"
	"strings"
	"testing"

	"gioui.org/unit"
)

// TestParseConfig (SET-3): blank and comment lines are ignored, keys are split at the first `=`
// with spaces and tabs trimmed, a CR before the LF is dropped, and a last line need not end in LF.
func TestParseConfig(t *testing.T) {
	rows := []struct {
		name string
		in   string
		want Settings
	}{
		{"empty file", "", Settings{}},
		{"comments and blank lines", "# a comment\n\n   \n\t\n# theme = dark\n", Settings{}},
		{"theme", "theme = dark\n", Settings{Theme: ThemeDark}},
		{"theme without spaces", "theme=light\n", Settings{Theme: ThemeLight}},
		{"font size with a tab and a CR", "\tfont-size =  18 \r\n", Settings{FontSize: 18}},
		{"both keys", "theme = auto\nfont-size = 20\n", Settings{Theme: ThemeAuto, FontSize: 20}},
		{"no final newline", "theme = dark", Settings{Theme: ThemeDark}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got, err := ParseConfig([]byte(r.in))
			if err != nil {
				t.Fatalf("ParseConfig(%q): %v", r.in, err)
			}
			if got != r.want {
				t.Errorf("ParseConfig(%q) = %+v, want %+v", r.in, got, r.want)
			}
		})
	}
}

// TestParseConfigErrors (SET-5): each bad line gives its line number and message, through
// *ConfigError; for a repeated key the message names the first line; the first error in line order
// is the one reported.
func TestParseConfigErrors(t *testing.T) {
	rows := []struct {
		name string
		in   string
		line int
		msg  string
	}{
		{"theme with no =", "theme\n", 1, "expected KEY = VALUE"},
		{"empty key", "= dark\n", 1, "expected KEY = VALUE"},
		{"unknown key", "colour = red\n", 1, "unknown setting: colour"},
		{"theme in the wrong case", "theme = Dark\n", 1, "theme must be light, dark or auto, not Dark"},
		{"theme with no value", "theme =\n", 1, "theme must be light, dark or auto, not "},
		{"font size below the range", "font-size = 9\n", 1, "font-size must be a whole number from 10 to 32, not 9"},
		{"font size above the range", "font-size = 33\n", 1, "font-size must be a whole number from 10 to 32, not 33"},
		{"font size with a point", "font-size = 16.5\n", 1, "font-size must be a whole number from 10 to 32, not 16.5"},
		{"font size with a sign", "font-size = +16\n", 1, "font-size must be a whole number from 10 to 32, not +16"},
		{"font size with an exponent", "font-size = 1e1\n", 1, "font-size must be a whole number from 10 to 32, not 1e1"},
		{"font size with no value", "font-size = \n", 1, "font-size must be a whole number from 10 to 32, not "},
		{"key given twice", "theme = dark\n\ntheme = light\n", 3, "theme is already set on line 1"},
		{"first error in line order", "theme = dark\ncolour = red\nfont-size = 99\n", 2, "unknown setting: colour"},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(r.in))
			var ce *ConfigError
			if !errors.As(err, &ce) {
				t.Fatalf("ParseConfig(%q) error = %v, want a *ConfigError", r.in, err)
			}
			if ce.Line != r.line || ce.Msg != r.msg {
				t.Errorf("ParseConfig(%q) = line %d %q, want line %d %q", r.in, ce.Line, ce.Msg, r.line, r.msg)
			}
		})
	}
}

// TestParseConfigBytes (SET-5): a NUL byte and invalid UTF-8 make the file not a text file, and a
// file over 4096 bytes is too large. A file of exactly 4096 bytes is read. Each error has no line.
func TestParseConfigBytes(t *testing.T) {
	rows := []struct {
		name string
		in   string
		ok   bool
		msg  string
	}{
		{"a NUL byte", "theme = dark\x00\n", false, "is not a text file"},
		{"invalid UTF-8", "theme = \xff\n", false, "is not a text file"},
		{"4097 bytes", strings.Repeat("#", 4097), false, "is too large: 4097 bytes, the limit is 4096"},
		{"4096 bytes", strings.Repeat("#", 4096), true, ""},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(r.in))
			if r.ok {
				if err != nil {
					t.Fatalf("ParseConfig: %v", err)
				}
				return
			}
			var ce *ConfigError
			if !errors.As(err, &ce) {
				t.Fatalf("ParseConfig error = %v, want a *ConfigError", err)
			}
			if ce.Line != 0 || ce.Msg != r.msg {
				t.Errorf("ParseConfig = line %d %q, want line 0 %q", ce.Line, ce.Msg, r.msg)
			}
		})
	}
}

// TestConfigPath (SET-2): an absolute XDG_CONFIG_HOME names the file; otherwise an absolute HOME
// does; a relative value is ignored; with neither there is no file.
func TestConfigPath(t *testing.T) {
	rows := []struct {
		name string
		env  map[string]string
		path string
		ok   bool
	}{
		{"XDG_CONFIG_HOME", map[string]string{"XDG_CONFIG_HOME": "/x"}, "/x/md-viewer/config", true},
		{"relative XDG_CONFIG_HOME falls back to HOME", map[string]string{"XDG_CONFIG_HOME": "rel", "HOME": "/h"},
			"/h/.config/md-viewer/config", true},
		{"neither set", map[string]string{}, "", false},
		{"both relative", map[string]string{"XDG_CONFIG_HOME": "rel", "HOME": "h"}, "", false},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got, ok := ConfigPath(func(k string) string { return r.env[k] })
			if got != r.path || ok != r.ok {
				t.Errorf("ConfigPath = %q, %v; want %q, %v", got, ok, r.path, r.ok)
			}
		})
	}
}

// TestSettingsOverResolved (SET-1): the flags' settings win field by field over the file's, and
// an unset field is the default.
func TestSettingsOverResolved(t *testing.T) {
	flags := Settings{Theme: ThemeDark}
	file := Settings{Theme: ThemeLight, FontSize: 20}
	if got := flags.Over(file); got != (Settings{Theme: ThemeDark, FontSize: 20}) {
		t.Errorf("flags.Over(file) = %+v, want dark at 20", got)
	}
	if got := (Settings{}).Over(file); got != file {
		t.Errorf("unset.Over(file) = %+v, want the file's %+v", got, file)
	}
	if got := (Settings{}).Resolved(); got != (Settings{Theme: ThemeAuto, FontSize: 16}) {
		t.Errorf("unset.Resolved() = %+v, want auto at 16", got)
	}
	if got := flags.Resolved(); got != (Settings{Theme: ThemeDark, FontSize: 16}) {
		t.Errorf("dark.Resolved() = %+v, want dark at 16", got)
	}
}

// TestFontSizeMetric (FONT-2): the size multiplies PxPerSp by font-size over 16, and PxPerDp is
// unchanged. The values are literal, so a wrong constant fails.
func TestFontSizeMetric(t *testing.T) {
	base := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	rows := []struct {
		size FontSize
		want unit.Metric
	}{
		{20, unit.Metric{PxPerDp: 1, PxPerSp: 1.25}},
		{16, unit.Metric{PxPerDp: 1, PxPerSp: 1}},
		{10, unit.Metric{PxPerDp: 1, PxPerSp: 0.625}},
	}
	for _, r := range rows {
		if got := r.size.Metric(base); got != r.want {
			t.Errorf("FontSize(%d).Metric = %+v, want %+v", r.size, got, r.want)
		}
	}
}

// TestFontSizeBounds (FONT-1): a font size is written in decimal digits only, from 10 to 32.
func TestFontSizeBounds(t *testing.T) {
	for _, s := range []string{"10", "32", "16"} {
		if _, ok := ParseFontSize(s); !ok {
			t.Errorf("ParseFontSize(%q) is refused, want accepted", s)
		}
	}
	for _, s := range []string{"9", "33", "016x", "-16", "16.5", "+16", "1e1", "", " 16"} {
		if _, ok := ParseFontSize(s); ok {
			t.Errorf("ParseFontSize(%q) is accepted, want refused", s)
		}
	}
}

// TestParseTheme (THEME-1): the three names, in lower case only.
func TestParseTheme(t *testing.T) {
	rows := map[string]Theme{"light": ThemeLight, "dark": ThemeDark, "auto": ThemeAuto}
	for s, want := range rows {
		if got, ok := ParseTheme(s); !ok || got != want {
			t.Errorf("ParseTheme(%q) = %v, %v; want %v, true", s, got, ok, want)
		}
	}
	for _, s := range []string{"Dark", "blue", "", "system"} {
		if _, ok := ParseTheme(s); ok {
			t.Errorf("ParseTheme(%q) is accepted, want refused", s)
		}
	}
}
