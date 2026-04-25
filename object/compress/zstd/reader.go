package zstd

import (
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Reader wraps an io.ReadCloser with zstd decompression.
type Reader struct {
	zr     *zstd.Decoder
	closer io.Closer
	closed bool
	mu     sync.Mutex
}

// NewReader creates a new zstd reader that decompresses data from the underlying reader.
func NewReader(r io.ReadCloser) (*Reader, error) {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &Reader{
		zr:     zr,
		closer: r,
	}, nil
}

// NewReaderWithOptions creates a new zstd reader with custom options.
// This allows fine-grained control over decompression parameters.
func NewReaderWithOptions(r io.ReadCloser, opts ...zstd.DOption) (*Reader, error) {
	zr, err := zstd.NewReader(r, opts...)
	if err != nil {
		return nil, err
	}
	return &Reader{
		zr:     zr,
		closer: r,
	}, nil
}

// Read reads decompressed data from the underlying reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, io.ErrClosedPipe
	}

	return r.zr.Read(p)
}

// Close closes both the zstd decoder and the underlying reader.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	// Close zstd decoder
	r.zr.Close()

	// Close underlying reader
	return r.closer.Close()
}

// Ensure Reader implements io.ReadCloser
var _ io.ReadCloser = (*Reader)(nil)
