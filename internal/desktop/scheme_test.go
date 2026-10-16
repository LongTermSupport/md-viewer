package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// mapEnv is a getenv over a map: the tests never read the process environment.
func mapEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// statOnly reports a path as existing only when it is want; no other path is touched.
func statOnly(want string) func(string) error {
	return func(p string) error {
		if p == want {
			return nil
		}
		return fs.ErrNotExist
	}
}

// TestBusAddress (THEME-2): the environment's session bus address is used as it is when it is set
// and not autolaunch; otherwise the runtime directory's bus socket when XDG_RUNTIME_DIR is absolute
// and that path exists; otherwise there is no bus.
func TestBusAddress(t *testing.T) {
	const runDir = "/run/with space"
	busPath := runDir + "/bus"
	escaped := "unix:path=" + dbus.EscapeBusAddressValue(busPath)
	rows := []struct {
		name  string
		env   map[string]string
		stat  func(string) error
		want  string
		found bool
	}{
		{"environment address is used as is", map[string]string{"DBUS_SESSION_BUS_ADDRESS": "unix:path=/tmp/s"},
			statOnly(busPath), "unix:path=/tmp/s", true},
		{"autolaunch counts as no address", map[string]string{"DBUS_SESSION_BUS_ADDRESS": "autolaunch:"},
			statOnly(""), "", false},
		{"autolaunch falls back to the runtime bus", map[string]string{"DBUS_SESSION_BUS_ADDRESS": "autolaunch:", "XDG_RUNTIME_DIR": runDir},
			statOnly(busPath), escaped, true},
		{"empty address, runtime bus escaped", map[string]string{"XDG_RUNTIME_DIR": runDir},
			statOnly(busPath), escaped, true},
		{"no bus file gives none", map[string]string{"XDG_RUNTIME_DIR": runDir},
			statOnly(""), "", false},
		{"a relative runtime directory gives none", map[string]string{"XDG_RUNTIME_DIR": "run"},
			statOnly("run/bus"), "", false},
		{"nothing set gives none", map[string]string{}, statOnly(busPath), "", false},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got, ok := BusAddress(mapEnv(r.env), r.stat)
			if got != r.want || ok != r.found {
				t.Errorf("BusAddress = %q, %v; want %q, %v", got, ok, r.want, r.found)
			}
		})
	}
}

// TestSchemeFromReply (THEME-2): a uint32 reply 1 is dark, 0, 2 and any other value are no
// preference or light, and a reply of another type is an error that names its signature.
func TestSchemeFromReply(t *testing.T) {
	rows := []struct {
		value uint32
		want  Scheme
	}{
		{0, SchemeNoPreference},
		{1, SchemeDark},
		{2, SchemeLight},
		{7, SchemeNoPreference},
	}
	for _, r := range rows {
		got, err := schemeFromReply(dbus.MakeVariant(r.value))
		if err != nil {
			t.Errorf("schemeFromReply(%d): error %v", r.value, err)
		}
		if got != r.want {
			t.Errorf("schemeFromReply(%d) = %d, want %d", r.value, got, r.want)
		}
	}
	_, err := schemeFromReply(dbus.MakeVariant("1"))
	if err == nil {
		t.Fatalf("schemeFromReply of a string gave no error")
	}
	if !strings.Contains(err.Error(), `"s"`) {
		t.Errorf("error %q does not name the signature s", err)
	}
}

// TestUnavailable (THEME-3): the four names of THEME-3 are unavailable, wrapped or not; a failure
// with another name, a deadline and a plain error are not.
func TestUnavailable(t *testing.T) {
	const prefix = "org.freedesktop.DBus.Error."
	for _, name := range []string{
		prefix + "ServiceUnknown",
		prefix + "NameHasNoOwner",
		prefix + "UnknownMethod",
		prefix + "UnknownInterface",
		prefix + "UnknownObject",
		"org.freedesktop.portal.Error.NotFound",
	} {
		if !unavailable(dbus.Error{Name: name}) {
			t.Errorf("%s is not unavailable", name)
		}
		if !unavailable(fmt.Errorf("query: %w", dbus.Error{Name: name})) {
			t.Errorf("wrapped %s is not unavailable", name)
		}
	}
	for _, err := range []error{
		dbus.Error{Name: prefix + "Failed"},
		context.DeadlineExceeded,
		errors.New("connection refused"),
	} {
		if unavailable(err) {
			t.Errorf("%v is unavailable, want a failure", err)
		}
	}
}
