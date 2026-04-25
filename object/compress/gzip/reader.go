package gzip

import (
	"compress/gzip"
	"io"
	"sync"
)

// Reader wraps an io.ReadCloser with gzip decompression.
type Reader struct {
	gr     *gzip.Reader
	closer io.Closer
	closed bool
	mu     sync.Mutex
}

// NewReader creates a new gzip reader that decompresses data from the underlying reader.
func NewReader(r io.ReadCloser) (*Reader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &Reader{
		gr:     gr,
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

	return r.gr.Read(p)
}

// Close closes both the gzip reader and the underlying reader.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	// Close gzip reader first
	if err := r.gr.Close(); err != nil {
		_ = r.closer.Close()
		return err
	}

	return r.closer.Close()
}

// Header returns the gzip header.
func (r *Reader) Header() gzip.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gr.Header
}

// Ensure Reader implements io.ReadCloser
var _ io.ReadCloser = (*Reader)(nil)
