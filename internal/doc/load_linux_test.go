//go:build linux

package doc

import (
	"errors"
	"path/filepath"
	"testing"
)

// checkNotRegular fails the test unless Load gave no document and a *ReadError for path whose Err
// is "not a regular file" (CLI-8).
func checkNotRegular(t *testing.T, path string, d *Document, err error) {
	t.Helper()
	var re *ReadError
	if !errors.As(err, &re) {
		t.Fatalf("Load(%q) error = %v, want a *ReadError", path, err)
	}
	if d != nil {
		t.Errorf("Load(%q) gave a document, want none", path)
	}
	if re.Path != path {
		t.Errorf("ReadError.Path = %q, want %q", re.Path, path)
	}
	if re.Err == nil || re.Err.Error() != "not a regular file" {
		t.Errorf("ReadError.Err = %v, want not a regular file", re.Err)
	}
}

// TestLoadNonRegular (CLI-8, DOC-13): Load refuses a FIFO with no writer and a socket as not a
// regular file, without reading them. The FIFO row is bounded, as Load would block on it if the
// Stat were skipped. A symbolic link to a regular file loads, and takes the link's name and
// directory as Name and Dir.
func TestLoadNonRegular(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo.md")
	mkfifo(t, fifo)
	sock := filepath.Join(dir, "sock.md")
	mknodSocket(t, sock)
	target := filepath.Join(dir, "real.md")
	writeTestFile(t, target, "# Real\n")
	link := filepath.Join(dir, "link.md")
	symlink(t, target, link)
	clock := newStepClock()

	t.Run("fifo without a writer", func(t *testing.T) {
		var d *Document
		var err error
		runBounded(t, fifo, func() { d, _, err = Load(fifo, clock.Now) }, nil)
		checkNotRegular(t, fifo, d, err)
	})

	t.Run("socket", func(t *testing.T) {
		d, _, err := Load(sock, clock.Now)
		checkNotRegular(t, sock, d, err)
	})

	t.Run("symlink to a regular file", func(t *testing.T) {
		d, _, err := Load(link, clock.Now)
		if err != nil {
			t.Fatalf("Load(%q): %v", link, err)
		}
		if d == nil {
			t.Fatalf("Load(%q) gave no document", link)
		}
		if d.Name != "link.md" {
			t.Errorf("Name = %q, want link.md", d.Name)
		}
		if d.Dir != dir {
			t.Errorf("Dir = %q, want %q", d.Dir, dir)
		}
	})
}
