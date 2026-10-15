package doc

import "time"

// MaxFileSize is a stub until the red commit's implementation step.
const MaxFileSize = 0

// LoadStats is a stub.
type LoadStats struct{ Read, Parse time.Duration }

// ReadError is a stub.
type ReadError struct {
	Path string
	Err  error
}

func (e *ReadError) Error() string { return "" }

// DirectoryError is a stub.
type DirectoryError struct{ Path string }

func (e *DirectoryError) Error() string { return "" }

// TooLargeError is a stub.
type TooLargeError struct {
	Path string
	Size int64
}

func (e *TooLargeError) Error() string { return "" }

// BinaryError is a stub.
type BinaryError struct{ Path string }

func (e *BinaryError) Error() string { return "" }

// Load is a stub.
func Load(path string, now func() time.Time) (*Document, LoadStats, error) {
	return nil, LoadStats{}, nil
}
