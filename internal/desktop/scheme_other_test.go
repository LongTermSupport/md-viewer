//go:build !linux

package desktop

import (
	"context"
	"testing"
)

// TestColorSchemeOther (THEME-2, the seam): where no query exists yet, ColorScheme gives no
// preference and no error, so auto is light. It is built only off Linux, and the cross-platform
// vet covers it.
func TestColorSchemeOther(t *testing.T) {
	got, err := ColorScheme(context.Background(), func(string) string { return "" })
	if err != nil || got != SchemeNoPreference {
		t.Errorf("ColorScheme = %d, %v; want no preference and nil", got, err)
	}
}
