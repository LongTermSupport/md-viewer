package doc

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// ErrDirectory is the reason for a path that names a directory (CLI-8, STYLE-IMG-3).
var ErrDirectory = errors.New("is a directory")

// ErrNotRegular is the reason for a path that is neither a regular file nor a directory (CLI-8,
// STYLE-IMG-3).
var ErrNotRegular = errors.New("not a regular file")

// openFlags opens read-only, without blocking and without taking a controlling terminal. An open
// with O_NONBLOCK does not wait for a FIFO's writer, and O_NOCTTY stops a terminal that is swapped
// in for the file from becoming the controlling terminal. Windows has neither FIFOs nor controlling
// terminals in this sense and ignores both bits, so the constant holds on every platform.
const openFlags = os.O_RDONLY | syscall.O_NONBLOCK | syscall.O_NOCTTY

// OpenRegular opens the regular file at path for reading and returns it with its info (CLI-8).
// Stat comes first, so a path that is not a regular file or a directory is refused without being
// opened: opening a device can act on it. The file is then opened without blocking and judged again
// on the open file, so a path replaced after its Stat is refused too, and is never read. The info is
// the open file's, so its size is the size that will be read. Every error is an *fs.PathError whose
// Err is the operating system's error, ErrDirectory or ErrNotRegular. The caller closes the file.
func OpenRegular(path string) (*os.File, fs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if err := judge(path, info); err != nil {
		return nil, nil, err
	}
	return openStat(path)
}

// openStat opens path without blocking and judges the open file. It is the check that refuses a
// path replaced after its Stat. A refused file is closed, and a close error is joined to the refusal.
func openStat(path string) (*os.File, fs.FileInfo, error) {
	f, err := os.OpenFile(path, openFlags, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err == nil {
		err = judge(path, info)
	}
	if err != nil {
		if cerr := f.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
		return nil, nil, err
	}
	return f, info, nil
}

// judge refuses what md-viewer does not read: a directory with ErrDirectory, and any other file
// that is not regular (a device, FIFO or socket) with ErrNotRegular.
func judge(path string, fi fs.FileInfo) error {
	switch {
	case fi.IsDir():
		return &fs.PathError{Op: "open", Path: path, Err: ErrDirectory}
	case !fi.Mode().IsRegular():
		return &fs.PathError{Op: "open", Path: path, Err: ErrNotRegular}
	}
	return nil
}
