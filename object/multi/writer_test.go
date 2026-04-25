package multi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
	"github.com/plexusone/omnistorage-core/object/backend/memory"
)

func TestNewWriter(t *testing.T) {
	b1 := memory.New()
	b2 := memory.New()

	mw, err := NewWriter(b1, b2)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if mw.Backends() != 2 {
		t.Errorf("Backends() = %d, want 2", mw.Backends())
	}
}

func TestNewWriterNoBackends(t *testing.T) {
	_, err := NewWriter()
	if err == nil {
		t.Error("NewWriter with no backends should fail")
	}
}

func TestNewWriterNilBackends(t *testing.T) {
	_, err := NewWriter(nil, nil)
	if err == nil {
		t.Error("NewWriter with only nil backends should fail")
	}

	// Mixed nil and valid should work
	b := memory.New()
	mw, err := NewWriter(nil, b, nil)
	if err != nil {
		t.Fatalf("NewWriter with mixed backends failed: %v", err)
	}
	if mw.Backends() != 1 {
		t.Errorf("Backends() = %d, want 1", mw.Backends())
	}
}

func TestWriteToMultipleBackends(t *testing.T) {
	b1 := memory.New()
	b2 := memory.New()
	b3 := memory.New()

	mw, err := NewWriter(b1, b2, b3)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ctx := context.Background()
	path := "test/file.txt"
	data := []byte("hello multi-writer")

	// Write to all backends
	w, err := mw.NewWriter(ctx, path)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify data in all backends
	for i, b := range []omnistorage.Backend{b1, b2, b3} {
		r, err := b.NewReader(ctx, path)
		if err != nil {
			t.Errorf("Backend %d: NewReader failed: %v", i, err)
			continue
		}

		result, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Errorf("Backend %d: ReadAll failed: %v", i, err)
			continue
		}

		if !bytes.Equal(result, data) {
			t.Errorf("Backend %d: data = %q, want %q", i, result, data)
		}
	}
}

func TestWriteAllModeFailure(t *testing.T) {
	b1 := memory.New()
	b2 := &failingBackend{fail: true}

	mw, err := NewWriterWithOptions(
		[]omnistorage.Backend{b1, b2},
		WithMode(WriteAll),
	)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ctx := context.Background()
	_, err = mw.NewWriter(ctx, "test/file.txt")
	if err == nil {
		t.Error("NewWriter should fail in WriteAll mode when a backend fails")
	}

	var me *MultiError
	if !errors.As(err, &me) {
		t.Errorf("Error should be *MultiError, got %T", err)
	}
}

func TestWriteBestEffortMode(t *testing.T) {
	b1 := memory.New()
	b2 := &failingBackend{fail: true}
	b3 := memory.New()

	mw, err := NewWriterWithOptions(
		[]omnistorage.Backend{b1, b2, b3},
		WithMode(WriteBestEffort),
	)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ctx := context.Background()
	path := "test/file.txt"
	data := []byte("best effort data")

	w, err := mw.NewWriter(ctx, path)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify data in working backends
	for i, b := range []omnistorage.Backend{b1, b3} {
		r, err := b.NewReader(ctx, path)
		if err != nil {
			t.Errorf("Backend %d: NewReader failed: %v", i, err)
			continue
		}

		result, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Errorf("Backend %d: ReadAll failed: %v", i, err)
			continue
		}

		if !bytes.Equal(result, data) {
			t.Errorf("Backend %d: data = %q, want %q", i, result, data)
		}
	}
}

func TestWriteQuorumModeSuccess(t *testing.T) {
	b1 := memory.New()
	b2 := memory.New()
	b3 := &failingBackend{fail: true}

	mw, err := NewWriterWithOptions(
		[]omnistorage.Backend{b1, b2, b3},
		WithMode(WriteQuorum),
	)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ctx := context.Background()
	path := "test/file.txt"
	data := []byte("quorum data")

	// 2 out of 3 backends is a majority, should succeed
	w, err := mw.NewWriter(ctx, path)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	_, err = w.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestWriteQuorumModeFailure(t *testing.T) {
	b1 := memory.New()
	b2 := &failingBackend{fail: true}
	b3 := &failingBackend{fail: true}

	mw, err := NewWriterWithOptions(
		[]omnistorage.Backend{b1, b2, b3},
		WithMode(WriteQuorum),
	)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ctx := context.Background()

	// 1 out of 3 backends is not a majority, should fail
	_, err = mw.NewWriter(ctx, "test/file.txt")
	if err == nil {
		t.Error("NewWriter should fail when quorum cannot be achieved")
	}
}

func TestMultipleWrites(t *testing.T) {
	b1 := memory.New()
	b2 := memory.New()

	mw, err := NewWriter(b1, b2)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ctx := context.Background()
	path := "test/file.txt"

	w, err := mw.NewWriter(ctx, path)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Write multiple times
	chunks := []string{"hello ", "world ", "test"}
	for _, chunk := range chunks {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify concatenated data
	expected := []byte("hello world test")
	for i, b := range []omnistorage.Backend{b1, b2} {
		r, _ := b.NewReader(ctx, path)
		result, _ := io.ReadAll(r)
		_ = r.Close()

		if !bytes.Equal(result, expected) {
			t.Errorf("Backend %d: data = %q, want %q", i, result, expected)
		}
	}
}

func TestDoubleClose(t *testing.T) {
	b1 := memory.New()
	mw, _ := NewWriter(b1)

	ctx := context.Background()
	w, _ := mw.NewWriter(ctx, "test/file.txt")
	_, _ = w.Write([]byte("data"))

	// First close should succeed
	if err := w.Close(); err != nil {
		t.Errorf("First Close failed: %v", err)
	}

	// Second close should not panic and return nil
	if err := w.Close(); err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

func TestWriteAfterClose(t *testing.T) {
	b1 := memory.New()
	mw, _ := NewWriter(b1)

	ctx := context.Background()
	w, _ := mw.NewWriter(ctx, "test/file.txt")
	_ = w.Close()

	_, err := w.Write([]byte("data"))
	if err == nil {
		t.Error("Write after Close should fail")
	}
}

func TestMultiError(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	me := &MultiError{Errors: []error{err1, err2}}

	// Test Error()
	if me.Error() == "" {
		t.Error("Error() returned empty string")
	}

	// Test Unwrap()
	if !errors.Is(me, err1) {
		t.Error("errors.Is should match first error")
	}

	// Test All()
	if len(me.All()) != 2 {
		t.Errorf("All() returned %d errors, want 2", len(me.All()))
	}
}

func TestMultiErrorEmpty(t *testing.T) {
	me := &MultiError{}

	if me.Error() != "no errors" {
		t.Errorf("Error() = %q, want 'no errors'", me.Error())
	}

	if me.Unwrap() != nil {
		t.Error("Unwrap() should return nil for empty MultiError")
	}
}

func TestMultiErrorSingle(t *testing.T) {
	err := errors.New("single error")
	me := &MultiError{Errors: []error{err}}

	if me.Error() != "single error" {
		t.Errorf("Error() = %q, want 'single error'", me.Error())
	}
}

// failingBackend is a test backend that always fails.
type failingBackend struct {
	fail bool
}

func (f *failingBackend) NewWriter(_ context.Context, _ string, _ ...omnistorage.WriterOption) (io.WriteCloser, error) {
	if f.fail {
		return nil, errors.New("backend failure")
	}
	return nil, nil
}

func (f *failingBackend) NewReader(_ context.Context, _ string, _ ...omnistorage.ReaderOption) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (f *failingBackend) Exists(_ context.Context, _ string) (bool, error) {
	return false, errors.New("not implemented")
}

func (f *failingBackend) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (f *failingBackend) List(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (f *failingBackend) Close() error {
	return nil
}
