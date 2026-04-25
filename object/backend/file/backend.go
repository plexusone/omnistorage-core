// Package file provides a local filesystem backend for omnistorage.
package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

func init() {
	omnistorage.Register("file", NewFromConfig)
}

// Config holds configuration for the file backend.
type Config struct {
	// Root is the root directory for all operations.
	// All paths are relative to this directory.
	Root string

	// CreateDirs controls whether parent directories are created automatically.
	// Default: true
	CreateDirs bool

	// DirPermissions is the permission mode for created directories.
	// Default: 0755
	DirPermissions os.FileMode

	// FilePermissions is the permission mode for created files.
	// Default: 0644
	FilePermissions os.FileMode
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Root:            ".",
		CreateDirs:      true,
		DirPermissions:  0755,
		FilePermissions: 0644,
	}
}

// Backend implements omnistorage.Backend for local filesystem.
type Backend struct {
	config Config
	closed bool
	mu     sync.RWMutex
}

// New creates a new file backend with the given configuration.
func New(config Config) *Backend {
	if config.Root == "" {
		config.Root = "."
	}
	if config.DirPermissions == 0 {
		config.DirPermissions = 0755
	}
	if config.FilePermissions == 0 {
		config.FilePermissions = 0644
	}
	return &Backend{
		config: config,
	}
}

// NewFromConfig creates a new file backend from a config map.
// Supported keys:
//   - root: root directory (default: ".")
//   - create_dirs: "true" or "false" (default: "true")
func NewFromConfig(configMap map[string]string) (omnistorage.Backend, error) {
	config := DefaultConfig()

	if root, ok := configMap["root"]; ok && root != "" {
		config.Root = root
	}

	if createDirs, ok := configMap["create_dirs"]; ok {
		config.CreateDirs = createDirs != "false"
	}

	return New(config), nil
}

// NewWriter creates a writer for the given path.
func (b *Backend) NewWriter(ctx context.Context, path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := b.validatePath(path); err != nil {
		return nil, err
	}

	fullPath := b.fullPath(path)

	// Create parent directories if configured
	if b.config.CreateDirs {
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, b.config.DirPermissions); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, b.config.FilePermissions)
	if err != nil {
		return nil, fmt.Errorf("creating file %s: %w", path, err)
	}

	return f, nil
}

// NewReader creates a reader for the given path.
func (b *Backend) NewReader(ctx context.Context, path string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := b.validatePath(path); err != nil {
		return nil, err
	}

	fullPath := b.fullPath(path)

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, omnistorage.ErrNotFound
		}
		if os.IsPermission(err) {
			return nil, omnistorage.ErrPermissionDenied
		}
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}

	// Apply options
	config := omnistorage.ApplyReaderOptions(opts...)

	// Handle offset if specified
	if config.Offset > 0 {
		if _, err := f.Seek(config.Offset, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("seeking to offset %d: %w", config.Offset, err)
		}
	}

	// Handle limit if specified
	if config.Limit > 0 {
		return &limitedReadCloser{
			r:      io.LimitReader(f, config.Limit),
			closer: f,
		}, nil
	}

	return f, nil
}

// Exists checks if a path exists.
func (b *Backend) Exists(ctx context.Context, path string) (bool, error) {
	if err := b.checkClosed(); err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	if err := b.validatePath(path); err != nil {
		return false, err
	}

	fullPath := b.fullPath(path)

	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking existence of %s: %w", path, err)
}

// Delete removes a path.
func (b *Backend) Delete(ctx context.Context, path string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := b.validatePath(path); err != nil {
		return err
	}

	fullPath := b.fullPath(path)

	err := os.Remove(fullPath)
	if err == nil || os.IsNotExist(err) {
		return nil // Idempotent
	}
	if os.IsPermission(err) {
		return omnistorage.ErrPermissionDenied
	}
	return fmt.Errorf("deleting %s: %w", path, err)
}

// List lists paths with the given prefix.
func (b *Backend) List(ctx context.Context, prefix string) ([]string, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var paths []string

	root := b.config.Root
	if prefix != "" {
		root = b.fullPath(prefix)
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Check context on each iteration
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			// Skip permission errors, return others
			if os.IsPermission(err) {
				return nil
			}
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		rel, err := filepath.Rel(b.config.Root, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for consistency
		rel = filepath.ToSlash(rel)

		paths = append(paths, rel)
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("listing %s: %w", prefix, err)
	}

	return paths, nil
}

// Close releases any resources held by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// fullPath returns the full filesystem path for a relative path.
func (b *Backend) fullPath(path string) string {
	// Convert forward slashes to OS-specific separator
	path = filepath.FromSlash(path)
	return filepath.Join(b.config.Root, path)
}

// validatePath checks if a path is valid.
func (b *Backend) validatePath(path string) error {
	if path == "" {
		return omnistorage.ErrInvalidPath
	}

	// Check for path traversal attempts
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../") {
		return omnistorage.ErrInvalidPath
	}

	return nil
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

// limitedReadCloser wraps a limited reader with a closer.
type limitedReadCloser struct {
	r      io.Reader
	closer io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (n int, err error) {
	return l.r.Read(p)
}

func (l *limitedReadCloser) Close() error {
	return l.closer.Close()
}

// Ensure Backend implements omnistorage.Backend
var _ omnistorage.Backend = (*Backend)(nil)
