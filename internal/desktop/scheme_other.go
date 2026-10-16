//go:build !linux

package desktop

import (
	"context"
)

// ColorScheme gives no preference on platforms without a query yet (THEME-2): the seam for macOS
// and Windows.
func ColorScheme(ctx context.Context, getenv func(string) string) (Scheme, error) {
	return SchemeNoPreference, nil
}
