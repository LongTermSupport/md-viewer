// Package desktop asks the desktop for its colour scheme (THEME-2). It is the only package that
// imports godbus, and it has no other effect: it never starts a bus and never writes anything.
package desktop

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

// The session bus, and the portal the query asks (THEME-2).
const (
	busEnv         = "DBUS_SESSION_BUS_ADDRESS"
	unixPrefix     = "unix:"
	runtimeEnv     = "XDG_RUNTIME_DIR"
	busSocketName  = "bus"
	portalName     = "org.freedesktop.portal.Desktop"
	portalPath     = "/org/freedesktop/portal/desktop"
	portalIface    = "org.freedesktop.portal.Settings"
	portalMethod   = portalIface + ".ReadOne"
	appearanceNS   = "org.freedesktop.appearance"
	colorSchemeKey = "color-scheme"
	// portalDark and portalLight are the color-scheme values of the portal (THEME-2).
	portalDark  = 1
	portalLight = 2
)

// unavailableNames are the error names of THEME-3: the desktop states no preference, or there is
// no bus or no portal to ask.
var unavailableNames = map[string]bool{
	"org.freedesktop.DBus.Error.ServiceUnknown":   true,
	"org.freedesktop.DBus.Error.NameHasNoOwner":   true,
	"org.freedesktop.DBus.Error.UnknownMethod":    true,
	"org.freedesktop.DBus.Error.UnknownInterface": true,
	"org.freedesktop.DBus.Error.UnknownObject":    true,
	"org.freedesktop.portal.Error.NotFound":       true,
}

// Scheme is the colour scheme the desktop prefers (THEME-2). The values are the portal's own
// numbering for the color-scheme setting: 0 no preference, 1 dark, 2 light.
type Scheme uint8

// The schemes of THEME-2.
const (
	SchemeNoPreference Scheme = iota
	SchemeDark
	SchemeLight
)

// BusAddress is the session bus address md-viewer connects to (THEME-2), and whether there is one.
// stat reports whether a path exists.
func BusAddress(getenv func(string) string, stat func(string) error) (string, bool) {
	if addr := getenv(busEnv); isSingleUnix(addr) {
		return addr, true
	}
	dir := getenv(runtimeEnv)
	if !filepath.IsAbs(dir) {
		return "", false
	}
	path := filepath.Join(dir, busSocketName)
	if stat(path) != nil {
		return "", false
	}
	return "unix:path=" + dbus.EscapeBusAddressValue(path), true
}

// isSingleUnix reports whether addr is one unix: address. A list, or a tcp: address that godbus
// dials without the query's deadline, is not used (THEME-2).
func isSingleUnix(addr string) bool {
	return strings.HasPrefix(addr, unixPrefix) && !strings.Contains(addr, ";")
}

// schemeFromReply reads the portal's reply (THEME-2): a uint32 variant, 1 is dark, 2 is light, and
// any other value is no preference. A reply of any other type is an error that names its signature.
func schemeFromReply(v dbus.Variant) (Scheme, error) {
	sig := v.Signature().String()
	if sig != "u" {
		return SchemeNoPreference, fmt.Errorf("colour scheme reply has type %q, want \"u\"", sig)
	}
	n, ok := v.Value().(uint32)
	if !ok {
		return SchemeNoPreference, fmt.Errorf("colour scheme reply has type %T, want uint32", v.Value())
	}
	switch n {
	case portalDark:
		return SchemeDark, nil
	case portalLight:
		return SchemeLight, nil
	default:
		return SchemeNoPreference, nil
	}
}

// unavailable reports whether err is one of THEME-3's names, for which the desktop states no
// preference and md-viewer uses light without a message. It looks through wrapping.
func unavailable(err error) bool {
	var de dbus.Error
	return errors.As(err, &de) && unavailableNames[de.Name]
}
