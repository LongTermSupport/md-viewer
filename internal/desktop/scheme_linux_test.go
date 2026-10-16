//go:build linux

package desktop

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// privateBusConfig is a session bus that listens on one socket and lets every client own names.
// It is the test's own bus: nothing here reaches the desktop's bus.
const privateBusConfig = `<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:path=%s</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>
`

// startPrivateBus starts a dbus-daemon in a temporary directory and returns its address. The
// process ends with the test's context. A missing dbus-daemon fails the test: it never skips.
func startPrivateBus(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Fatalf("dbus-daemon is not on PATH; the image and CI install the dbus-daemon package: %v", err)
	}
	dir := t.TempDir()
	conf := filepath.Join(dir, "bus.conf")
	if err := os.WriteFile(conf, []byte(fmt.Sprintf(privateBusConfig, filepath.Join(dir, "bus"))), 0o600); err != nil {
		t.Fatalf("write the bus config: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), bin, "--config-file="+conf, "--nofork", "--print-address=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe for the bus address: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start dbus-daemon: %v", err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("dbus-daemon printed no address: %v", err)
	}
	return strings.TrimSpace(line)
}

// The setting the portal answers (THEME-2). The test pins the strings itself, so a wrong name in the
// code is an error here and not a silent answer.
const (
	wantNamespace = "org.freedesktop.appearance"
	wantKey       = "color-scheme"
)

// fakePortal serves org.freedesktop.portal.Settings.ReadOne with whatever reply the test sets. It
// answers only wantNamespace and wantKey; any other pair is an error.
type fakePortal struct {
	conn  *dbus.Conn
	mu    sync.Mutex
	reply func() (dbus.Variant, error)
	asked []string // "namespace key" of each call, in order
}

// ReadOne is the portal's method. It records the pair it is asked for, then returns the current
// reply of the test when the pair is the colour scheme's.
func (p *fakePortal) ReadOne(namespace, key string) (dbus.Variant, error) {
	p.mu.Lock()
	p.asked = append(p.asked, namespace+" "+key)
	reply := p.reply
	p.mu.Unlock()
	if namespace != wantNamespace || key != wantKey {
		return dbus.Variant{}, dbus.Error{Name: "org.example.UnexpectedSetting"}
	}
	return reply()
}

// setReply makes the next calls answer with reply.
func (p *fakePortal) setReply(reply func() (dbus.Variant, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reply = reply
}

// askedFor is the pairs the portal has been asked for, in order.
func (p *fakePortal) askedFor() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.asked...)
}

// servePortal starts a private bus, and connects a fake portal to it with the portal's object
// exported. The portal's name is not yet owned, so a query finds no portal until claim is called.
func servePortal(t *testing.T) (string, *fakePortal) {
	t.Helper()
	addr := startPrivateBus(t)
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatalf("connect the fake portal to the private bus: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close the fake portal's connection: %v", err)
		}
	})
	portal := &fakePortal{conn: conn, reply: func() (dbus.Variant, error) {
		return dbus.Variant{}, dbus.Error{Name: "org.freedesktop.portal.Error.NotFound"}
	}}
	if err := conn.ExportAll(portal, "/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings"); err != nil {
		t.Fatalf("export the fake portal: %v", err)
	}
	return addr, portal
}

// claim makes the fake portal the owner of the portal's name, so a query on the bus reaches it.
func (p *fakePortal) claim(t *testing.T) {
	t.Helper()
	reply, err := p.conn.RequestName("org.freedesktop.portal.Desktop", dbus.NameFlagDoNotQueue)
	if err != nil {
		t.Fatalf("request the portal's name: %v", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("request the portal's name: reply %v, want primary owner", reply)
	}
}

// TestColorSchemePrivateBus (THEME-2, THEME-3): ColorScheme asks a portal on a private bus, and
// gives the scheme the portal states. Rows run in order: the first has no portal on the bus, so it
// has no owner for the portal's name; the silent row runs last, as its reply never comes. The fake
// environment holds only the private address, so the desktop's bus is never reached.
func TestColorSchemePrivateBus(t *testing.T) {
	addr, portal := servePortal(t)
	env := func(string) string { return "" }
	getenv := func(k string) string {
		if k == "DBUS_SESSION_BUS_ADDRESS" {
			return addr
		}
		return env(k)
	}
	query := func(t *testing.T, timeout time.Duration) (Scheme, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), timeout)
		defer cancel()
		return ColorScheme(ctx, getenv)
	}
	const generous = 5 * time.Second

	t.Run("no portal on the bus gives no preference", func(t *testing.T) {
		got, err := query(t, generous)
		if err != nil || got != SchemeNoPreference {
			t.Errorf("ColorScheme = %d, %v; want no preference and nil", got, err)
		}
	})

	portal.claim(t)

	t.Run("dark (1)", func(t *testing.T) {
		portal.setReply(func() (dbus.Variant, error) { return dbus.MakeVariant(uint32(1)), nil })
		got, err := query(t, generous)
		if err != nil || got != SchemeDark {
			t.Errorf("ColorScheme = %d, %v; want dark and nil", got, err)
		}
	})

	t.Run("light (2)", func(t *testing.T) {
		portal.setReply(func() (dbus.Variant, error) { return dbus.MakeVariant(uint32(2)), nil })
		got, err := query(t, generous)
		if err != nil || got != SchemeLight {
			t.Errorf("ColorScheme = %d, %v; want light and nil", got, err)
		}
	})

	t.Run("key not found gives no preference", func(t *testing.T) {
		portal.setReply(func() (dbus.Variant, error) {
			return dbus.Variant{}, dbus.Error{Name: "org.freedesktop.portal.Error.NotFound"}
		})
		got, err := query(t, generous)
		if err != nil || got != SchemeNoPreference {
			t.Errorf("ColorScheme = %d, %v; want no preference and nil", got, err)
		}
	})

	t.Run("a string reply is an error", func(t *testing.T) {
		portal.setReply(func() (dbus.Variant, error) { return dbus.MakeVariant("1"), nil })
		if _, err := query(t, generous); err == nil {
			t.Errorf("ColorScheme of a string reply gave no error")
		}
	})

	t.Run("an address nobody listens on is an error", func(t *testing.T) {
		missing := "unix:path=" + filepath.Join(t.TempDir(), "nobody")
		ctx, cancel := context.WithTimeout(t.Context(), generous)
		defer cancel()
		_, err := ColorScheme(ctx, func(k string) string {
			if k == "DBUS_SESSION_BUS_ADDRESS" {
				return missing
			}
			return ""
		})
		if err == nil {
			t.Errorf("ColorScheme on a missing socket gave no error")
		}
	})

	t.Run("a silent portal gives the deadline", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		portal.setReply(func() (dbus.Variant, error) {
			<-release
			return dbus.MakeVariant(uint32(1)), nil
		})
		_, err := query(t, 50*time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("ColorScheme error = %v, want one matching context.DeadlineExceeded", err)
		}
	})
}

// TestColorSchemeAsksPortalKey (THEME-2): the query asks the portal once, for the colour scheme of
// the appearance namespace. The fake portal refuses any other pair with an error, so a wrong name
// shows as an error here, and the recorded pairs show what was asked.
func TestColorSchemeAsksPortalKey(t *testing.T) {
	addr, portal := servePortal(t)
	portal.claim(t)
	portal.setReply(func() (dbus.Variant, error) { return dbus.MakeVariant(uint32(1)), nil })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	got, err := ColorScheme(ctx, func(k string) string {
		if k == "DBUS_SESSION_BUS_ADDRESS" {
			return addr
		}
		return ""
	})
	if err != nil || got != SchemeDark {
		t.Errorf("ColorScheme = %d, %v; want dark and nil", got, err)
	}
	want := []string{wantNamespace + " " + wantKey}
	if asked := portal.askedFor(); !slices.Equal(asked, want) {
		t.Errorf("the portal was asked %q, want %q", asked, want)
	}
}
