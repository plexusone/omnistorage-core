// Package zstd provides Zstandard compression support for omnistorage.
//
// Zstandard (zstd) offers better compression ratios than gzip at similar speeds,
// and significantly faster decompression. It's ideal for:
//   - Log files and structured data (NDJSON, CSV)
//   - Large file archives
//   - Real-time compression where decompression speed matters
//
// Basic usage:
//
//	fileWriter, _ := backend.NewWriter(ctx, "data.ndjson.zst")
//	zstdWriter, _ := zstd.NewWriter(fileWriter)
//	// Write data...
//	zstdWriter.Close()
package zstd

import (
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// CompressionLevel represents zstd compression levels.
type CompressionLevel int

const (
	// SpeedFastest provides the fastest compression speed.
	// Compression ratio is lower but speed is maximized.
	SpeedFastest CompressionLevel = iota + 1

	// SpeedDefault provides a good balance of speed and compression.
	// This is the recommended level for most use cases.
	SpeedDefault

	// SpeedBetterCompression provides better compression at slower speed.
	SpeedBetterCompression

	// SpeedBestCompression provides the best compression ratio.
	// Significantly slower than other levels.
	SpeedBestCompression
)

// toZstdLevel converts our level to the zstd library's level.
func (l CompressionLevel) toZstdLevel() zstd.EncoderLevel {
	switch l {
	case SpeedFastest:
		return zstd.SpeedFastest
	case SpeedDefault:
		return zstd.SpeedDefault
	case SpeedBetterCompression:
		return zstd.SpeedBetterCompression
	case SpeedBestCompression:
		return zstd.SpeedBestCompression
	default:
		return zstd.SpeedDefault
	}
}

// Writer wraps an io.WriteCloser with zstd compression.
type Writer struct {
	zw     *zstd.Encoder
	closer io.Closer
	closed bool
	mu     sync.Mutex
}

// NewWriter creates a new zstd writer with default compression level.
func NewWriter(w io.WriteCloser) (*Writer, error) {
	return NewWriterLevel(w, SpeedDefault)
}

// NewWriterLevel creates a new zstd writer with the specified compression level.
func NewWriterLevel(w io.WriteCloser, level CompressionLevel) (*Writer, error) {
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(level.toZstdLevel()))
	if err != nil {
		return nil, err
	}
	return &Writer{
		zw:     zw,
		closer: w,
	}, nil
}

// NewWriterWithOptions creates a new zstd writer with custom options.
// This allows fine-grained control over compression parameters.
func NewWriterWithOptions(w io.WriteCloser, opts ...zstd.EOption) (*Writer, error) {
	zw, err := zstd.NewWriter(w, opts...)
	if err != nil {
		return nil, err
	}
	return &Writer{
		zw:     zw,
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

	return w.zw.Write(p)
}

// Flush flushes any pending compressed data.
// Note: Zstd doesn't have a traditional flush like gzip.
// This method ensures all written data is encoded.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return io.ErrClosedPipe
	}

	return w.zw.Flush()
}

// Close flushes any remaining data and closes both the zstd encoder
// and the underlying writer.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Close zstd encoder first (flushes remaining data)
	if err := w.zw.Close(); err != nil {
		_ = w.closer.Close()
		return err
	}

	return w.closer.Close()
}

// Ensure Writer implements io.WriteCloser
var _ io.WriteCloser = (*Writer)(nil)
