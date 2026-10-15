//go:build linux

package doc

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// openBounded runs call, which opens fifo, in a goroutine, and returns its result. A call that has
// not returned after a minute is a failure, and only a failure reaches the bound: the test then
// opens fifo for reading and writing, which does not block on Linux and so releases a blocked open,
// waits for call to return, and fails. On success the result arrives at once.
func openBounded(t *testing.T, fifo string, call func() (*os.File, error)) (*os.File, error) {
	t.Helper()
	type result struct {
		f   *os.File
		err error
	}
	done := make(chan result, 1)
	go func() {
		f, err := call()
		done <- result{f, err}
	}()
	select {
	case r := <-done:
		return r.f, r.err
	case <-time.After(time.Minute):
	}
	release, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open %s to release the blocked call: %v", fifo, err)
	}
	r := <-done
	if r.f != nil {
		closeOpened(t, r.f)
	}
	closeOpened(t, release)
	t.Fatalf("a call blocked on the FIFO %s with no writer", fifo)
	return nil, nil
}

// mkfifo makes a FIFO at path.
func mkfifo(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo %s: %v", path, err)
	}
}

// mknodSocket makes a socket node at path. Opening it fails with ENXIO.
func mknodSocket(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mknod(path, syscall.S_IFSOCK|0o600, 0); err != nil {
		t.Fatalf("mknod socket %s: %v", path, err)
	}
}

// symlink makes link point at target.
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s: %v", link, err)
	}
}

// TestOpenStatNonRegular (CLI-8): openStat is the open-and-check step after the first Stat. Its
// path here is not a regular file at open time, whatever the first Stat saw: a path replaced after
// that Stat. Each row gives no file. A FIFO with no writer would block an open without
// O_NONBLOCK, so those rows are bounded.
func TestOpenStatNonRegular(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	mkfifo(t, fifo)
	fifoHeld := filepath.Join(dir, "fifo-held")
	mkfifo(t, fifoHeld)
	fifoLink := filepath.Join(dir, "fifo-link")
	symlink(t, fifo, fifoLink)
	sock := filepath.Join(dir, "sock")
	mknodSocket(t, sock)

	t.Run("fifo without a writer", func(t *testing.T) {
		f, err := openBounded(t, fifo, func() (*os.File, error) {
			f, _, err := openStat(fifo)
			return f, err
		})
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("fifo with a writer", func(t *testing.T) {
		held, err := os.OpenFile(fifoHeld, os.O_RDWR, 0)
		if err != nil {
			t.Fatalf("open %s O_RDWR: %v", fifoHeld, err)
		}
		t.Cleanup(func() { closeOpened(t, held) })
		if _, err := held.Write([]byte("x")); err != nil {
			t.Fatalf("write to the FIFO: %v", err)
		}
		f, _, err := openStat(fifoHeld)
		wantError(t, f, err, ErrNotRegular)
		buf := make([]byte, 1)
		if _, err := io.ReadFull(held, buf); err != nil {
			t.Fatalf("read the FIFO: %v", err)
		}
		if buf[0] != 'x' {
			t.Errorf("the FIFO holds %q after openStat, want the byte x unread", buf)
		}
	})

	t.Run("symlink to a fifo without a writer", func(t *testing.T) {
		f, err := openBounded(t, fifo, func() (*os.File, error) {
			f, _, err := openStat(fifoLink)
			return f, err
		})
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("/dev/null", func(t *testing.T) {
		f, _, err := openStat(os.DevNull)
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("/dev/zero", func(t *testing.T) {
		f, _, err := openStat("/dev/zero")
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("socket", func(t *testing.T) {
		f, _, err := openStat(sock)
		wantError(t, f, err, syscall.ENXIO)
		var pe *fs.PathError
		if !errors.As(err, &pe) {
			t.Errorf("err = %v, want an *fs.PathError", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		f, _, err := openStat(dir)
		wantError(t, f, err, ErrDirectory)
	})
}

// TestOpenStatRegular (CLI-8): a regular file, and a symbolic link to one, open and read in full,
// and the file can be read again after a seek.
func TestOpenStatRegular(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	writeTestFile(t, path, "hello\n")
	link := filepath.Join(dir, "link.md")
	symlink(t, path, link)

	for _, p := range []string{path, link} {
		t.Run(filepath.Base(p), func(t *testing.T) {
			f, fi, err := openStat(p)
			if err != nil {
				t.Fatalf("openStat: %v", err)
			}
			if f == nil || fi == nil {
				t.Fatalf("openStat returned no file or no info")
			}
			t.Cleanup(func() { closeOpened(t, f) })
			if !fi.Mode().IsRegular() {
				t.Errorf("mode %v, want a regular file", fi.Mode())
			}
			for pass := range 2 {
				if pass == 1 {
					if _, err := f.Seek(0, io.SeekStart); err != nil {
						t.Fatalf("seek: %v", err)
					}
				}
				got, err := io.ReadAll(f)
				if err != nil {
					t.Fatalf("read pass %d: %v", pass, err)
				}
				if string(got) != "hello\n" {
					t.Errorf("read pass %d = %q, want %q", pass, got, "hello\n")
				}
			}
		})
	}
}

// TestOpenRegularNonRegular (CLI-8): the first Stat refuses a path that is not a regular file before
// any open, so none of these opens. A socket node is refused with ErrNotRegular here, where
// openStat gives ENXIO for the same node swapped in after its Stat. A symbolic link to a regular
// file opens. The FIFO row is bounded, as the open would block if the Stat were skipped.
func TestOpenRegularNonRegular(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	mkfifo(t, fifo)
	sock := filepath.Join(dir, "sock")
	mknodSocket(t, sock)
	path := filepath.Join(dir, "a.md")
	writeTestFile(t, path, "hello\n")
	link := filepath.Join(dir, "link.md")
	symlink(t, path, link)

	t.Run("fifo without a writer", func(t *testing.T) {
		f, err := openBounded(t, fifo, func() (*os.File, error) {
			f, _, err := OpenRegular(fifo)
			return f, err
		})
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("/dev/zero", func(t *testing.T) {
		f, _, err := OpenRegular("/dev/zero")
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("socket", func(t *testing.T) {
		f, _, err := OpenRegular(sock)
		wantError(t, f, err, ErrNotRegular)
	})

	t.Run("symlink to a regular file", func(t *testing.T) {
		f, fi, err := OpenRegular(link)
		if err != nil {
			t.Fatalf("OpenRegular: %v", err)
		}
		if f == nil || fi == nil {
			t.Fatalf("OpenRegular returned no file or no info")
		}
		t.Cleanup(func() { closeOpened(t, f) })
		if !fi.Mode().IsRegular() {
			t.Errorf("mode %v, want a regular file", fi.Mode())
		}
	})
}
