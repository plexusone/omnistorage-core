package object

import (
	"bytes"
	"testing"
)

func TestHashBytes(t *testing.T) {
	data := []byte("hello world")

	tests := []struct {
		hashType HashType
		expected string
	}{
		{HashMD5, "5eb63bbbe01eeed093cb22bb8f5acdc3"},
		{HashSHA1, "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed"},
		{HashSHA256, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
	}

	for _, tt := range tests {
		t.Run(string(tt.hashType), func(t *testing.T) {
			result := HashBytes(data, tt.hashType)
			if result != tt.expected {
				t.Errorf("HashBytes(%s) = %s, want %s", tt.hashType, result, tt.expected)
			}
		})
	}
}

func TestHashBytesUnsupported(t *testing.T) {
	result := HashBytes([]byte("test"), HashType("unsupported"))
	if result != "" {
		t.Errorf("HashBytes with unsupported type = %q, want empty string", result)
	}
}

func TestHashReader(t *testing.T) {
	data := []byte("hello world")
	reader := bytes.NewReader(data)

	result, err := HashReader(reader, HashMD5)
	if err != nil {
		t.Fatalf("HashReader failed: %v", err)
	}

	expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	if result != expected {
		t.Errorf("HashReader(MD5) = %s, want %s", result, expected)
	}
}

func TestHashReaderUnsupported(t *testing.T) {
	reader := bytes.NewReader([]byte("test"))
	_, err := HashReader(reader, HashType("unsupported"))
	if err != ErrNotSupported {
		t.Errorf("HashReader with unsupported type error = %v, want ErrNotSupported", err)
	}
}

func TestNewHash(t *testing.T) {
	for _, ht := range SupportedHashes() {
		h := NewHash(ht)
		if h == nil {
			t.Errorf("NewHash(%s) returned nil", ht)
		}
	}

	// Unsupported hash type should return nil
	if h := NewHash(HashType("unsupported")); h != nil {
		t.Error("NewHash(unsupported) should return nil")
	}
}

func TestHashTypeString(t *testing.T) {
	if HashMD5.String() != "md5" {
		t.Errorf("HashMD5.String() = %q, want %q", HashMD5.String(), "md5")
	}
}

func TestHashSet(t *testing.T) {
	hs := make(HashSet)

	// Test Set and Get
	hs.Set(HashMD5, "abc123")
	if got := hs.Get(HashMD5); got != "abc123" {
		t.Errorf("HashSet.Get(MD5) = %q, want %q", got, "abc123")
	}

	// Test Has
	if !hs.Has(HashMD5) {
		t.Error("HashSet.Has(MD5) = false, want true")
	}
	if hs.Has(HashSHA1) {
		t.Error("HashSet.Has(SHA1) = true, want false")
	}

	// Test Get on missing key
	if got := hs.Get(HashSHA1); got != "" {
		t.Errorf("HashSet.Get(SHA1) = %q, want empty string", got)
	}
}

func TestHashSetNil(t *testing.T) {
	var hs HashSet

	// Nil hash set should not panic
	if got := hs.Get(HashMD5); got != "" {
		t.Errorf("nil HashSet.Get() = %q, want empty string", got)
	}
	if hs.Has(HashMD5) {
		t.Error("nil HashSet.Has() = true, want false")
	}
}

func TestHashSetEqual(t *testing.T) {
	hs1 := HashSet{HashMD5: "abc", HashSHA1: "def"}
	hs2 := HashSet{HashMD5: "abc", HashSHA256: "ghi"}
	hs3 := HashSet{HashMD5: "xyz", HashSHA1: "def"}
	hs4 := HashSet{HashSHA256: "ghi"} // No common with hs1

	// Same hash on common type
	if !hs1.Equal(hs2) {
		t.Error("hs1.Equal(hs2) = false, want true (common MD5 matches)")
	}

	// Different hash on common type
	if hs1.Equal(hs3) {
		t.Error("hs1.Equal(hs3) = true, want false (MD5 differs)")
	}

	// No common hash type
	if hs1.Equal(hs4) {
		t.Error("hs1.Equal(hs4) = true, want false (no common hash type)")
	}

	// Nil hash sets
	var nilHS HashSet
	if hs1.Equal(nilHS) {
		t.Error("hs1.Equal(nil) = true, want false")
	}
	if nilHS.Equal(hs1) {
		t.Error("nil.Equal(hs1) = true, want false")
	}
}

func TestSupportedHashes(t *testing.T) {
	hashes := SupportedHashes()
	if len(hashes) == 0 {
		t.Error("SupportedHashes() returned empty slice")
	}

	// Check expected hash types are present
	expected := map[HashType]bool{
		HashMD5:    false,
		HashSHA1:   false,
		HashSHA256: false,
		HashCRC32C: false,
	}

	for _, h := range hashes {
		expected[h] = true
	}

	for h, found := range expected {
		if !found {
			t.Errorf("SupportedHashes() missing %s", h)
		}
	}
}
