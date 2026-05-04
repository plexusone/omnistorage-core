// Package omnistorage provides a lightweight storage abstraction layer for Go.
//
// This package re-exports the core object storage interfaces and automatically
// registers all built-in backends (file, memory, channel, sftp, dropbox).
//
// For cloud storage backends (S3, GCS, GitHub, Google Drive), use
// github.com/plexusone/omnistorage which includes both core and cloud backends.
//
// Usage:
//
//	import "github.com/plexusone/omnistorage-core"
//
//	backend, _ := omnistorage.Open("file", map[string]string{
//	    "root": "/path/to/storage",
//	})
//
// Available backends:
//   - file: Local filesystem storage
//   - memory: In-memory storage (for testing)
//   - channel: In-process channel-based storage
//   - sftp: SSH file transfer protocol
//   - dropbox: Dropbox cloud storage
package omnistorage

import (
	// Re-export core object storage types
	"github.com/plexusone/omnistorage-core/object"

	// Import backends for init() registration
	_ "github.com/plexusone/omnistorage-core/object/backend/channel"
	_ "github.com/plexusone/omnistorage-core/object/backend/dropbox"
	_ "github.com/plexusone/omnistorage-core/object/backend/file"
	_ "github.com/plexusone/omnistorage-core/object/backend/memory"
	_ "github.com/plexusone/omnistorage-core/object/backend/sftp"
)

// Re-export core object storage types.
type (
	// Backend represents a storage backend (file, memory, sftp, etc.).
	Backend = object.Backend

	// ExtendedBackend extends Backend with additional operations.
	ExtendedBackend = object.ExtendedBackend

	// RecordWriter writes framed records to an underlying writer.
	RecordWriter = object.RecordWriter

	// RecordReader reads framed records from an underlying reader.
	RecordReader = object.RecordReader

	// ObjectInfo contains metadata about a stored object.
	ObjectInfo = object.ObjectInfo

	// Features describes capabilities of a backend.
	Features = object.Features

	// HashType identifies a hash algorithm.
	HashType = object.HashType

	// WriterOption configures a writer.
	WriterOption = object.WriterOption

	// ReaderOption configures a reader.
	ReaderOption = object.ReaderOption
)

// Re-export core functions.
var (
	// Register registers a backend factory.
	Register = object.Register

	// Open creates a backend from the registry.
	Open = object.Open

	// WithContentType sets the content type for a writer.
	WithContentType = object.WithContentType

	// WithMetadata sets metadata for a writer.
	WithMetadata = object.WithMetadata

	// WithOffset sets the read offset.
	WithOffset = object.WithOffset

	// WithLimit sets the read limit.
	WithLimit = object.WithLimit

	// ApplyWriterOptions applies writer options.
	ApplyWriterOptions = object.ApplyWriterOptions

	// ApplyReaderOptions applies reader options.
	ApplyReaderOptions = object.ApplyReaderOptions

	// Backends returns a sorted list of registered backend names.
	Backends = object.Backends
)

// Re-export core errors.
var (
	ErrNotFound         = object.ErrNotFound
	ErrPermissionDenied = object.ErrPermissionDenied
	ErrBackendClosed    = object.ErrBackendClosed
	ErrNotSupported     = object.ErrNotSupported
	ErrInvalidPath      = object.ErrInvalidPath
	ErrWriterClosed     = object.ErrWriterClosed
)

// Re-export hash types.
const (
	HashMD5    = object.HashMD5
	HashSHA1   = object.HashSHA1
	HashSHA256 = object.HashSHA256
	HashCRC32C = object.HashCRC32C
)
