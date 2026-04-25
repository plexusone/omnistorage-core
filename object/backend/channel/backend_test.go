package channel

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	backend := New()

	if backend == nil {
		t.Fatal("New() returned nil")
	}

	if backend.bufferSize != 100 {
		t.Errorf("Default buffer size = %d, want 100", backend.bufferSize)
	}

	if backend.persistent {
		t.Error("Default persistent should be false")
	}
}

func TestNewWithOptions(t *testing.T) {
	backend := New(
		WithBufferSize(50),
		WithPersistence(true),
	)

	if backend.bufferSize != 50 {
		t.Errorf("Buffer size = %d, want 50", backend.bufferSize)
	}

	if !backend.persistent {
		t.Error("Persistent should be true")
	}
}

func TestNewFromConfig(t *testing.T) {
	config := map[string]string{
		"buffer_size": "200",
		"persistent":  "true",
	}

	b, err := NewFromConfig(config)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}

	backend := b.(*Backend)

	if backend.bufferSize != 200 {
		t.Errorf("Buffer size = %d, want 200", backend.bufferSize)
	}

	if !backend.persistent {
		t.Error("Persistent should be true")
	}
}

func TestWriteAndRead(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Start reader in goroutine
	done := make(chan struct{})
	var readData []byte
	var readErr error

	go func() {
		defer close(done)
		r, err := backend.NewReader(ctx, "test/path")
		if err != nil {
			readErr = err
			return
		}
		defer func() { _ = r.Close() }()

		buf := make([]byte, 1024)
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			readErr = err
			return
		}
		readData = buf[:n]
	}()

	// Give reader time to start
	time.Sleep(10 * time.Millisecond)

	// Write data
	w, err := backend.NewWriter(ctx, "test/path")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	testData := []byte("hello world")
	n, err := w.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(testData) {
		t.Errorf("Write returned %d, want %d", n, len(testData))
	}

	// Close writer to signal EOF
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Wait for reader to finish
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Reader timed out")
	}

	if readErr != nil {
		t.Fatalf("Read failed: %v", readErr)
	}

	if !bytes.Equal(readData, testData) {
		t.Errorf("Read data = %q, want %q", readData, testData)
	}
}

func TestPersistentMode(t *testing.T) {
	backend := New(WithPersistence(true))
	ctx := context.Background()

	// Write data first
	w, err := backend.NewWriter(ctx, "test/path")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	testData := []byte("persistent data")
	if _, err := w.Write(testData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Don't close writer yet - create a reader
	r, err := backend.NewReader(ctx, "test/path")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	// Read the buffered data
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !bytes.Equal(buf[:n], testData) {
		t.Errorf("Read data = %q, want %q", buf[:n], testData)
	}

	_ = w.Close()
	_ = r.Close()
}

func TestExists(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Check non-existent path
	exists, err := backend.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true for non-existent path")
	}

	// Create a channel
	w, _ := backend.NewWriter(ctx, "test/path")
	_ = w.Close()

	// Check existing path
	exists, err = backend.Exists(ctx, "test/path")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing path")
	}
}

func TestDelete(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Create a channel
	w, _ := backend.NewWriter(ctx, "test/path")
	_ = w.Close()

	// Delete it
	if err := backend.Delete(ctx, "test/path"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, _ := backend.Exists(ctx, "test/path")
	if exists {
		t.Error("Path still exists after delete")
	}
}

func TestList(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Create several channels
	paths := []string{"a/1", "a/2", "b/1", "b/2"}
	for _, p := range paths {
		w, _ := backend.NewWriter(ctx, p)
		_ = w.Close()
	}

	// List all
	result, err := backend.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("List returned %d paths, want 4", len(result))
	}

	// List with prefix
	result, err = backend.List(ctx, "a/")
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("List with prefix returned %d paths, want 2", len(result))
	}
}

func TestClose(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Create a channel
	w, _ := backend.NewWriter(ctx, "test/path")
	_ = w.Close()

	// Close backend
	if err := backend.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify operations fail after close
	_, err := backend.NewWriter(ctx, "test")
	if err == nil {
		t.Error("NewWriter should fail after Close")
	}

	_, err = backend.NewReader(ctx, "test")
	if err == nil {
		t.Error("NewReader should fail after Close")
	}
}

func TestBroadcast(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Create multiple channels
	var wg sync.WaitGroup
	received := make([][]byte, 3)

	for i := 0; i < 3; i++ {
		path := "events/" + string(rune('a'+i))
		wg.Add(1)

		go func(idx int, p string) {
			defer wg.Done()
			r, _ := backend.NewReader(ctx, p)
			defer func() { _ = r.Close() }()

			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			received[idx] = buf[:n]
		}(i, path)
	}

	// Give readers time to start
	time.Sleep(20 * time.Millisecond)

	// Broadcast to all
	data := []byte("broadcast message")
	if err := backend.Broadcast(ctx, "events/", data); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// Close all channels to unblock readers
	for i := 0; i < 3; i++ {
		path := "events/" + string(rune('a'+i))
		_ = backend.Delete(ctx, path)
	}

	wg.Wait()

	// Verify all received
	for i, data := range received {
		if len(data) == 0 {
			t.Errorf("Reader %d received no data", i)
		}
	}
}

func TestChannelCount(t *testing.T) {
	backend := New()
	ctx := context.Background()

	if backend.ChannelCount() != 0 {
		t.Error("Initial channel count should be 0")
	}

	w1, _ := backend.NewWriter(ctx, "path1")
	w2, _ := backend.NewWriter(ctx, "path2")

	if backend.ChannelCount() != 2 {
		t.Errorf("Channel count = %d, want 2", backend.ChannelCount())
	}

	_ = w1.Close()
	_ = w2.Close()
}

func TestContextCancellation(t *testing.T) {
	backend := New()
	ctx, cancel := context.WithCancel(context.Background())

	// Create writer with cancelled context
	cancel()

	_, err := backend.NewWriter(ctx, "test")
	if err == nil {
		t.Error("NewWriter should fail with cancelled context")
	}

	_, err = backend.NewReader(ctx, "test")
	if err == nil {
		t.Error("NewReader should fail with cancelled context")
	}
}

func TestEmptyPath(t *testing.T) {
	backend := New()
	ctx := context.Background()

	_, err := backend.NewWriter(ctx, "")
	if err == nil {
		t.Error("NewWriter should fail with empty path")
	}

	_, err = backend.NewReader(ctx, "")
	if err == nil {
		t.Error("NewReader should fail with empty path")
	}
}

func TestMultipleWrites(t *testing.T) {
	backend := New()
	ctx := context.Background()

	// Start reader
	done := make(chan struct{})
	var messages [][]byte

	go func() {
		defer close(done)
		r, _ := backend.NewReader(ctx, "test")
		defer func() { _ = r.Close() }()

		for i := 0; i < 3; i++ {
			buf := make([]byte, 1024)
			n, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return
			}
			messages = append(messages, buf[:n])
		}
	}()

	// Give reader time to start
	time.Sleep(10 * time.Millisecond)

	// Write multiple messages
	w, _ := backend.NewWriter(ctx, "test")
	for i := 0; i < 3; i++ {
		_, _ = w.Write([]byte("message"))
	}
	_ = w.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for reader")
	}

	if len(messages) != 3 {
		t.Errorf("Received %d messages, want 3", len(messages))
	}
}

func TestDoubleClose(t *testing.T) {
	backend := New()
	ctx := context.Background()

	w, _ := backend.NewWriter(ctx, "test")
	_ = w.Close()
	_ = w.Close() // Should not panic

	r, _ := backend.NewReader(ctx, "test")
	_ = r.Close()
	_ = r.Close() // Should not panic

	_ = backend.Close()
	_ = backend.Close() // Should not panic
}
