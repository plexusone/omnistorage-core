package sync

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	// Create a bucket with 1000 bytes/second
	bucket := newTokenBucket(1000)

	if bucket == nil {
		t.Fatal("newTokenBucket returned nil for positive rate")
	}

	// Wait for tokens - should be instant since bucket starts full
	start := time.Now()
	bucket.wait(500)
	elapsed := time.Since(start)

	// Should be near-instant (bucket has 1000 tokens)
	if elapsed > 100*time.Millisecond {
		t.Errorf("First wait took %v, expected near-instant", elapsed)
	}

	// Wait for more tokens than available - should take ~500ms
	start = time.Now()
	bucket.wait(1000) // Need 1000 but only have ~500 left
	elapsed = time.Since(start)

	// Should take approximately 500ms to refill
	if elapsed < 400*time.Millisecond || elapsed > 700*time.Millisecond {
		t.Errorf("Second wait took %v, expected ~500ms", elapsed)
	}
}

func TestTokenBucketNil(t *testing.T) {
	// Zero rate should return nil bucket
	bucket := newTokenBucket(0)
	if bucket != nil {
		t.Error("newTokenBucket(0) should return nil")
	}

	// Negative rate should return nil bucket
	bucket = newTokenBucket(-100)
	if bucket != nil {
		t.Error("newTokenBucket(-100) should return nil")
	}
}

func TestTokenBucketReturnTokens(t *testing.T) {
	bucket := newTokenBucket(1000)

	// Consume all tokens
	bucket.wait(1000)

	// Return some tokens
	bucket.returnTokens(500)

	// Now we should have 500 tokens, so waiting for 500 should be instant
	start := time.Now()
	bucket.wait(500)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Wait after return took %v, expected near-instant", elapsed)
	}
}

func TestRateLimitedReader(t *testing.T) {
	// Create a small buffer for functional testing
	data := []byte("hello world test data for rate limiting")

	// Create a bucket - just test that it works, not specific timing
	bucket := newTokenBucket(1000) // 1000 bytes/second

	reader := newRateLimitedReader(bytes.NewReader(data), bucket)

	// Read all data
	result, err := io.ReadAll(reader)

	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(result, data) {
		t.Error("Data mismatch after rate-limited read")
	}
}

func TestRateLimitedReaderWithThrottling(t *testing.T) {
	// This test verifies that the rate limiter actually throttles
	// by consuming all tokens first, then measuring the refill time

	// Use a rate that accounts for io.ReadAll's 512-byte buffer size
	// 5KB/second means 512 bytes takes about 100ms
	bucket := newTokenBucket(5 * 1024) // 5KB/second

	// Drain the bucket completely
	bucket.wait(5 * 1024) // Consumes all initial tokens

	// Now reading should require waiting for token refill
	// io.ReadAll will try to read 512 bytes, which takes ~100ms at 5KB/s
	data := make([]byte, 512)
	reader := newRateLimitedReader(bytes.NewReader(data), bucket)

	start := time.Now()
	result, err := io.ReadAll(reader)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(result) != len(data) {
		t.Errorf("Data length = %d, want %d", len(result), len(data))
	}

	// Should take at least 50ms (512 bytes at 5KB/s after draining)
	if elapsed < 50*time.Millisecond {
		t.Errorf("Rate-limited read took %v, expected at least 50ms", elapsed)
	}
}

func TestRateLimitedReaderNoLimit(t *testing.T) {
	data := []byte("hello world")
	reader := newRateLimitedReader(bytes.NewReader(data), nil)

	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(result, data) {
		t.Error("Data mismatch")
	}
}
