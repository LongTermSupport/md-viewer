package app

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

// TestExecOpenerStartError checks that an opener program that does not exist gives an error
// that matches fs.ErrNotExist (LINK-2: the opener cannot start). The program is a path in a
// temporary directory, so no xdg-open is needed.
func TestExecOpenerStartError(t *testing.T) {
	o := execOpener{name: filepath.Join(t.TempDir(), "missing")}
	err := o.Open(t.Context(), "x")
	if err == nil {
		t.Fatal("Open of a missing program returned nil, want an error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open error %v does not match fs.ErrNotExist", err)
	}
}
