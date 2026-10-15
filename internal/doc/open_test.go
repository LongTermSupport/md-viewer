package doc

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// closeOpened closes f, failing the test on error. A test calls it for every file it opened.
func closeOpened(t *testing.T, f *os.File) {
	t.Helper()
	if err := f.Close(); err != nil {
		t.Errorf("close %s: %v", f.Name(), err)
	}
}

// wantError checks a refused open: no file, and an error that is or wraps want.
func wantError(t *testing.T, f *os.File, err, want error) {
	t.Helper()
	if f != nil {
		t.Errorf("got a file %s, want none", f.Name())
		closeOpened(t, f)
	}
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestOpenRegular (CLI-8): a regular file opens with its size and content; a missing path gives
// the operating system's error; a directory gives ErrDirectory. A refused path gives no file.
func TestOpenRegular(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a.md")
		writeTestFile(t, path, "hello\n")
		f, fi, err := OpenRegular(path)
		if err != nil {
			t.Fatalf("OpenRegular: %v", err)
		}
		if f == nil || fi == nil {
			t.Fatalf("OpenRegular returned no file or no info")
		}
		t.Cleanup(func() { closeOpened(t, f) })
		if fi.Size() != 6 {
			t.Errorf("size = %d, want 6", fi.Size())
		}
		got, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(got) != "hello\n" {
			t.Errorf("content = %q, want %q", got, "hello\n")
		}
	})

	t.Run("missing", func(t *testing.T) {
		f, _, err := OpenRegular(filepath.Join(t.TempDir(), "missing.md"))
		wantError(t, f, err, fs.ErrNotExist)
		var pe *fs.PathError
		if !errors.As(err, &pe) {
			t.Errorf("err = %v, want an *fs.PathError", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		f, _, err := OpenRegular(t.TempDir())
		wantError(t, f, err, ErrDirectory)
	})
}

// TestOpenFlags (CLI-8): the opener reads only, and asks for a non-blocking open that takes no
// controlling terminal. Windows ignores the two bits, which is why they are not build-tagged.
func TestOpenFlags(t *testing.T) {
	if openFlags&(os.O_RDONLY|os.O_WRONLY|os.O_RDWR) != os.O_RDONLY {
		t.Errorf("openFlags %#x does not open read-only", openFlags)
	}
	if openFlags&syscall.O_NONBLOCK == 0 {
		t.Errorf("openFlags %#x lacks O_NONBLOCK", openFlags)
	}
	if openFlags&syscall.O_NOCTTY == 0 {
		t.Errorf("openFlags %#x lacks O_NOCTTY", openFlags)
	}
}
