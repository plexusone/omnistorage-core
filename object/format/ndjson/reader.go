package ndjson

import (
	"bufio"
	"bytes"
	"io"
	"sync"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

// Reader implements omnistorage.RecordReader for NDJSON format.
// Each record is read as a single line (delimited by newlines).
type Reader struct {
	scanner *bufio.Scanner
	closer  io.Closer
	closed  bool
	mu      sync.Mutex
}

// NewReader creates a new NDJSON reader that reads from the given io.ReadCloser.
// The reader will be closed when the NDJSON reader is closed.
func NewReader(r io.ReadCloser) *Reader {
	return NewReaderSize(r, DefaultBufferSize)
}

// NewReaderSize creates a new NDJSON reader with the specified buffer size.
// The buffer size determines the maximum line length that can be read.
func NewReaderSize(r io.ReadCloser, bufferSize int) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, bufferSize), bufferSize)
	return &Reader{
		scanner: scanner,
		closer:  r,
	}
}

// Read reads the next record (JSON line) from the reader.
// Returns io.EOF when no more records are available.
// Empty lines are skipped.
// The returned slice is a copy and is valid until the next call to Read.
func (r *Reader) Read() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, omnistorage.ErrReaderClosed
	}

	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		// Skip empty lines
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		// Return a copy since scanner reuses the buffer
		result := make([]byte, len(line))
		copy(result, line)
		return result, nil
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}

// Close releases any resources held by the reader.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return r.closer.Close()
}

// Ensure Reader implements omnistorage.RecordReader
var _ omnistorage.RecordReader = (*Reader)(nil)
