// Package multi provides fan-out writing to multiple backends simultaneously.
//
// The multi-writer allows writing the same data to multiple storage backends
// at once, useful for:
//   - Replication across storage providers
//   - Writing to both local and remote storage
//   - Backup during write operations
//   - Testing with multiple backends
//
// Example usage:
//
//	// Create backends
//	local := memory.New()
//	s3Backend, _ := s3.New(config)
//	gcsBackend, _ := gcs.New(config)
//
//	// Create multi-writer
//	mw := multi.NewWriter(local, s3Backend, gcsBackend)
//
//	// Write to all backends simultaneously
//	w, _ := mw.NewWriter(ctx, "data/file.json")
//	w.Write([]byte(`{"key": "value"}`))
//	w.Close()
package multi

import (
	"context"
	"errors"
	"io"
	"sync"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

// WriteMode determines how the multi-writer handles failures.
type WriteMode int

const (
	// WriteAll requires all backends to succeed.
	// If any backend fails, the entire write fails.
	WriteAll WriteMode = iota

	// WriteBestEffort writes to all backends but continues on failure.
	// Errors are collected and returned, but writing continues.
	WriteBestEffort

	// WriteQuorum requires a majority of backends to succeed.
	WriteQuorum
)

// Writer provides fan-out writing to multiple backends.
type Writer struct {
	backends []omnistorage.Backend
	mode     WriteMode
	mu       sync.RWMutex
}

// Option configures a multi-writer.
type Option func(*Writer)

// WithMode sets the write mode.
func WithMode(mode WriteMode) Option {
	return func(w *Writer) {
		w.mode = mode
	}
}

// NewWriter creates a new multi-writer for the given backends.
// At least one backend must be provided.
func NewWriter(backends ...omnistorage.Backend) (*Writer, error) {
	if len(backends) == 0 {
		return nil, errors.New("at least one backend is required")
	}

	// Filter nil backends
	var validBackends []omnistorage.Backend
	for _, b := range backends {
		if b != nil {
			validBackends = append(validBackends, b)
		}
	}

	if len(validBackends) == 0 {
		return nil, errors.New("no valid backends provided")
	}

	return &Writer{
		backends: validBackends,
		mode:     WriteAll,
	}, nil
}

// NewWriterWithOptions creates a new multi-writer with options.
func NewWriterWithOptions(backends []omnistorage.Backend, opts ...Option) (*Writer, error) {
	w, err := NewWriter(backends...)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// NewWriter creates a writer that fans out to all backends.
func (w *Writer) NewWriter(ctx context.Context, path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Create writers for all backends
	writers := make([]io.WriteCloser, 0, len(w.backends))
	var errs []error

	for _, b := range w.backends {
		writer, err := b.NewWriter(ctx, path, opts...)
		if err != nil {
			errs = append(errs, err)
			if w.mode == WriteAll {
				// Close any writers we've created
				for _, wr := range writers {
					_ = wr.Close()
				}
				return nil, &MultiError{Errors: errs}
			}
			continue
		}
		writers = append(writers, writer)
	}

	// Check quorum
	if w.mode == WriteQuorum && len(writers) <= len(w.backends)/2 {
		for _, wr := range writers {
			_ = wr.Close()
		}
		return nil, &MultiError{Errors: append(errs, errors.New("failed to achieve write quorum"))}
	}

	if len(writers) == 0 {
		return nil, &MultiError{Errors: errs}
	}

	return &multiWriteCloser{
		writers: writers,
		mode:    w.mode,
		errs:    errs,
	}, nil
}

// Backends returns the number of backends.
func (w *Writer) Backends() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.backends)
}

// multiWriteCloser writes to multiple underlying writers.
type multiWriteCloser struct {
	writers []io.WriteCloser
	mode    WriteMode
	errs    []error
	mu      sync.Mutex
	closed  bool
}

// Write writes data to all underlying writers.
func (m *multiWriteCloser) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, omnistorage.ErrWriterClosed
	}

	var (
		successCount int
		writeErrs    []error
		maxN         int
	)

	for _, w := range m.writers {
		written, werr := w.Write(p)
		if werr != nil {
			writeErrs = append(writeErrs, werr)
			if m.mode == WriteAll {
				return 0, &MultiError{Errors: writeErrs}
			}
		} else {
			successCount++
			if written > maxN {
				maxN = written
			}
		}
	}

	// Check quorum
	if m.mode == WriteQuorum && successCount <= len(m.writers)/2 {
		return 0, &MultiError{Errors: append(writeErrs, errors.New("write quorum not achieved"))}
	}

	if successCount == 0 {
		return 0, &MultiError{Errors: writeErrs}
	}

	// For best effort and quorum modes, return the max bytes written
	return maxN, nil
}

// Close closes all underlying writers.
func (m *multiWriteCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	var closeErrs []error
	for _, w := range m.writers {
		if err := w.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	if len(closeErrs) > 0 {
		return &MultiError{Errors: closeErrs}
	}
	return nil
}

// MultiError represents multiple errors from multi-backend operations.
type MultiError struct {
	Errors []error
}

// Error implements the error interface.
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return e.Errors[0].Error() + " (and more errors)"
}

// Unwrap returns the first error for errors.Is/As compatibility.
func (e *MultiError) Unwrap() error {
	if len(e.Errors) > 0 {
		return e.Errors[0]
	}
	return nil
}

// All returns all errors.
func (e *MultiError) All() []error {
	return e.Errors
}
