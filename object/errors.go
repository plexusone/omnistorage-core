package object

import "errors"

// Common errors returned by omnistorage backends and utilities.
var (
	// ErrNotFound is returned when a path does not exist.
	ErrNotFound = errors.New("omnistorage: not found")

	// ErrAlreadyExists is returned when attempting to create a path that already exists,
	// if the backend does not support overwriting.
	ErrAlreadyExists = errors.New("omnistorage: already exists")

	// ErrPermissionDenied is returned when access to a path is denied.
	ErrPermissionDenied = errors.New("omnistorage: permission denied")

	// ErrBackendClosed is returned when operating on a closed backend.
	ErrBackendClosed = errors.New("omnistorage: backend closed")

	// ErrWriterClosed is returned when writing to a closed writer.
	ErrWriterClosed = errors.New("omnistorage: writer closed")

	// ErrReaderClosed is returned when reading from a closed reader.
	ErrReaderClosed = errors.New("omnistorage: reader closed")

	// ErrInvalidPath is returned when a path is invalid (e.g., contains forbidden characters).
	ErrInvalidPath = errors.New("omnistorage: invalid path")

	// ErrNotSupported is returned when an operation is not supported by the backend.
	ErrNotSupported = errors.New("omnistorage: operation not supported")

	// ErrUnknownBackend is returned by Open when the backend name is not registered.
	ErrUnknownBackend = errors.New("omnistorage: unknown backend")
)

// IsNotFound returns true if the error indicates a path was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsPermissionDenied returns true if the error indicates permission was denied.
func IsPermissionDenied(err error) bool {
	return errors.Is(err, ErrPermissionDenied)
}

// IsNotSupported returns true if the error indicates an unsupported operation.
func IsNotSupported(err error) bool {
	return errors.Is(err, ErrNotSupported)
}
