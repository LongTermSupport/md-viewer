package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/md-viewer/internal/app"
)

// runBinary runs bin with args and returns its stdout, stderr and exit code.
func runBinary(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running %s %q: %v", bin, args, err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func TestMainUsageExit(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "md-viewer")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Exit codes are literals (CLI-6), so the test does not reuse the constants it checks.
	t.Run("help", func(t *testing.T) {
		stdout, stderr, code := runBinary(t, bin, "--help")
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		// The prefix check stops an empty app.Usage from passing the equality check below.
		if !strings.HasPrefix(stdout, "Usage: md-viewer [--dark] [--time] FILE\n") {
			t.Errorf("stdout = %q, want the CLI-4 usage text", stdout)
		}
		if stdout != app.Usage {
			t.Errorf("stdout = %q, want %q", stdout, app.Usage)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("no arguments", func(t *testing.T) {
		stdout, stderr, code := runBinary(t, bin)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		const want = "md-viewer: missing FILE argument\nRun 'md-viewer --help' for usage.\n"
		if stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		stdout, stderr, code := runBinary(t, bin, "--nope", "a.md")
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		const want = "md-viewer: unknown flag: --nope\nRun 'md-viewer --help' for usage.\n"
		if stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	})

	// CLI-8: a FILE that cannot be read is reported with its path as typed.
	t.Run("missing FILE", func(t *testing.T) {
		stdout, stderr, code := runBinary(t, bin, "nosuchfile.md")
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		const want = "md-viewer: cannot read nosuchfile.md: no such file or directory\n"
		if stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	})

	// A file that loads still ends with the M2 line and exit 1; M10 removes it (P13).
	// elements.md has four HTML fragments, so DOC-9's line comes first.
	t.Run("loaded FILE is not implemented yet", func(t *testing.T) {
		stdout, stderr, code := runBinary(t, bin, "../../testdata/elements.md")
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		const want = "md-viewer: 4 HTML fragment(s) not shown: HTML is not rendered\n" +
			"md-viewer: not implemented yet\n"
		if stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	})

	t.Run("version", func(t *testing.T) {
		stdout, stderr, code := runBinary(t, bin, "--version")
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.HasPrefix(stdout, "md-viewer ") {
			t.Errorf("stdout = %q, want prefix %q", stdout, "md-viewer ")
		}
		if strings.Count(stdout, "\n") != 1 || !strings.HasSuffix(stdout, "\n") {
			t.Errorf("stdout = %q, want one line ending in a newline", stdout)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	// Help wins over version (CLI-3): the usage text, not the version line.
	for _, args := range [][]string{{"--version", "--help"}, {"-h"}} {
		t.Run("help wins "+strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := runBinary(t, bin, args...)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if stdout != app.Usage {
				t.Errorf("stdout = %q, want %q", stdout, app.Usage)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}
