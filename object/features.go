package object

// Features describes the capabilities of a backend.
// Use this to check what operations are supported before calling them,
// or to select optimal code paths.
type Features struct {
	// Copy indicates the backend supports server-side copy.
	// When true, Copy() is efficient (no data transfer through client).
	// When false, use CopyPath() helper which streams through the client.
	Copy bool

	// Move indicates the backend supports server-side move/rename.
	// When true, Move() is efficient (no data transfer).
	// When false, use MovePath() helper which copies then deletes.
	Move bool

	// Mkdir indicates the backend supports creating directories.
	// Object stores (S3, GCS) typically don't need this (directories are implicit).
	// Filesystems and some cloud drives require explicit directory creation.
	Mkdir bool

	// Rmdir indicates the backend supports removing directories.
	Rmdir bool

	// Stat indicates the backend supports getting object metadata.
	// When true, Stat() returns size, modtime, hashes, etc.
	Stat bool

	// Hashes lists the hash types supported by this backend.
	// Use this to determine which hash to request from ObjectInfo.Hash().
	Hashes []HashType

	// CanStream indicates the backend supports streaming writes.
	// When true, data can be written incrementally.
	// When false, the entire content must be buffered before upload.
	CanStream bool

	// ServerSideEncryption indicates the backend supports server-side encryption.
	ServerSideEncryption bool

	// Versioning indicates the backend supports object versioning.
	Versioning bool

	// RangeRead indicates the backend supports reading byte ranges.
	// When true, WithOffset() and WithLimit() are efficient.
	RangeRead bool

	// ListPrefix indicates the backend supports efficient prefix listing.
	// When true, List() with a prefix is efficient.
	// When false, the entire tree may be scanned.
	ListPrefix bool

	// SetModTime indicates the backend supports setting modification time.
	// When true, modification time can be preserved during copy/sync.
	SetModTime bool

	// CustomMetadata indicates the backend supports custom metadata.
	// When true, arbitrary key-value metadata can be stored with objects.
	CustomMetadata bool
}

// SupportsHash returns true if the backend supports the given hash type.
func (f Features) SupportsHash(t HashType) bool {
	for _, h := range f.Hashes {
		if h == t {
			return true
		}
	}
	return false
}

// PreferredHash returns the preferred hash type for this backend.
// Returns HashNone if no hashes are supported.
// Preference order: SHA256, SHA1, MD5, CRC32C.
func (f Features) PreferredHash() HashType {
	preferenceOrder := []HashType{HashSHA256, HashSHA1, HashMD5, HashCRC32C}
	for _, t := range preferenceOrder {
		if f.SupportsHash(t) {
			return t
		}
	}
	return HashNone
}

// CommonHash returns a hash type supported by both feature sets.
// Returns HashNone if no common hash type exists.
func (f Features) CommonHash(other Features) HashType {
	preferenceOrder := []HashType{HashSHA256, HashSHA1, HashMD5, HashCRC32C}
	for _, t := range preferenceOrder {
		if f.SupportsHash(t) && other.SupportsHash(t) {
			return t
		}
	}
	return HashNone
}
