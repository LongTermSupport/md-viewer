//go:build linux

package app

// SystemOpener is the platform's link opener. Stub: an opener with no program.
func SystemOpener() Opener {
	return execOpener{}
}
