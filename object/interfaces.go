// Package omnistorage provides a unified storage abstraction layer for Go.
//
// It supports multiple storage backends (local files, S3, GCS, etc.) through
// a common interface, with composable layers for compression and record framing.
//
// Basic usage:
//
//	backend, _ := file.New(map[string]string{"root": "/data"})
//	raw, _ := backend.NewWriter(ctx, "logs/app.ndjson")
//	w := ndjson.NewWriter(raw)
//	w.Write([]byte(`{"msg":"hello"}`))
//	w.Close()
package object

import (
	"context"
	"io"
)

// Backend represents a storage backend (S3, GCS, local file, etc.).
// Implementations handle raw byte transport to/from storage.
//
// Backends are safe for concurrent use by multiple goroutines.
// All methods accept a context.Context for cancellation and timeouts.
type Backend interface {
	// NewWriter creates a writer for the given path/key.
	// The returned writer must be closed after use to ensure
	// all data is flushed and resources are released.
	//
	// The path format depends on the backend:
	//   - File backend: relative path from root
	//   - S3 backend: object key
	//   - etc.
	//
	// The context is used for the initial creation; the returned writer
	// may have its own context or timeout handling.
	NewWriter(ctx context.Context, path string, opts ...WriterOption) (io.WriteCloser, error)

	// NewReader creates a reader for the given path/key.
	// Returns ErrNotFound if the path does not exist.
	// The returned reader must be closed after use.
	//
	// The context is used for the initial creation; the returned reader
	// may have its own context or timeout handling.
	NewReader(ctx context.Context, path string, opts ...ReaderOption) (io.ReadCloser, error)

	// Exists checks if a path exists.
	Exists(ctx context.Context, path string) (bool, error)

	// Delete removes a path.
	// Returns nil if the path does not exist (idempotent).
	Delete(ctx context.Context, path string) error

	// List lists paths with the given prefix.
	// Returns an empty slice if no paths match.
	// The returned paths are relative to the backend root.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close releases any resources held by the backend.
	// After Close, all other methods return ErrBackendClosed.
	Close() error
}

// RecordWriter writes framed records (byte slices) to an underlying writer.
// Implementations handle record delimiting (newlines, length-prefix, etc.).
//
// RecordWriter is useful for streaming record-oriented data like logs,
// events, or NDJSON documents.
type RecordWriter interface {
	// Write writes a single record.
	// The record should not contain the delimiter (e.g., no trailing newline for NDJSON).
	// Implementations may buffer writes; call Flush to ensure data is written.
	Write(data []byte) error

	// Flush flushes any buffered data to the underlying writer.
	Flush() error

	// Close flushes any remaining data and closes the writer.
	// After Close, Write and Flush return errors.
	Close() error
}

// RecordReader reads framed records from an underlying reader.
// Implementations handle record parsing (newlines, length-prefix, etc.).
type RecordReader interface {
	// Read reads the next record.
	// Returns io.EOF when no more records are available.
	// The returned slice is valid until the next call to Read.
	Read() ([]byte, error)

	// Close releases any resources held by the reader.
	Close() error
}
