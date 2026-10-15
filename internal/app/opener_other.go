//go:build !linux

package app

import (
	"context"
	"fmt"
	"runtime"
)

// unsupportedOpener is the link opener of a platform that has none yet. Its Open fails, so a
// click prints LINK-2's line and nothing else happens.
type unsupportedOpener struct{}

// Open always fails, naming the platform.
func (unsupportedOpener) Open(_ context.Context, _ string) error {
	return fmt.Errorf("no link opener on %s", runtime.GOOS)
}

// SystemOpener is the platform's link opener: none yet, off Linux (ARCHITECTURE "Cross-platform
// seams").
func SystemOpener() Opener {
	return unsupportedOpener{}
}
