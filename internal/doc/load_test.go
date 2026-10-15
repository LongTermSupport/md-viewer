package doc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// stepClock is a fake clock that advances 1 ms per reading, so tests never read the wall clock.
type stepClock struct{ t time.Time }

func (c *stepClock) Now() time.Time {
	now := c.t
	c.t = c.t.Add(time.Millisecond)
	return now
}

// newStepClock returns a fake clock that starts at a fixed time.
func newStepClock() *stepClock {
	return &stepClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestMaxFileSize pins the limit of CLI-8 (67108864 bytes, 64 MiB), written as a literal.
func TestMaxFileSize(t *testing.T) {
	if MaxFileSize != 67108864 {
		t.Errorf("MaxFileSize = %d, want 67108864", MaxFileSize)
	}
}

// TestLoadErrors checks each CLI-8 condition gives its error type with the right fields.
func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	clock := newStepClock()

	t.Run("missing file", func(t *testing.T) {
		p := filepath.Join(dir, "nosuch.md")
		d, _, err := Load(p, clock.Now)
		var re *ReadError
		if !errors.As(err, &re) {
			t.Fatalf("Load(%q) error = %v, want a *ReadError", p, err)
		}
		if d != nil {
			t.Fatalf("Load(%q) document = %v, want nil", p, d)
		}
		if re.Path != p {
			t.Errorf("ReadError.Path = %q, want %q", re.Path, p)
		}
		if !errors.Is(re.Err, fs.ErrNotExist) {
			t.Errorf("ReadError.Err = %v, want fs.ErrNotExist", re.Err)
		}
		if re.Err.Error() != "no such file or directory" {
			t.Errorf("ReadError.Err = %q, want %q", re.Err.Error(), "no such file or directory")
		}
	})

	t.Run("directory", func(t *testing.T) {
		d, _, err := Load(dir, clock.Now)
		var de *DirectoryError
		if !errors.As(err, &de) {
			t.Fatalf("Load(%q) error = %v, want a *DirectoryError", dir, err)
		}
		if d != nil {
			t.Fatalf("Load(%q) document = %v, want nil", dir, d)
		}
		if de.Path != dir {
			t.Errorf("DirectoryError.Path = %q, want %q", de.Path, dir)
		}
	})

	t.Run("too large", func(t *testing.T) {
		p := filepath.Join(dir, "big.md")
		writeTestFile(t, p, "")
		// A sparse file: it is not written out, but Stat reports its size.
		if err := os.Truncate(p, MaxFileSize+1); err != nil {
			t.Fatalf("truncate %s: %v", p, err)
		}
		d, _, err := Load(p, clock.Now)
		var te *TooLargeError
		if !errors.As(err, &te) {
			t.Fatalf("Load(%q) error = %v, want a *TooLargeError", p, err)
		}
		if d != nil {
			t.Fatalf("Load(%q) document = %v, want nil", p, d)
		}
		if te.Path != p {
			t.Errorf("TooLargeError.Path = %q, want %q", te.Path, p)
		}
		if te.Size != 67108865 {
			t.Errorf("TooLargeError.Size = %d, want 67108865", te.Size)
		}
	})

	t.Run("NUL byte", func(t *testing.T) {
		p := filepath.Join(dir, "nul.md")
		writeTestFile(t, p, "a\x00b")
		d, _, err := Load(p, clock.Now)
		var be *BinaryError
		if !errors.As(err, &be) {
			t.Fatalf("Load(%q) error = %v, want a *BinaryError", p, err)
		}
		if d != nil {
			t.Fatalf("Load(%q) document = %v, want nil", p, d)
		}
		if be.Path != p {
			t.Errorf("BinaryError.Path = %q, want %q", be.Path, p)
		}
	})
}

// TestLoadOK loads the fixture with a 1 ms clock: the name and absolute directory are set, and
// each of Read and Parse covers one step of the clock (DOC-13).
func TestLoadOK(t *testing.T) {
	d, stats, err := Load(elementsPath, newStepClock().Now)
	if err != nil {
		t.Fatalf("Load(%q) error = %v, want nil", elementsPath, err)
	}
	if d == nil {
		t.Fatalf("Load(%q) document = nil, want a document", elementsPath)
	}
	if d.Name != "elements.md" {
		t.Errorf("Name = %q, want %q", d.Name, "elements.md")
	}
	wantDir, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if !filepath.IsAbs(d.Dir) || d.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", d.Dir, wantDir)
	}
	if stats.Read != time.Millisecond {
		t.Errorf("stats.Read = %v, want 1ms", stats.Read)
	}
	if stats.Parse != time.Millisecond {
		t.Errorf("stats.Parse = %v, want 1ms", stats.Parse)
	}
	if len(d.Blocks) == 0 {
		t.Errorf("Blocks is empty, want the fixture's blocks")
	}
}

// TestLoadAnyExtension checks that the extension is not checked (CLI-10): a file with any
// name, or none, is read as Markdown.
func TestLoadAnyExtension(t *testing.T) {
	dir := t.TempDir()
	want := []Block{hd(1, txt("a", 0))}
	for _, name := range []string{"notes.txt", "README"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, name)
			writeTestFile(t, p, "# a\n")
			d, _, err := Load(p, newStepClock().Now)
			if err != nil {
				t.Fatalf("Load(%q) error = %v, want nil", p, err)
			}
			if d == nil {
				t.Fatalf("Load(%q) document = nil, want a document", p)
			}
			if !reflect.DeepEqual(d.Blocks, want) {
				t.Errorf("Blocks = %#v, want %#v", d.Blocks, want)
			}
		})
	}
}
