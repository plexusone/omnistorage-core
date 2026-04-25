package object

import "time"

// ObjectInfo provides metadata about a stored object.
// Not all backends support all fields; check Features() for capabilities.
type ObjectInfo interface {
	// Path returns the object's path relative to the backend root.
	Path() string

	// Size returns the object's size in bytes.
	// Returns -1 if the size is unknown.
	Size() int64

	// ModTime returns the object's last modification time.
	// Returns zero time if unknown.
	ModTime() time.Time

	// IsDir returns true if this object represents a directory.
	IsDir() bool

	// ContentType returns the MIME type of the object.
	// Returns empty string if unknown.
	ContentType() string

	// Hash returns the object's hash for the given hash type.
	// Returns empty string if the hash type is not available.
	// Use Features().Hashes to check which hash types are supported.
	Hash(HashType) string

	// Metadata returns custom metadata associated with the object.
	// Returns nil if no custom metadata or backend doesn't support it.
	// Use Features().CustomMetadata to check if supported.
	Metadata() map[string]string
}

// BasicObjectInfo is a simple implementation of ObjectInfo.
// Use this when creating ObjectInfo instances in backend implementations.
type BasicObjectInfo struct {
	ObjectPath        string
	ObjectSize        int64
	ObjectModTime     time.Time
	ObjectIsDir       bool
	ObjectContentType string
	ObjectHashes      map[HashType]string
	ObjectMetadata    map[string]string
}

// Path returns the object's path.
func (o *BasicObjectInfo) Path() string {
	return o.ObjectPath
}

// Size returns the object's size in bytes.
func (o *BasicObjectInfo) Size() int64 {
	return o.ObjectSize
}

// ModTime returns the object's last modification time.
func (o *BasicObjectInfo) ModTime() time.Time {
	return o.ObjectModTime
}

// IsDir returns true if this object represents a directory.
func (o *BasicObjectInfo) IsDir() bool {
	return o.ObjectIsDir
}

// ContentType returns the MIME type of the object.
func (o *BasicObjectInfo) ContentType() string {
	return o.ObjectContentType
}

// Hash returns the object's hash for the given hash type.
func (o *BasicObjectInfo) Hash(t HashType) string {
	if o.ObjectHashes == nil {
		return ""
	}
	return o.ObjectHashes[t]
}

// Metadata returns the object's custom metadata.
func (o *BasicObjectInfo) Metadata() map[string]string {
	return o.ObjectMetadata
}

// Ensure BasicObjectInfo implements ObjectInfo
var _ ObjectInfo = (*BasicObjectInfo)(nil)
