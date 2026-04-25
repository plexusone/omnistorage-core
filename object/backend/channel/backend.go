// Package channel provides a Go channel-based backend for omnistorage.
//
// The channel backend is useful for:
//   - Pipeline processing between goroutines
//   - In-process message passing and streaming
//   - Test fixtures for streaming data
//   - Building data processing pipelines
//
// Each path corresponds to a separate channel. Writers send data to channels,
// and readers receive data from channels.
//
// Example usage:
//
//	backend := channel.New()
//
//	// Producer goroutine
//	go func() {
//	    w, _ := backend.NewWriter(ctx, "events")
//	    w.Write([]byte("event1"))
//	    w.Write([]byte("event2"))
//	    w.Close() // Signals end of stream
//	}()
//
//	// Consumer goroutine
//	r, _ := backend.NewReader(ctx, "events")
//	for {
//	    buf := make([]byte, 1024)
//	    n, err := r.Read(buf)
//	    if err == io.EOF { break }
//	    process(buf[:n])
//	}
package channel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

func init() {
	omnistorage.Register("channel", NewFromConfig)
}

// Message represents a message sent through a channel.
type Message struct {
	Data []byte
	Path string
}

// channelInfo holds state for a single channel/path.
type channelInfo struct {
	ch       chan []byte
	closed   bool
	buffered [][]byte // Buffered data for readers that connect after writes
	mu       sync.RWMutex
}

// Backend implements omnistorage.Backend using Go channels.
type Backend struct {
	channels   map[string]*channelInfo
	bufferSize int  // Channel buffer size
	persistent bool // Whether to buffer data for late readers
	closed     bool
	mu         sync.RWMutex
}

// Option configures a channel backend.
type Option func(*Backend)

// WithBufferSize sets the channel buffer size.
// Default is 100 messages.
func WithBufferSize(size int) Option {
	return func(b *Backend) {
		b.bufferSize = size
	}
}

// WithPersistence enables buffering data for readers that connect after writers.
// When enabled, data written to a channel is stored and replayed to new readers.
// This is useful for testing but uses more memory.
func WithPersistence(enabled bool) Option {
	return func(b *Backend) {
		b.persistent = enabled
	}
}

// New creates a new channel backend with optional configuration.
func New(opts ...Option) *Backend {
	b := &Backend{
		channels:   make(map[string]*channelInfo),
		bufferSize: 100,
		persistent: false,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// NewFromConfig creates a new channel backend from a config map.
// Supported options:
//   - buffer_size: Channel buffer size (default: 100)
//   - persistent: Buffer data for late readers (default: false)
func NewFromConfig(config map[string]string) (omnistorage.Backend, error) {
	var opts []Option

	if sizeStr, ok := config["buffer_size"]; ok {
		var size int
		if _, err := fmt.Sscanf(sizeStr, "%d", &size); err == nil && size > 0 {
			opts = append(opts, WithBufferSize(size))
		}
	}

	if persistStr, ok := config["persistent"]; ok {
		if persistStr == "true" || persistStr == "1" {
			opts = append(opts, WithPersistence(true))
		}
	}

	return New(opts...), nil
}

// getOrCreateChannel gets or creates a channel for the given path.
func (b *Backend) getOrCreateChannel(path string) *channelInfo {
	b.mu.Lock()
	defer b.mu.Unlock()

	if info, exists := b.channels[path]; exists {
		return info
	}

	info := &channelInfo{
		ch: make(chan []byte, b.bufferSize),
	}
	b.channels[path] = info
	return info
}

// NewWriter creates a writer for the given path.
// Data written will be sent to any readers on the same path.
func (b *Backend) NewWriter(ctx context.Context, path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if path == "" {
		return nil, omnistorage.ErrInvalidPath
	}

	info := b.getOrCreateChannel(path)

	return &channelWriter{
		backend: b,
		info:    info,
		path:    path,
		ctx:     ctx,
	}, nil
}

// NewReader creates a reader for the given path.
// It receives data from any writers on the same path.
func (b *Backend) NewReader(ctx context.Context, path string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if path == "" {
		return nil, omnistorage.ErrInvalidPath
	}

	info := b.getOrCreateChannel(path)

	reader := &channelReader{
		backend: b,
		info:    info,
		path:    path,
		ctx:     ctx,
	}

	// If persistent mode, replay buffered data
	if b.persistent {
		info.mu.RLock()
		if len(info.buffered) > 0 {
			// Concatenate all buffered data
			var total int
			for _, d := range info.buffered {
				total += len(d)
			}
			combined := make([]byte, 0, total)
			for _, d := range info.buffered {
				combined = append(combined, d...)
			}
			reader.buffer = bytes.NewBuffer(combined)
		}
		info.mu.RUnlock()
	}

	return reader, nil
}

// Exists checks if a channel exists for the given path.
func (b *Backend) Exists(ctx context.Context, path string) (bool, error) {
	if err := b.checkClosed(); err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	b.mu.RLock()
	_, exists := b.channels[path]
	b.mu.RUnlock()

	return exists, nil
}

// Delete removes a channel for the given path.
func (b *Backend) Delete(ctx context.Context, path string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if info, exists := b.channels[path]; exists {
		info.mu.Lock()
		if !info.closed {
			close(info.ch)
			info.closed = true
		}
		info.mu.Unlock()
		delete(b.channels, path)
	}

	return nil
}

// List returns all channel paths with the given prefix.
func (b *Backend) List(ctx context.Context, prefix string) ([]string, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	var paths []string
	for path := range b.channels {
		if prefix == "" || strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

// Close closes all channels and releases resources.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Close all channels
	for _, info := range b.channels {
		info.mu.Lock()
		if !info.closed {
			close(info.ch)
			info.closed = true
		}
		info.mu.Unlock()
	}

	b.channels = nil
	return nil
}

// Broadcast sends data to all channels matching a prefix.
func (b *Backend) Broadcast(ctx context.Context, prefix string, data []byte) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for path, info := range b.channels {
		if prefix == "" || strings.HasPrefix(path, prefix) {
			info.mu.RLock()
			closed := info.closed
			info.mu.RUnlock()

			if !closed {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case info.ch <- data:
				default:
					// Channel full, skip (non-blocking)
				}
			}
		}
	}

	return nil
}

// ChannelCount returns the number of active channels.
func (b *Backend) ChannelCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.channels)
}

// checkClosed returns an error if the backend is closed.
func (b *Backend) checkClosed() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return omnistorage.ErrBackendClosed
	}
	return nil
}

// channelWriter implements io.WriteCloser for channel backend.
type channelWriter struct {
	backend *Backend
	info    *channelInfo
	path    string
	ctx     context.Context
	closed  bool
	mu      sync.Mutex
}

func (w *channelWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, omnistorage.ErrWriterClosed
	}

	// Check context
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}

	// Make a copy of data
	data := make([]byte, len(p))
	copy(data, p)

	// If persistent mode, buffer the data
	if w.backend.persistent {
		w.info.mu.Lock()
		w.info.buffered = append(w.info.buffered, data)
		w.info.mu.Unlock()
	}

	// Send to channel (non-blocking with select)
	w.info.mu.RLock()
	closed := w.info.closed
	w.info.mu.RUnlock()

	if closed {
		return 0, io.ErrClosedPipe
	}

	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	case w.info.ch <- data:
		return len(p), nil
	}
}

func (w *channelWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Close the channel to signal EOF to readers
	w.info.mu.Lock()
	if !w.info.closed {
		close(w.info.ch)
		w.info.closed = true
	}
	w.info.mu.Unlock()

	return nil
}

// channelReader implements io.ReadCloser for channel backend.
type channelReader struct {
	backend *Backend
	info    *channelInfo
	path    string
	ctx     context.Context
	buffer  *bytes.Buffer // Buffer for persistent mode or partial reads
	current []byte        // Current message being read
	offset  int           // Offset into current message
	closed  bool
	mu      sync.Mutex
}

func (r *channelReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, omnistorage.ErrReaderClosed
	}

	// Check context
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	// If we have a buffer from persistent mode, read from it first
	if r.buffer != nil && r.buffer.Len() > 0 {
		return r.buffer.Read(p)
	}

	// If we have leftover data from a previous message, return it
	if r.offset < len(r.current) {
		n = copy(p, r.current[r.offset:])
		r.offset += n
		return n, nil
	}

	// Get next message from channel
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case data, ok := <-r.info.ch:
		if !ok {
			return 0, io.EOF
		}
		r.current = data
		r.offset = 0
		n = copy(p, r.current)
		r.offset = n
		return n, nil
	}
}

func (r *channelReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	return nil
}

// Ensure Backend implements omnistorage.Backend
var _ omnistorage.Backend = (*Backend)(nil)
