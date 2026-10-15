package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// helperExitEnv names the variable that tells TestOpenerHelperProcess the code to exit with.
const helperExitEnv = "MDV_OPENER_HELPER_EXIT"

// TestExecOpenerWait checks LINK-2: an opener that exits non-zero gives an error that says so, and
// one that exits 0 gives nil. The opener is this test binary, re-run with the target as its
// -test.run flag, so only TestOpenerHelperProcess runs in it, and no external program is needed.
func TestExecOpenerWait(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{name: "exit 3 is an error", code: "3", wantErr: true},
		{name: "exit 0 is not", code: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(helperExitEnv, tt.code)
			o := execOpener{name: os.Args[0]}
			err := o.Open(t.Context(), "-test.run=^TestOpenerHelperProcess$")
			if !tt.wantErr {
				if err != nil {
					t.Errorf("Open of a program that exits 0 returned %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Open of a program that exits 3 returned nil, want an error")
			}
			if !strings.Contains(err.Error(), "exit status 3") {
				t.Errorf("Open error is %q, want it to say exit status 3", err)
			}
		})
	}
}

// TestOpenerHelperProcess is the program TestExecOpenerWait runs. It exits with the code in
// helperExitEnv. Run as an ordinary test, without that variable, it does nothing.
func TestOpenerHelperProcess(t *testing.T) {
	code, ok := os.LookupEnv(helperExitEnv)
	if !ok {
		return
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		t.Fatalf("%s is %q: %v", helperExitEnv, code, err)
	}
	os.Exit(n)
}

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
