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
		return SchemeNoPreference, failure(ctx, "connect to the session bus", err)
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
		return SchemeNoPreference, failure(ctx, "read the colour scheme", callErr)
	}
	scheme, err = schemeFromReply(reply)
	return scheme, err
}

// failure is the error for a step that did not finish. A deadline stops the step, and godbus then
// reports the closed connection, so when the context is done its error is the cause the caller sees.
func failure(ctx context.Context, step string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w", step, ctxErr)
	}
	return fmt.Errorf("%s: %w", step, err)
}

// statPath reports whether path exists, as an error from os.Stat.
func statPath(path string) error {
	_, err := os.Stat(path)
	return err
}
