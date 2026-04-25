// Package gzip provides gzip compression support for omnistorage.
package gzip

import (
	"compress/gzip"
	"io"
	"sync"
)

// CompressionLevel represents gzip compression levels.
type CompressionLevel int

const (
	// NoCompression provides no compression.
	NoCompression CompressionLevel = gzip.NoCompression
	// BestSpeed provides fastest compression.
	BestSpeed CompressionLevel = gzip.BestSpeed
	// BestCompression provides best compression ratio.
	BestCompression CompressionLevel = gzip.BestCompression
	// DefaultCompression provides a balance of speed and compression.
	DefaultCompression CompressionLevel = gzip.DefaultCompression
	// HuffmanOnly uses Huffman encoding only.
	HuffmanOnly CompressionLevel = gzip.HuffmanOnly
)

// Writer wraps an io.WriteCloser with gzip compression.
type Writer struct {
	gw     *gzip.Writer
	closer io.Closer
	closed bool
	mu     sync.Mutex
}

// NewWriter creates a new gzip writer with default compression level.
func NewWriter(w io.WriteCloser) (*Writer, error) {
	return NewWriterLevel(w, DefaultCompression)
}

// NewWriterLevel creates a new gzip writer with the specified compression level.
func NewWriterLevel(w io.WriteCloser, level CompressionLevel) (*Writer, error) {
	gw, err := gzip.NewWriterLevel(w, int(level))
	if err != nil {
		return nil, err
	}
	return &Writer{
		gw:     gw,
		closer: w,
	}, nil
}

// Write writes compressed data to the underlying writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, io.ErrClosedPipe
	}

	return w.gw.Write(p)
}

// Flush flushes any pending compressed data.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return io.ErrClosedPipe
	}

	return w.gw.Flush()
}

// Close flushes any remaining data and closes both the gzip writer
// and the underlying writer.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Close gzip writer first (flushes remaining data)
	if err := w.gw.Close(); err != nil {
		_ = w.closer.Close()
		return err
	}

	return w.closer.Close()
}

// Ensure Writer implements io.WriteCloser
var _ io.WriteCloser = (*Writer)(nil)
