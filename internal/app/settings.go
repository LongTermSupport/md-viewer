package app

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/render"
)

// Theme is the colour scheme setting (THEME-1). ThemeUnset means the setting was not given.
type Theme uint8

// The themes. ThemeUnset is the zero value, so a setting that was not given is not a theme.
const (
	ThemeUnset Theme = iota
	ThemeAuto
	ThemeLight
	ThemeDark
)

// FontSize is the body text size in whole sp (FONT-1). Zero means the setting was not given.
type FontSize uint8

// The settings' bounds and defaults. Each is one named value, with the expectation it comes from.
const (
	MinFontSize     FontSize = 10                        // FONT-1
	MaxFontSize     FontSize = 32                        // FONT-1
	DefaultFontSize          = FontSize(render.BodySize) // FONT-1: STYLE-TEXT-1's 16 sp
	DefaultTheme             = ThemeAuto                 // THEME-1
	maxConfigSize            = 4096                      // SET-5
	schemeTimeout            = 100 * time.Millisecond    // THEME-3
)

// Settings is what the command line or the configuration file gives (SET-1). An unset field is
// ThemeUnset or 0.
type Settings struct {
	Theme    Theme
	FontSize FontSize
}

// ConfigError is a configuration file's error (SET-5). Line is the 1-based line, or 0 for an error
// of the whole file. Msg is the text after the file's name and line.
type ConfigError struct {
	Line int
	Msg  string
}

// Error is the message without the file's name.
func (e *ConfigError) Error() string {
	if e.Line == 0 {
		return e.Msg
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// Messages of the configuration file (SET-5). Each is completed with the file's name, and with the
// line where there is one.
const (
	msgCfgNotText     = "is not a text file"
	msgCfgTooLarge    = "is too large: %d bytes, the limit is %d"
	msgCfgSyntax      = "expected KEY = VALUE"
	msgCfgUnknownKey  = "unknown setting: %s"
	msgCfgDuplicate   = "%s is already set on line %d"
	msgCfgBadTheme    = "theme must be light, dark or auto, not %s"
	msgCfgBadFontSize = "font-size must be a whole number from %d to %d, not %s"
)

// The keys of the configuration file (SET-3).
const (
	keyTheme    = "theme"
	keyFontSize = "font-size"
)

// ParseTheme reads a theme name, light, dark or auto (THEME-1), as the flags and the file both do.
func ParseTheme(s string) (Theme, bool) {
	switch s {
	case "light":
		return ThemeLight, true
	case "dark":
		return ThemeDark, true
	case "auto":
		return ThemeAuto, true
	default:
		return ThemeUnset, false
	}
}

// ParseFontSize reads a font size in decimal digits, from MinFontSize to MaxFontSize (FONT-1).
// A sign, a point, an exponent or a space is refused.
func ParseFontSize(s string) (FontSize, bool) {
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil || n < uint64(MinFontSize) || n > uint64(MaxFontSize) {
		return 0, false
	}
	return FontSize(n), true
}

// ParseConfig reads a configuration file's bytes (SET-3, SET-5). The size and the text checks come
// first. Then the lines are read in order, and the first error is returned as a *ConfigError.
func ParseConfig(data []byte) (Settings, error) {
	if len(data) > maxConfigSize {
		return Settings{}, &ConfigError{Msg: fmt.Sprintf(msgCfgTooLarge, len(data), maxConfigSize)}
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return Settings{}, &ConfigError{Msg: msgCfgNotText}
	}
	var s Settings
	setOn := map[string]int{}
	for i, raw := range strings.Split(string(data), "\n") {
		n := i + 1
		line := strings.TrimSuffix(raw, "\r")
		if strings.TrimLeft(line, " \t") == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.Trim(key, " \t")
		value = strings.Trim(value, " \t")
		if !ok || key == "" {
			return Settings{}, &ConfigError{Line: n, Msg: msgCfgSyntax}
		}
		if key != keyTheme && key != keyFontSize {
			return Settings{}, &ConfigError{Line: n, Msg: fmt.Sprintf(msgCfgUnknownKey, key)}
		}
		if prev, dup := setOn[key]; dup {
			return Settings{}, &ConfigError{Line: n, Msg: fmt.Sprintf(msgCfgDuplicate, key, prev)}
		}
		setOn[key] = n
		switch key {
		case keyTheme:
			t, ok := ParseTheme(value)
			if !ok {
				return Settings{}, &ConfigError{Line: n, Msg: fmt.Sprintf(msgCfgBadTheme, value)}
			}
			s.Theme = t
		case keyFontSize:
			f, ok := ParseFontSize(value)
			if !ok {
				return Settings{}, &ConfigError{Line: n, Msg: fmt.Sprintf(msgCfgBadFontSize, MinFontSize, MaxFontSize, value)}
			}
			s.FontSize = f
		}
	}
	return s, nil
}

// ConfigPath is the configuration file's path, from the environment (SET-2), and whether there is
// one. An absolute XDG_CONFIG_HOME comes first, then an absolute HOME; a relative value is ignored.
func ConfigPath(getenv func(string) string) (string, bool) {
	if dir := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, progName, "config"), true
	}
	if home := getenv("HOME"); filepath.IsAbs(home) {
		return filepath.Join(home, ".config", progName, "config"), true
	}
	return "", false
}

// Over is s with the unset fields taken from lower (SET-1).
func (s Settings) Over(lower Settings) Settings {
	if s.Theme == ThemeUnset {
		s.Theme = lower.Theme
	}
	if s.FontSize == 0 {
		s.FontSize = lower.FontSize
	}
	return s
}

// Resolved is s with each unset field at its default (SET-1).
func (s Settings) Resolved() Settings {
	if s.Theme == ThemeUnset {
		s.Theme = DefaultTheme
	}
	if s.FontSize == 0 {
		s.FontSize = DefaultFontSize
	}
	return s
}

// Metric scales the text of m by the font size (FONT-2): PxPerSp by the size over 16. PxPerDp is
// unchanged. The size is resolved before it is used (Resolved).
func (f FontSize) Metric(m unit.Metric) unit.Metric {
	m.PxPerSp *= float32(f) / float32(DefaultFontSize)
	return m
}
