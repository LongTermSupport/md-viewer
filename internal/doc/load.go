package doc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// MaxFileSize is the largest file Load reads: 64 MiB (CLI-8, DOC-13).
const MaxFileSize = 64 << 20

// nulByte is the byte that makes a file binary rather than text (CLI-8).
const nulByte = 0

// LoadStats are the two timings of Load, for --time (SPEED-2, DOC-13). Read covers the stat,
// the read and the NUL check; Parse covers Parse.
type LoadStats struct{ Read, Parse time.Duration }

// ReadError is a file that cannot be stat'ed or read (CLI-8). Path is as passed to Load. Err is
// the operating system's error under the *fs.PathError.
type ReadError struct {
	Path string
	Err  error
}

func (e *ReadError) Error() string { return "read " + e.Path + ": " + e.Err.Error() }

// Unwrap returns the operating system's error, so errors.Is sees it.
func (e *ReadError) Unwrap() error { return e.Err }

// DirectoryError is a path that names a directory (CLI-8).
type DirectoryError struct{ Path string }

func (e *DirectoryError) Error() string { return e.Path + " is a directory" }

// TooLargeError is a file larger than MaxFileSize (CLI-8). Size is its size in bytes.
type TooLargeError struct {
	Path string
	Size int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("%s is %d bytes, larger than the limit of %d", e.Path, e.Size, MaxFileSize)
}

// BinaryError is a file with a NUL byte in it (CLI-8).
type BinaryError struct{ Path string }

func (e *BinaryError) Error() string { return e.Path + " contains a NUL byte" }

// Load reads and parses the file at path (DOC-13). It applies CLI-8's checks in order: stat,
// directory, size, read, NUL byte. The errors of those checks are the typed errors above, as
// pointers. now is called three times on success: before the stat, after the NUL check, and
// after Parse. Document.Name is the base name of path and Document.Dir the absolute directory.
func Load(path string, now func() time.Time) (*Document, LoadStats, error) {
	t0 := now()
	info, err := os.Stat(path)
	if err != nil {
		return nil, LoadStats{}, readError(path, err)
	}
	if info.IsDir() {
		return nil, LoadStats{}, &DirectoryError{Path: path}
	}
	if info.Size() > MaxFileSize {
		return nil, LoadStats{}, &TooLargeError{Path: path, Size: info.Size()}
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, LoadStats{}, readError(path, err)
	}
	if bytes.IndexByte(src, nulByte) >= 0 {
		return nil, LoadStats{}, &BinaryError{Path: path}
	}
	t1 := now()
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, LoadStats{}, err
	}
	d, err := Parse(src, filepath.Dir(abs))
	if err != nil {
		return nil, LoadStats{}, err
	}
	t2 := now()
	d.Name = filepath.Base(path)
	return d, LoadStats{Read: t1.Sub(t0), Parse: t2.Sub(t1)}, nil
}

// readCapped is a stub for the red run: it reads at most MaxFileSize bytes and does not detect
// the overflow.
func readCapped(r io.Reader, path string) ([]byte, error) {
	src, err := io.ReadAll(io.LimitReader(r, MaxFileSize))
	if err != nil {
		return nil, readError(path, err)
	}
	return src, nil
}

// readError wraps an error from the operating system about path as a ReadError. The error
// under the *fs.PathError is kept, since its text is CLI-8's REASON.
func readError(path string, err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return &ReadError{Path: path, Err: err}
}
