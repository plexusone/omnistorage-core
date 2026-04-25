package object

import (
	"crypto/md5"  //nolint:gosec // MD5 used for content verification, not security
	"crypto/sha1" //nolint:gosec // SHA1 used for content verification, not security
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"hash/crc32"
	"io"
)

// HashType represents a hash algorithm used for content verification.
type HashType string

const (
	// HashNone indicates no hash.
	HashNone HashType = ""

	// HashMD5 is the MD5 hash algorithm.
	// Supported by: S3, GCS, Azure, most backends.
	HashMD5 HashType = "md5"

	// HashSHA1 is the SHA-1 hash algorithm.
	// Supported by: GCS, Dropbox.
	HashSHA1 HashType = "sha1"

	// HashSHA256 is the SHA-256 hash algorithm.
	// Supported by: S3 (as x-amz-checksum-sha256).
	HashSHA256 HashType = "sha256"

	// HashCRC32C is the CRC32C checksum.
	// Supported by: GCS.
	HashCRC32C HashType = "crc32c"
)

// String returns the string representation of the hash type.
func (h HashType) String() string {
	return string(h)
}

// SupportedHashes returns all supported hash types.
func SupportedHashes() []HashType {
	return []HashType{HashMD5, HashSHA1, HashSHA256, HashCRC32C}
}

// NewHash creates a new hash.Hash for the given hash type.
// Returns nil if the hash type is not supported.
func NewHash(t HashType) hash.Hash {
	switch t {
	case HashMD5:
		return md5.New() //nolint:gosec // MD5 used for content verification
	case HashSHA1:
		return sha1.New() //nolint:gosec // SHA1 used for content verification
	case HashSHA256:
		return sha256.New()
	case HashCRC32C:
		return crc32.New(crc32.MakeTable(crc32.Castagnoli))
	default:
		return nil
	}
}

// HashReader computes the hash of data from a reader.
// Returns the hex-encoded hash string.
func HashReader(r io.Reader, t HashType) (string, error) {
	h := NewHash(t)
	if h == nil {
		return "", ErrNotSupported
	}

	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashBytes computes the hash of a byte slice.
// Returns the hex-encoded hash string.
func HashBytes(data []byte, t HashType) string {
	h := NewHash(t)
	if h == nil {
		return ""
	}

	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// HashSet holds multiple hash values for an object.
type HashSet map[HashType]string

// Get returns the hash value for the given type, or empty string if not present.
func (hs HashSet) Get(t HashType) string {
	if hs == nil {
		return ""
	}
	return hs[t]
}

// Set sets a hash value.
func (hs HashSet) Set(t HashType, value string) {
	if hs != nil {
		hs[t] = value
	}
}

// Has returns true if the hash set contains the given hash type.
func (hs HashSet) Has(t HashType) bool {
	if hs == nil {
		return false
	}
	_, ok := hs[t]
	return ok
}

// Equal compares two hash sets for equality on common hash types.
// Returns true if at least one common hash type matches.
// Returns false if no common hash types exist or if any common hash differs.
func (hs HashSet) Equal(other HashSet) bool {
	if hs == nil || other == nil {
		return false
	}

	foundCommon := false
	for t, v := range hs {
		if ov, ok := other[t]; ok {
			foundCommon = true
			if v != ov {
				return false
			}
		}
	}

	return foundCommon
}
