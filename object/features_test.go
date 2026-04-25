package object

import "testing"

func TestFeaturesSupportsHash(t *testing.T) {
	f := Features{
		Hashes: []HashType{HashMD5, HashSHA256},
	}

	if !f.SupportsHash(HashMD5) {
		t.Error("SupportsHash(MD5) = false, want true")
	}
	if !f.SupportsHash(HashSHA256) {
		t.Error("SupportsHash(SHA256) = false, want true")
	}
	if f.SupportsHash(HashSHA1) {
		t.Error("SupportsHash(SHA1) = true, want false")
	}
}

func TestFeaturesSupportsHashEmpty(t *testing.T) {
	f := Features{}

	if f.SupportsHash(HashMD5) {
		t.Error("SupportsHash(MD5) with empty Hashes = true, want false")
	}
}

func TestFeaturesPreferredHash(t *testing.T) {
	tests := []struct {
		name     string
		hashes   []HashType
		expected HashType
	}{
		{
			name:     "prefers SHA256",
			hashes:   []HashType{HashMD5, HashSHA1, HashSHA256},
			expected: HashSHA256,
		},
		{
			name:     "prefers SHA1 when no SHA256",
			hashes:   []HashType{HashMD5, HashSHA1, HashCRC32C},
			expected: HashSHA1,
		},
		{
			name:     "prefers MD5 when no SHA",
			hashes:   []HashType{HashMD5, HashCRC32C},
			expected: HashMD5,
		},
		{
			name:     "falls back to CRC32C",
			hashes:   []HashType{HashCRC32C},
			expected: HashCRC32C,
		},
		{
			name:     "returns HashNone when empty",
			hashes:   []HashType{},
			expected: HashNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Features{Hashes: tt.hashes}
			if got := f.PreferredHash(); got != tt.expected {
				t.Errorf("PreferredHash() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestFeaturesCommonHash(t *testing.T) {
	tests := []struct {
		name     string
		f1       Features
		f2       Features
		expected HashType
	}{
		{
			name:     "common SHA256",
			f1:       Features{Hashes: []HashType{HashMD5, HashSHA256}},
			f2:       Features{Hashes: []HashType{HashSHA1, HashSHA256}},
			expected: HashSHA256,
		},
		{
			name:     "common MD5 only",
			f1:       Features{Hashes: []HashType{HashMD5}},
			f2:       Features{Hashes: []HashType{HashMD5, HashSHA1}},
			expected: HashMD5,
		},
		{
			name:     "no common hash",
			f1:       Features{Hashes: []HashType{HashMD5}},
			f2:       Features{Hashes: []HashType{HashSHA1}},
			expected: HashNone,
		},
		{
			name:     "empty hashes",
			f1:       Features{Hashes: []HashType{}},
			f2:       Features{Hashes: []HashType{HashMD5}},
			expected: HashNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f1.CommonHash(tt.f2); got != tt.expected {
				t.Errorf("CommonHash() = %s, want %s", got, tt.expected)
			}
		})
	}
}
