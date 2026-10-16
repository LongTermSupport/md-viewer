package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

// ColorScheme asks the session bus's portal for the colour scheme (THEME-2). It gives no
// preference when there is no bus, or when the portal is unavailable (THEME-3). Any other failure is
// an error, which the caller reports and treats as light. The connection is made with
// dbus.Connect on the address BusAddress gives, never with the helpers that start a bus or read
// the process environment, and it is closed before the call returns.
func ColorScheme(ctx context.Context, getenv func(string) string) (scheme Scheme, err error) {
	addr, ok := BusAddress(getenv, statPath)
	if !ok {
		return SchemeNoPreference, nil
	}
	conn, err := dbus.Connect(addr, dbus.WithContext(ctx))
	if err != nil {
		return SchemeNoPreference, fmt.Errorf("connect to the session bus: %w", err)
	}
	defer func() {
		err = errors.Join(err, conn.Close())
	}()
	var reply dbus.Variant
	call := conn.Object(portalName, portalPath).CallWithContext(ctx, portalMethod, 0, appearanceNS, colorSchemeKey)
	if callErr := call.Store(&reply); callErr != nil {
		if unavailable(callErr) {
			return SchemeNoPreference, nil
		}
		return SchemeNoPreference, fmt.Errorf("read the colour scheme: %w", callErr)
	}
	scheme, err = schemeFromReply(reply)
	return scheme, err
}

// statPath reports whether path exists, as an error from os.Stat.
func statPath(path string) error {
	_, err := os.Stat(path)
	return err
}
