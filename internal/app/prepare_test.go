package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stepClock is a fake Clock that advances 1 ms per reading, so no test reads the wall clock.
type stepClock struct{ t time.Time }

func (c *stepClock) Now() time.Time {
	now := c.t
	c.t = c.t.Add(time.Millisecond)
	return now
}

// prepareRun is the outcome of one Prepare call.
type prepareRun struct {
	prep           Prepared
	code           ExitCode
	stdout, stderr string
}

// runPrepare calls Prepare with args and a fake clock, and captures both streams.
func runPrepare(t *testing.T, args ...string) prepareRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	clock := &stepClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	prep, code := Prepare(args, &stdout, &stderr, clock)
	return prepareRun{prep: prep, code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// checkRun asserts every observable result of a Prepare call. The literals at each call site
// are the expectations, so the test never reuses the constants it checks.
func checkRun(t *testing.T, r prepareRun, stdout, stderr string, code ExitCode, open bool) {
	t.Helper()
	if r.stdout != stdout {
		t.Errorf("stdout = %q, want %q", r.stdout, stdout)
	}
	if r.stderr != stderr {
		t.Errorf("stderr = %q, want %q", r.stderr, stderr)
	}
	if r.code != code {
		t.Errorf("exit code = %d, want %d", r.code, code)
	}
	if r.prep.OpenWindow != open {
		t.Errorf("OpenWindow = %v, want %v", r.prep.OpenWindow, open)
	}
}

// writeFile writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestPrepareArgs covers the output that moved from main into Prepare: help, version and
// usage errors (CLI-3, CLI-4, CLI-5, CLI-6, CLI-7).
func TestPrepareArgs(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		checkRun(t, runPrepare(t, "--help"), Usage, "", 0, false)
	})

	t.Run("version", func(t *testing.T) {
		r := runPrepare(t, "--version")
		if !strings.HasPrefix(r.stdout, "md-viewer ") {
			t.Errorf("stdout = %q, want prefix %q", r.stdout, "md-viewer ")
		}
		if strings.Count(r.stdout, "\n") != 1 || !strings.HasSuffix(r.stdout, "\n") {
			t.Errorf("stdout = %q, want one line ending in a newline", r.stdout)
		}
		if r.stderr != "" {
			t.Errorf("stderr = %q, want empty", r.stderr)
		}
		if r.code != 0 {
			t.Errorf("exit code = %d, want 0", r.code)
		}
		if r.prep.OpenWindow {
			t.Errorf("OpenWindow = true, want false")
		}
	})

	t.Run("no arguments", func(t *testing.T) {
		const stderr = "md-viewer: missing FILE argument\nRun 'md-viewer --help' for usage.\n"
		checkRun(t, runPrepare(t), "", stderr, 2, false)
	})

	// FILE is not read when --help is given, so a missing file is not an error.
	t.Run("help with a missing FILE", func(t *testing.T) {
		checkRun(t, runPrepare(t, "--help", "/no/such/file.md"), Usage, "", 0, false)
	})
}

// TestPrepareFileErrors checks CLI-8's four messages, byte for byte, with exit 1.
func TestPrepareFileErrors(t *testing.T) {
	tmp := t.TempDir()

	big := filepath.Join(tmp, "big.md")
	writeFile(t, big, "")
	if err := os.Truncate(big, 67108864+1); err != nil {
		t.Fatalf("truncate %s: %v", big, err)
	}
	nul := filepath.Join(tmp, "nul.md")
	writeFile(t, nul, "a\x00b")

	cases := []struct {
		name, path, stderr string
	}{
		{
			name:   "missing file",
			path:   filepath.Join(tmp, "nosuch.md"),
			stderr: "md-viewer: cannot read " + filepath.Join(tmp, "nosuch.md") + ": no such file or directory\n",
		},
		{
			// The path is as typed: a relative FILE is reported relative.
			name:   "relative missing file",
			path:   "./nosuch-m2.md",
			stderr: "md-viewer: cannot read ./nosuch-m2.md: no such file or directory\n",
		},
		{
			name:   "directory",
			path:   tmp,
			stderr: "md-viewer: cannot read " + tmp + ": is a directory\n",
		},
		{
			name:   "too large",
			path:   big,
			stderr: "md-viewer: " + big + " is too large: 67108865 bytes, the limit is 67108864\n",
		},
		{
			name:   "NUL byte",
			path:   nul,
			stderr: "md-viewer: " + nul + " is not a text file: it contains a NUL byte\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkRun(t, runPrepare(t, c.path), "", c.stderr, 1, false)
		})
	}
}

// TestPrepareHTMLLine checks DOC-9's line: one for a file with HTML, none for a file without.
// A loaded file opens the window (OpenWindow) with exit 0.
func TestPrepareHTMLLine(t *testing.T) {
	tmp := t.TempDir()

	html := filepath.Join(tmp, "html.md")
	writeFile(t, html, "<div>x</div>\n\na <b>c</b>\n")
	checkRun(t, runPrepare(t, html), "",
		"md-viewer: 3 HTML fragment(s) not shown: HTML is not rendered\n", 0, true)

	plain := filepath.Join(tmp, "plain.md")
	writeFile(t, plain, "# a\n")
	checkRun(t, runPrepare(t, plain), "", "", 0, true)
}
