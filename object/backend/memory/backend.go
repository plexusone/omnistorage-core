// Package memory provides an in-memory backend for omnistorage.
//
// The memory backend is useful for:
//   - Unit testing without filesystem access
//   - Temporary storage and caching
//   - Development and prototyping
//   - Fast ephemeral storage
//
// Data is stored in RAM and lost when the backend is closed or the process exits.
package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

func init() {
	omnistorage.Register("memory", NewFromConfig)
}

// object represents a stored object in memory.
type object struct {
	data        []byte
	contentType string
	modTime     time.Time
	isDir       bool
}

// Backend implements omnistorage.ExtendedBackend for in-memory storage.
type Backend struct {
	objects map[string]*object
	closed  bool
	mu      sync.RWMutex
}

// New creates a new memory backend.
func New() *Backend {
	return &Backend{
		objects: make(map[string]*object),
	}
}

// NewFromConfig creates a new memory backend from a config map.
// The memory backend ignores all configuration options.
func NewFromConfig(_ map[string]string) (omnistorage.Backend, error) {
	return New(), nil
}

// NewWriter creates a writer for the given path.
func (b *Backend) NewWriter(ctx context.Context, p string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := validatePath(p); err != nil {
		return nil, err
	}

	config := omnistorage.ApplyWriterOptions(opts...)

	return &memoryWriter{
		backend:     b,
		path:        normalizePath(p),
		buffer:      &bytes.Buffer{},
		contentType: config.ContentType,
	}, nil
}

// NewReader creates a reader for the given path.
func (b *Backend) NewReader(ctx context.Context, p string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := validatePath(p); err != nil {
		return nil, err
	}

	normalPath := normalizePath(p)

	b.mu.RLock()
	obj, exists := b.objects[normalPath]
	b.mu.RUnlock()

	if !exists {
		return nil, omnistorage.ErrNotFound
	}

	if obj.isDir {
		return nil, fmt.Errorf("cannot read directory: %s", p)
	}

	config := omnistorage.ApplyReaderOptions(opts...)

	// Make a copy of the data to avoid race conditions
	data := make([]byte, len(obj.data))
	copy(data, obj.data)

	// Apply offset
	if config.Offset > 0 {
		if config.Offset >= int64(len(data)) {
			data = []byte{}
		} else {
			data = data[config.Offset:]
		}
	}

	// Apply limit
	if config.Limit > 0 && int64(len(data)) > config.Limit {
		data = data[:config.Limit]
	}

	return &memoryReader{
		reader: bytes.NewReader(data),
	}, nil
}

// Exists checks if a path exists.
func (b *Backend) Exists(ctx context.Context, p string) (bool, error) {
	if err := b.checkClosed(); err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	if err := validatePath(p); err != nil {
		return false, err
	}

	normalPath := normalizePath(p)

	b.mu.RLock()
	_, exists := b.objects[normalPath]
	b.mu.RUnlock()

	return exists, nil
}

// Delete removes a path.
func (b *Backend) Delete(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validatePath(p); err != nil {
		return err
	}

	normalPath := normalizePath(p)

	b.mu.Lock()
	delete(b.objects, normalPath)
	b.mu.Unlock()

	return nil
}

// List lists paths with the given prefix.
func (b *Backend) List(ctx context.Context, prefix string) ([]string, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalPrefix := normalizePath(prefix)

	b.mu.RLock()
	defer b.mu.RUnlock()

	var paths []string
	for p, obj := range b.objects {
		// Check context periodically
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Skip directories in listing (only list files)
		if obj.isDir {
			continue
		}

		// Match prefix
		if normalPrefix == "" || strings.HasPrefix(p, normalPrefix) || strings.HasPrefix(p, normalPrefix+"/") {
			paths = append(paths, p)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

// Close releases any resources held by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	b.objects = nil
	return nil
}

// Stat returns metadata about an object at the given path.
func (b *Backend) Stat(ctx context.Context, p string) (omnistorage.ObjectInfo, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := validatePath(p); err != nil {
		return nil, err
	}

	normalPath := normalizePath(p)

	b.mu.RLock()
	obj, exists := b.objects[normalPath]
	b.mu.RUnlock()

	if !exists {
		return nil, omnistorage.ErrNotFound
	}

	size := int64(len(obj.data))
	if obj.isDir {
		size = 0
	}

	return &omnistorage.BasicObjectInfo{
		ObjectPath:        normalPath,
		ObjectSize:        size,
		ObjectModTime:     obj.modTime,
		ObjectIsDir:       obj.isDir,
		ObjectContentType: obj.contentType,
	}, nil
}

// Mkdir creates a directory at the given path.
func (b *Backend) Mkdir(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validatePath(p); err != nil {
		return err
	}

	normalPath := normalizePath(p)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Create all parent directories
	parts := strings.Split(normalPath, "/")
	for i := range parts {
		dirPath := strings.Join(parts[:i+1], "/")
		if _, exists := b.objects[dirPath]; !exists {
			b.objects[dirPath] = &object{
				isDir:   true,
				modTime: time.Now(),
			}
		}
	}

	return nil
}

// Rmdir removes an empty directory.
func (b *Backend) Rmdir(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validatePath(p); err != nil {
		return err
	}

	normalPath := normalizePath(p)

	b.mu.Lock()
	defer b.mu.Unlock()

	obj, exists := b.objects[normalPath]
	if !exists {
		return omnistorage.ErrNotFound
	}

	if !obj.isDir {
		return fmt.Errorf("not a directory: %s", p)
	}

	// Check if directory is empty
	prefix := normalPath + "/"
	for objPath := range b.objects {
		if strings.HasPrefix(objPath, prefix) {
			return fmt.Errorf("directory not empty: %s", p)
		}
	}

	delete(b.objects, normalPath)
	return nil
}

// Copy copies an object from src to dst.
func (b *Backend) Copy(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validatePath(src); err != nil {
		return err
	}
	if err := validatePath(dst); err != nil {
		return err
	}

	srcPath := normalizePath(src)
	dstPath := normalizePath(dst)

	b.mu.Lock()
	defer b.mu.Unlock()

	srcObj, exists := b.objects[srcPath]
	if !exists {
		return omnistorage.ErrNotFound
	}

	if srcObj.isDir {
		return fmt.Errorf("cannot copy directory: %s", src)
	}

	// Create a copy of the data
	dataCopy := make([]byte, len(srcObj.data))
	copy(dataCopy, srcObj.data)

	b.objects[dstPath] = &object{
		data:        dataCopy,
		contentType: srcObj.contentType,
		modTime:     time.Now(),
		isDir:       false,
	}

	return nil
}

// Move moves/renames an object from src to dst.
func (b *Backend) Move(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validatePath(src); err != nil {
		return err
	}
	if err := validatePath(dst); err != nil {
		return err
	}

	srcPath := normalizePath(src)
	dstPath := normalizePath(dst)

	b.mu.Lock()
	defer b.mu.Unlock()

	srcObj, exists := b.objects[srcPath]
	if !exists {
		return omnistorage.ErrNotFound
	}

	if srcObj.isDir {
		return fmt.Errorf("cannot move directory: %s", src)
	}

	// Move the object
	b.objects[dstPath] = &object{
		data:        srcObj.data,
		contentType: srcObj.contentType,
		modTime:     time.Now(),
		isDir:       false,
	}
	delete(b.objects, srcPath)

	return nil
}

// Features returns the capabilities of the memory backend.
func (b *Backend) Features() omnistorage.Features {
	return omnistorage.Features{
		Copy:                 true,
		Move:                 true,
		Mkdir:                true,
		Rmdir:                true,
		Stat:                 true,
		Hashes:               []omnistorage.HashType{}, // Can compute on demand
		CanStream:            true,
		ServerSideEncryption: false,
		Versioning:           false,
		RangeRead:            true,
		ListPrefix:           true,
	}
}

// Size returns the total size of all objects in the backend.
// This is useful for monitoring memory usage.
func (b *Backend) Size() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var total int64
	for _, obj := range b.objects {
		total += int64(len(obj.data))
	}
	return total
}

// Count returns the number of objects in the backend.
func (b *Backend) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	for _, obj := range b.objects {
		if !obj.isDir {
			count++
		}
	}
	return count
}

// Clear removes all objects from the backend.
func (b *Backend) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.objects = make(map[string]*object)
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

// validatePath checks if a path is valid.
func validatePath(p string) error {
	if p == "" {
		return omnistorage.ErrInvalidPath
	}

	// Check for path traversal
	cleaned := path.Clean(p)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/../") {
		return omnistorage.ErrInvalidPath
	}

	return nil
}

// normalizePath normalizes a path for consistent storage.
func normalizePath(p string) string {
	if p == "" {
		return ""
	}
	// Clean the path
	p = path.Clean(p)
	// Remove leading slash
	p = strings.TrimPrefix(p, "/")
	// path.Clean("") returns ".", convert back to ""
	if p == "." {
		return ""
	}
	return p
}

// memoryWriter implements io.WriteCloser for memory backend.
type memoryWriter struct {
	backend     *Backend
	path        string
	buffer      *bytes.Buffer
	contentType string
	closed      bool
	mu          sync.Mutex
}

func (w *memoryWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, omnistorage.ErrWriterClosed
	}

	return w.buffer.Write(p)
}

func (w *memoryWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Store the data in the backend
	w.backend.mu.Lock()
	defer w.backend.mu.Unlock()

	if w.backend.closed {
		return omnistorage.ErrBackendClosed
	}

	w.backend.objects[w.path] = &object{
		data:        w.buffer.Bytes(),
		contentType: w.contentType,
		modTime:     time.Now(),
		isDir:       false,
	}

	return nil
}

// memoryReader implements io.ReadCloser for memory backend.
type memoryReader struct {
	reader *bytes.Reader
	closed bool
	mu     sync.Mutex
}

func (r *memoryReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, omnistorage.ErrReaderClosed
	}

	return r.reader.Read(p)
}

func (r *memoryReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	return nil
}

// Ensure Backend implements omnistorage.ExtendedBackend
var _ omnistorage.ExtendedBackend = (*Backend)(nil)
