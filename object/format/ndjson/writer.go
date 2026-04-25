// Package ndjson provides NDJSON (newline-delimited JSON) format support for omnistorage.
package ndjson

import (
	"bufio"
	"bytes"
	"io"
	"sync"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

const (
	// DefaultBufferSize is the default buffer size for writers.
	DefaultBufferSize = 64 * 1024 // 64KB
)

// Writer implements omnistorage.RecordWriter for NDJSON format.
// Each record is written as a single line followed by a newline character.
type Writer struct {
	w       *bufio.Writer
	closer  io.Closer
	closed  bool
	mu      sync.Mutex
	newline []byte
}

// NewWriter creates a new NDJSON writer that writes to the given io.WriteCloser.
// The writer will be closed when the NDJSON writer is closed.
func NewWriter(w io.WriteCloser) *Writer {
	return NewWriterSize(w, DefaultBufferSize)
}

// NewWriterSize creates a new NDJSON writer with the specified buffer size.
func NewWriterSize(w io.WriteCloser, bufferSize int) *Writer {
	return &Writer{
		w:       bufio.NewWriterSize(w, bufferSize),
		closer:  w,
		newline: []byte{'\n'},
	}
}

// Write writes a single record (JSON line) to the writer.
// The record should not contain embedded newlines; if it does, they will be
// preserved but may cause issues when reading.
func (w *Writer) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return omnistorage.ErrWriterClosed
	}

	// Write the data
	if _, err := w.w.Write(data); err != nil {
		return err
	}

	// Write newline delimiter
	if _, err := w.w.Write(w.newline); err != nil {
		return err
	}

	return nil
}

// WriteJSON writes a single record, trimming any trailing whitespace/newlines
// before adding the standard newline delimiter.
func (w *Writer) WriteJSON(data []byte) error {
	return w.Write(bytes.TrimRight(data, " \t\r\n"))
}

// Flush flushes any buffered data to the underlying writer.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return omnistorage.ErrWriterClosed
	}

	return w.w.Flush()
}

// Close flushes any remaining data and closes the writer.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Flush buffered data
	if err := w.w.Flush(); err != nil {
		_ = w.closer.Close()
		return err
	}

	return w.closer.Close()
}

// Ensure Writer implements omnistorage.RecordWriter
var _ omnistorage.RecordWriter = (*Writer)(nil)
