//go:build linux

package app

// openerName is the program that opens a link on Linux (LINK-1).
const openerName = "xdg-open"

// SystemOpener is the platform's link opener.
func SystemOpener() Opener {
	return execOpener{name: openerName}
}
