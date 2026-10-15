package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LongTermSupport/md-viewer/internal/app"
	"github.com/LongTermSupport/md-viewer/internal/window"
)

// elementsFile is the smoke run's document, relative to this package.
const elementsFile = "../../testdata/elements.md"

// preWindowLines are the lines md-viewer prints for elementsFile before the window opens: DOC-9's
// line, then one for each image that is not drawn (STYLE-IMG-3). Byte for byte.
const preWindowLines = "md-viewer: 4 HTML fragment(s) not shown: HTML is not rendered\n" +
	"md-viewer: image not shown: img/missing.png: no such file or directory\n" +
	"md-viewer: image not shown: https://example.com/remote.png: remote images are not loaded\n"

// cannotOpen starts the one line that says the window cannot be opened (CLI-9).
const cannotOpen = "md-viewer: cannot open a window: "

// smokeLine is the --time line (SPEED-2).
var smokeLine = regexp.MustCompile(`^read_ms=[0-9]+\.[0-9] parse_ms=[0-9]+\.[0-9] fonts_ms=[0-9]+\.[0-9] first_frame_ms=[0-9]+\.[0-9]\n$`)

// failWriter is a writer whose every write fails.
type failWriter struct{}

// Write fails.
func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// runCmd runs name with args, in env when env is not nil and otherwise in the test's own
// environment. It returns the stdout, the stderr and the exit code.
func runCmd(t *testing.T, env []string, name string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running %s %q: %v", name, args, err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

// runBinary runs bin with args and returns its stdout, stderr and exit code.
func runBinary(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()
	return runCmd(t, nil, bin, args...)
}

// buildBinary builds md-viewer into a temporary directory and returns the binary's path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "md-viewer")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// noDisplayEnv is the test's environment without any display: DISPLAY, WAYLAND_DISPLAY and
// WAYLAND_SOCKET are removed, and XDG_RUNTIME_DIR is an empty directory. Without that directory,
// libwayland looks for a compositor socket in the user's runtime directory, which may be a live
// desktop's. A test that wants X11 under xvfb-run leaves DISPLAY to xvfb-run.
func noDisplayEnv(t *testing.T) []string {
	t.Helper()
	env := []string{"XDG_RUNTIME_DIR=" + t.TempDir()}
	for _, kv := range os.Environ() {
		name := strings.SplitN(kv, "=", 2)[0]
		switch name {
		case "DISPLAY", "WAYLAND_DISPLAY", "WAYLAND_SOCKET", "XDG_RUNTIME_DIR":
			continue
		}
		env = append(env, kv)
	}
	return env
}

func TestMainUsageExit(t *testing.T) {
	bin := buildBinary(t)

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

// TestFrameErrorMapping (CLI-6, CLI-9): the window's result becomes the exit code and the one
// stderr line. A frame failure prints its own error, however it is wrapped; any other error is
// CLI-9's line. A failed write to stderr still gives exit code 1.
func TestFrameErrorMapping(t *testing.T) {
	frame := &window.FrameError{Err: errors.New("boom")}
	cases := []struct {
		name   string
		err    error
		code   int
		stderr string
	}{
		{name: "closed normally", err: nil, code: 0, stderr: ""},
		{name: "frame failure", err: frame, code: 1, stderr: "md-viewer: boom\n"},
		{name: "wrapped frame failure", err: fmt.Errorf("x: %w", frame), code: 1, stderr: "md-viewer: boom\n"},
		{name: "window cannot open", err: errors.New("wayland: no"), code: 1,
			stderr: "md-viewer: cannot open a window: wayland: no\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if got := int(exitCode(c.err, &stderr)); got != c.code {
				t.Errorf("exit code = %d, want %d", got, c.code)
			}
			if stderr.String() != c.stderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), c.stderr)
			}
		})
	}

	t.Run("failed write still exits 1", func(t *testing.T) {
		if got := int(exitCode(frame, failWriter{})); got != 1 {
			t.Errorf("exit code = %d, want 1", got)
		}
	})
}

// TestBinaryNoDisplay (CLI-9): with no display, md-viewer prints the three lines it prints before
// the window, then one line that says the window cannot be opened, with no stdout, and exits 1.
// The same holds with --time, and for a plain file with no lines before the window.
func TestBinaryNoDisplay(t *testing.T) {
	bin := buildBinary(t)
	plain := filepath.Join(t.TempDir(), "plain.md")
	writeFile(t, plain, "# a\n")
	cases := []struct {
		name string
		args []string
		pre  string
	}{
		{name: "elements", args: []string{elementsFile}, pre: preWindowLines},
		{name: "elements with --time", args: []string{"--time", elementsFile}, pre: preWindowLines},
		{name: "plain file", args: []string{plain}, pre: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, code := runCmd(t, noDisplayEnv(t), bin, c.args...)
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, c.pre) {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, c.pre)
			}
			last := stderr[len(c.pre):]
			if !strings.HasPrefix(last, cannotOpen) || strings.Count(last, "\n") != 1 || !strings.HasSuffix(last, "\n") {
				t.Errorf("the line after the window's preface is %q, want one line starting %q", last, cannotOpen)
			}
		})
	}
}

// TestBinaryTimeUnderXvfb (SPEED-2, the smoke run): under xvfb-run the binary opens its window,
// draws the first frame, prints the --time line and exits 0. The environment hides every display
// but the one xvfb-run starts (noDisplayEnv), so no Wayland compositor is reached. Only the lines
// that start with md-viewer's prefix are checked: Mesa and the other libraries print their own.
// A missing xvfb-run fails the test; it never skips. The test is not parallel, since concurrent
// xvfb-run -a calls can race for a display number.
func TestBinaryTimeUnderXvfb(t *testing.T) {
	if _, err := exec.LookPath("xvfb-run"); err != nil {
		t.Fatalf("the image lacks xvfb-run, which this test needs: %v", err)
	}
	bin := buildBinary(t)
	cases := []struct {
		name string
		args []string
	}{
		{name: "plain", args: []string{"--time", elementsFile}},
		{name: "dark", args: []string{"--dark", "--time", elementsFile}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"-a", bin}, c.args...)
			stdout, stderr, code := runCmd(t, noDisplayEnv(t), "xvfb-run", args...)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if !smokeLine.MatchString(stdout) {
				t.Errorf("stdout = %q, want one --time line", stdout)
			}
			var own strings.Builder
			for _, line := range strings.SplitAfter(stderr, "\n") {
				if strings.HasPrefix(line, "md-viewer: ") {
					own.WriteString(line)
				}
			}
			if own.String() != preWindowLines {
				t.Errorf("md-viewer's stderr lines are %q, want the three lines before the window", own.String())
			}
		})
	}
}
