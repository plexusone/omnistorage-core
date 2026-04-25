package sync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetrySuccess(t *testing.T) {
	calls := 0
	err := retryOperation(context.Background(), RetryConfig{MaxRetries: 3}, func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestRetryEventualSuccess(t *testing.T) {
	calls := 0
	err := retryOperation(context.Background(), RetryConfig{
		MaxRetries:   3,
		InitialDelay: time.Millisecond,
	}, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
}

func TestRetryExhausted(t *testing.T) {
	calls := 0
	err := retryOperation(context.Background(), RetryConfig{
		MaxRetries:   2,
		InitialDelay: time.Millisecond,
	}, func() error {
		calls++
		return errors.New("persistent error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should be called initial + 2 retries = 3 times
	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}

	// Check error type
	var retryErr *RetryError
	if !errors.As(err, &retryErr) {
		t.Error("Expected RetryError type")
	} else if retryErr.Attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", retryErr.Attempts)
	}
}

func TestRetryNoRetries(t *testing.T) {
	calls := 0
	err := retryOperation(context.Background(), RetryConfig{MaxRetries: 0}, func() error {
		calls++
		return errors.New("error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := retryOperation(ctx, RetryConfig{
		MaxRetries:   10,
		InitialDelay: 5 * time.Millisecond,
	}, func() error {
		calls++
		return errors.New("error")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

func TestRetryNonRetryableError(t *testing.T) {
	permanentErr := errors.New("permanent error")
	calls := 0

	err := retryOperation(context.Background(), RetryConfig{
		MaxRetries:   3,
		InitialDelay: time.Millisecond,
		RetryableErrors: func(err error) bool {
			return !errors.Is(err, permanentErr)
		},
	}, func() error {
		calls++
		return permanentErr
	})

	if !errors.Is(err, permanentErr) {
		t.Errorf("Expected permanent error, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("Expected 1 call (no retry), got %d", calls)
	}
}

func TestRetryExponentialBackoff(t *testing.T) {
	var callTimes []time.Time

	err := retryOperation(context.Background(), RetryConfig{
		MaxRetries:   3,
		InitialDelay: 20 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0, // No jitter for predictable test
	}, func() error {
		callTimes = append(callTimes, time.Now())
		return errors.New("error")
	})

	if err == nil {
		t.Error("Expected error")
	}

	// Should have 4 calls (initial + 3 retries)
	if len(callTimes) != 4 {
		t.Fatalf("Expected 4 calls, got %d", len(callTimes))
	}

	// Calculate delays between calls
	// First call is immediate, then delays of 20ms, 40ms, 80ms
	for i := 1; i < len(callTimes); i++ {
		delay := callTimes[i].Sub(callTimes[i-1])

		// Expected delays: 20ms, 40ms, 80ms (with 50% tolerance for CI variability)
		expectedBase := time.Duration(20*1<<(i-1)) * time.Millisecond
		minExpected := time.Duration(float64(expectedBase) * 0.5)
		maxExpected := time.Duration(float64(expectedBase) * 2.0)

		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay %d: %v not in expected range %v-%v", i, delay, minExpected, maxExpected)
		}
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", config.MaxRetries)
	}
	if config.InitialDelay != time.Second {
		t.Errorf("Expected InitialDelay=1s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay=30s, got %v", config.MaxDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier=2.0, got %v", config.Multiplier)
	}
	if config.Jitter != 0.1 {
		t.Errorf("Expected Jitter=0.1, got %v", config.Jitter)
	}
}

func TestIsRetryError(t *testing.T) {
	retryErr := &RetryError{Attempts: 3, LastErr: errors.New("test")}
	normalErr := errors.New("normal")

	if !IsRetryError(retryErr) {
		t.Error("Expected IsRetryError to return true for RetryError")
	}
	if IsRetryError(normalErr) {
		t.Error("Expected IsRetryError to return false for normal error")
	}
}

func TestRetryErrorUnwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	retryErr := &RetryError{Attempts: 3, LastErr: innerErr}

	if !errors.Is(retryErr, innerErr) {
		t.Error("RetryError should unwrap to inner error")
	}
}

type temporaryError struct{}

func (e temporaryError) Error() string   { return "temporary" }
func (e temporaryError) Temporary() bool { return true }

type timeoutError struct{}

func (e timeoutError) Error() string { return "timeout" }
func (e timeoutError) Timeout() bool { return true }

func TestIsTemporaryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"normal error", errors.New("error"), false},
		{"temporary error", temporaryError{}, true},
		{"timeout error", timeoutError{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsTemporaryError(tc.err)
			if got != tc.want {
				t.Errorf("IsTemporaryError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
