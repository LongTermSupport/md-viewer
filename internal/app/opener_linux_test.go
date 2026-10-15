//go:build linux

package app

import (
	"slices"
	"testing"
)

// TestSystemOpenerCommand checks LINK-1 and LINK-2 on Linux: the opener is xdg-open, and the
// target is one argument of the command, with no shell in between. The command is not started,
// so the test does not need xdg-open.
func TestSystemOpenerCommand(t *testing.T) {
	o, ok := SystemOpener().(execOpener)
	if !ok {
		t.Fatalf("SystemOpener is %T, want execOpener", SystemOpener())
	}
	if o.name != "xdg-open" {
		t.Errorf("opener program is %q, want %q", o.name, "xdg-open")
	}
	cmd := o.command(t.Context(), "https://e.org/a b")
	want := []string{"xdg-open", "https://e.org/a b"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("command arguments are %q, want %q", cmd.Args, want)
	}
}
