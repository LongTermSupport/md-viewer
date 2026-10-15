//go:build !linux

package app

import (
	"runtime"
	"strings"
	"testing"
)

// TestSystemOpenerOther checks that a platform without an opener yet says so, naming the OS
// (ARCHITECTURE "Cross-platform seams"). It runs only off Linux; on Linux it is not compiled.
func TestSystemOpenerOther(t *testing.T) {
	err := SystemOpener().Open(t.Context(), "https://e.org/")
	if err == nil {
		t.Fatal("Open returned nil on a platform with no opener, want an error")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("Open error %q does not name %s", err, runtime.GOOS)
	}
}
