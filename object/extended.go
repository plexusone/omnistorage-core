package object

import "context"

// ExtendedBackend extends Backend with additional operations for
// metadata access, directory management, and server-side operations.
//
// Not all backends support all operations. Use Features() to check
// which operations are available, or check for specific errors.
//
// Applications that only need basic read/write operations should use
// the Backend interface for broader compatibility.
type ExtendedBackend interface {
	Backend

	// Stat returns metadata about an object.
	// Returns ErrNotFound if the path does not exist.
	// Returns ErrNotSupported if the backend doesn't support Stat.
	Stat(ctx context.Context, path string) (ObjectInfo, error)

	// Mkdir creates a directory at the given path.
	// Creates parent directories as needed (like mkdir -p).
	// Returns nil if the directory already exists.
	// Returns ErrNotSupported for backends that don't need directories (S3, GCS).
	Mkdir(ctx context.Context, path string) error

	// Rmdir removes an empty directory.
	// Returns ErrNotFound if the directory does not exist.
	// Returns an error if the directory is not empty.
	// Returns ErrNotSupported for backends that don't need directories.
	Rmdir(ctx context.Context, path string) error

	// Copy copies an object from src to dst within the same backend.
	// This is a server-side copy when supported (no data transfer through client).
	// Returns ErrNotFound if src does not exist.
	// Returns ErrNotSupported if server-side copy is not available.
	// Use CopyPath() helper for a fallback that works with any backend.
	Copy(ctx context.Context, src, dst string) error

	// Move moves/renames an object from src to dst within the same backend.
	// This is a server-side move when supported (no data transfer).
	// Returns ErrNotFound if src does not exist.
	// Returns ErrNotSupported if server-side move is not available.
	// Use MovePath() helper for a fallback that works with any backend.
	Move(ctx context.Context, src, dst string) error

	// Features returns the capabilities of this backend.
	// Use this to check which operations are supported before calling them,
	// or to select optimal code paths.
	Features() Features
}

// AsExtended attempts to convert a Backend to ExtendedBackend.
// Returns the ExtendedBackend and true if the backend supports extended operations.
// Returns nil and false otherwise.
func AsExtended(b Backend) (ExtendedBackend, bool) {
	ext, ok := b.(ExtendedBackend)
	return ext, ok
}

// MustExtended converts a Backend to ExtendedBackend or panics.
// Use this when you know the backend supports extended operations.
func MustExtended(b Backend) ExtendedBackend {
	ext, ok := b.(ExtendedBackend)
	if !ok {
		panic("omnistorage: backend does not implement ExtendedBackend")
	}
	return ext
}
