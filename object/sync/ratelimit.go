package sync

import (
	"io"
	"sync"
	"time"
)

// rateLimitedReader wraps an io.Reader with bandwidth limiting.
// It uses a token bucket algorithm to throttle reads.
type rateLimitedReader struct {
	reader    io.Reader
	bucket    *tokenBucket
	chunkSize int
}

// newRateLimitedReader creates a reader that limits bandwidth.
// bytesPerSecond is the maximum bytes per second (0 = unlimited).
// The bucket is shared across all readers to enforce a global limit.
func newRateLimitedReader(r io.Reader, bucket *tokenBucket) *rateLimitedReader {
	return &rateLimitedReader{
		reader:    r,
		bucket:    bucket,
		chunkSize: 64 * 1024, // 64KB chunks for smooth limiting
	}
}

// Read implements io.Reader with rate limiting.
func (r *rateLimitedReader) Read(p []byte) (int, error) {
	if r.bucket == nil || r.bucket.rate == 0 {
		// No rate limiting
		return r.reader.Read(p)
	}

	// Limit read size to chunk size for smoother rate limiting
	toRead := len(p)
	if toRead > r.chunkSize {
		toRead = r.chunkSize
	}

	// Wait for tokens
	r.bucket.wait(toRead)

	// Perform the read
	n, err := r.reader.Read(p[:toRead])

	// If we read less than requested, return unused tokens
	if n < toRead {
		r.bucket.returnTokens(toRead - n)
	}

	return n, err
}

// tokenBucket implements a token bucket rate limiter.
// It's safe for concurrent use.
type tokenBucket struct {
	rate       int64 // bytes per second
	tokens     int64 // current available tokens
	maxTokens  int64 // maximum tokens (burst size)
	lastRefill time.Time
	mu         sync.Mutex
}

// newTokenBucket creates a new token bucket with the given rate.
// rate is in bytes per second. Burst size is set to 1 second worth of tokens.
func newTokenBucket(bytesPerSecond int64) *tokenBucket {
	if bytesPerSecond <= 0 {
		return nil
	}
	return &tokenBucket{
		rate:       bytesPerSecond,
		tokens:     bytesPerSecond, // Start with full bucket
		maxTokens:  bytesPerSecond, // 1 second burst
		lastRefill: time.Now(),
	}
}

// wait blocks until n tokens are available and consumes them.
func (tb *tokenBucket) wait(n int) {
	if tb == nil || tb.rate == 0 {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on elapsed time
	tb.refill()

	needed := int64(n)

	// If we need more tokens than available, wait
	for tb.tokens < needed {
		// Calculate wait time
		deficit := needed - tb.tokens
		waitDuration := time.Duration(deficit) * time.Second / time.Duration(tb.rate)

		// Release lock while waiting
		tb.mu.Unlock()
		time.Sleep(waitDuration)
		tb.mu.Lock()

		// Refill after waiting
		tb.refill()
	}

	// Consume tokens
	tb.tokens -= needed
}

// returnTokens returns unused tokens to the bucket.
func (tb *tokenBucket) returnTokens(n int) {
	if tb == nil || n <= 0 {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.tokens += int64(n)
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
}

// refill adds tokens based on elapsed time since last refill.
// Must be called with mu held.
func (tb *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tb.lastRefill = now

	// Add tokens proportional to elapsed time
	newTokens := int64(elapsed.Seconds() * float64(tb.rate))
	tb.tokens += newTokens
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
}
