package file

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

// Stat returns metadata about an object at the given path.
func (b *Backend) Stat(ctx context.Context, path string) (omnistorage.ObjectInfo, error) {
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

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, omnistorage.ErrNotFound
		}
		if os.IsPermission(err) {
			return nil, omnistorage.ErrPermissionDenied
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	// Determine content type from extension
	contentType := ""
	if !info.IsDir() {
		ext := filepath.Ext(path)
		if ext != "" {
			contentType = mime.TypeByExtension(ext)
		}
	}

	return &omnistorage.BasicObjectInfo{
		ObjectPath:        path,
		ObjectSize:        info.Size(),
		ObjectModTime:     info.ModTime(),
		ObjectIsDir:       info.IsDir(),
		ObjectContentType: contentType,
		ObjectHashes:      nil, // File backend doesn't store hashes; compute on demand if needed
	}, nil
}

// Mkdir creates a directory at the given path.
func (b *Backend) Mkdir(ctx context.Context, path string) error {
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

	err := os.MkdirAll(fullPath, b.config.DirPermissions)
	if err != nil {
		if os.IsPermission(err) {
			return omnistorage.ErrPermissionDenied
		}
		return fmt.Errorf("mkdir %s: %w", path, err)
	}

	return nil
}

// Rmdir removes an empty directory at the given path.
func (b *Backend) Rmdir(ctx context.Context, path string) error {
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

	// Check if path exists and is a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return omnistorage.ErrNotFound
		}
		if os.IsPermission(err) {
			return omnistorage.ErrPermissionDenied
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("rmdir %s: not a directory", path)
	}

	err = os.Remove(fullPath)
	if err != nil {
		if os.IsPermission(err) {
			return omnistorage.ErrPermissionDenied
		}
		return fmt.Errorf("rmdir %s: %w", path, err)
	}

	return nil
}

// Copy copies an object from src to dst using server-side copy (hard link or copy).
func (b *Backend) Copy(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := b.validatePath(src); err != nil {
		return err
	}
	if err := b.validatePath(dst); err != nil {
		return err
	}

	srcPath := b.fullPath(src)
	dstPath := b.fullPath(dst)

	// Check source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return omnistorage.ErrNotFound
		}
		if os.IsPermission(err) {
			return omnistorage.ErrPermissionDenied
		}
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("copy %s: source is a directory", src)
	}

	// Create parent directories if configured
	if b.config.CreateDirs {
		dir := filepath.Dir(dstPath)
		if err := os.MkdirAll(dir, b.config.DirPermissions); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Perform the copy
	return b.copyFile(srcPath, dstPath)
}

// Move moves/renames an object from src to dst.
func (b *Backend) Move(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := b.validatePath(src); err != nil {
		return err
	}
	if err := b.validatePath(dst); err != nil {
		return err
	}

	srcPath := b.fullPath(src)
	dstPath := b.fullPath(dst)

	// Check source exists
	_, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return omnistorage.ErrNotFound
		}
		if os.IsPermission(err) {
			return omnistorage.ErrPermissionDenied
		}
		return fmt.Errorf("stat %s: %w", src, err)
	}

	// Create parent directories if configured
	if b.config.CreateDirs {
		dir := filepath.Dir(dstPath)
		if err := os.MkdirAll(dir, b.config.DirPermissions); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Try rename first (atomic, same filesystem)
	err = os.Rename(srcPath, dstPath)
	if err == nil {
		return nil
	}

	// If rename fails (cross-filesystem), fall back to copy+delete
	if err := b.copyFile(srcPath, dstPath); err != nil {
		return err
	}

	return os.Remove(srcPath)
}

// Features returns the capabilities of the file backend.
func (b *Backend) Features() omnistorage.Features {
	return omnistorage.Features{
		Copy:                 true,
		Move:                 true,
		Mkdir:                true,
		Rmdir:                true,
		Stat:                 true,
		Hashes:               []omnistorage.HashType{}, // Hashes computed on demand, not stored
		CanStream:            true,
		ServerSideEncryption: false,
		Versioning:           false,
		RangeRead:            true,
		ListPrefix:           true,
	}
}

// copyFile copies a file from src to dst.
func (b *Backend) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, b.config.FilePermissions)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return dstFile.Close()
}

// Ensure Backend implements omnistorage.ExtendedBackend
var _ omnistorage.ExtendedBackend = (*Backend)(nil)
