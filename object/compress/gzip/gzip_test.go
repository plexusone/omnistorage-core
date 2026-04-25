package gzip

import (
	"bytes"
	"io"
	"testing"
)

// testWriteCloser wraps a bytes.Buffer with a Close method.
type testWriteCloser struct {
	*bytes.Buffer
	closed bool
}

func newTestWriteCloser() *testWriteCloser {
	return &testWriteCloser{Buffer: new(bytes.Buffer)}
}

func (t *testWriteCloser) Close() error {
	t.closed = true
	return nil
}

// testReadCloser wraps a bytes.Reader with a Close method.
type testReadCloser struct {
	*bytes.Reader
	closed bool
}

func newTestReadCloser(data []byte) *testReadCloser {
	return &testReadCloser{Reader: bytes.NewReader(data)}
}

func (t *testReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestWriterBasic(t *testing.T) {
	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	data := []byte("hello world")
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

	// Buffer should have compressed data
	if buf.Len() == 0 {
		t.Error("Buffer should have compressed data")
	}

	if !buf.closed {
		t.Error("Underlying writer should be closed")
	}
}

func TestWriterLevel(t *testing.T) {
	levels := []CompressionLevel{
		NoCompression,
		BestSpeed,
		DefaultCompression,
		BestCompression,
	}

	data := []byte("hello world, this is a test of gzip compression at various levels")

	for _, level := range levels {
		buf := newTestWriteCloser()
		w, err := NewWriterLevel(buf, level)
		if err != nil {
			t.Fatalf("NewWriterLevel(%d) failed: %v", level, err)
		}

		if _, err := w.Write(data); err != nil {
			t.Fatalf("Write at level %d failed: %v", level, err)
		}

		if err := w.Close(); err != nil {
			t.Fatalf("Close at level %d failed: %v", level, err)
		}

		if buf.Len() == 0 {
			t.Errorf("Buffer at level %d should have data", level)
		}
	}
}

func TestWriterFlush(t *testing.T) {
	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if _, err := w.Write([]byte("test data")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestWriterClosed(t *testing.T) {
	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Write after close should fail
	_, err = w.Write([]byte("test"))
	if err != io.ErrClosedPipe {
		t.Errorf("Write after Close: error = %v, want %v", err, io.ErrClosedPipe)
	}

	// Flush after close should fail
	err = w.Flush()
	if err != io.ErrClosedPipe {
		t.Errorf("Flush after Close: error = %v, want %v", err, io.ErrClosedPipe)
	}

	// Double close should be idempotent
	if err := w.Close(); err != nil {
		t.Errorf("Double Close: error = %v, want nil", err)
	}
}

func TestReaderBasic(t *testing.T) {
	// First compress some data
	compBuf := newTestWriteCloser()
	w, err := NewWriter(compBuf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	originalData := []byte("hello world, this is compressed data")
	if _, err := w.Write(originalData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer failed: %v", err)
	}

	// Now read and decompress
	readBuf := newTestReadCloser(compBuf.Bytes())
	r, err := NewReader(readBuf)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	decompressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close reader failed: %v", err)
	}

	if !bytes.Equal(decompressed, originalData) {
		t.Errorf("Decompressed = %q, want %q", decompressed, originalData)
	}

	if !readBuf.closed {
		t.Error("Underlying reader should be closed")
	}
}

func TestReaderClosed(t *testing.T) {
	// First compress some data
	compBuf := newTestWriteCloser()
	w, err := NewWriter(compBuf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	if _, err := w.Write([]byte("test")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer failed: %v", err)
	}

	// Create reader and close it
	readBuf := newTestReadCloser(compBuf.Bytes())
	r, err := NewReader(readBuf)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read after close should fail
	_, err = r.Read(make([]byte, 10))
	if err != io.ErrClosedPipe {
		t.Errorf("Read after Close: error = %v, want %v", err, io.ErrClosedPipe)
	}

	// Double close should be idempotent
	if err := r.Close(); err != nil {
		t.Errorf("Double Close: error = %v, want nil", err)
	}
}

func TestRoundTrip(t *testing.T) {
	testData := [][]byte{
		[]byte(""),
		[]byte("short"),
		[]byte("hello world"),
		bytes.Repeat([]byte("a"), 1000),
		bytes.Repeat([]byte("abcdefghij"), 1000),
	}

	for i, original := range testData {
		// Compress
		compBuf := newTestWriteCloser()
		w, err := NewWriter(compBuf)
		if err != nil {
			t.Fatalf("Test %d: NewWriter failed: %v", i, err)
		}
		if _, err := w.Write(original); err != nil {
			t.Fatalf("Test %d: Write failed: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Test %d: Close writer failed: %v", i, err)
		}

		// Decompress
		readBuf := newTestReadCloser(compBuf.Bytes())
		r, err := NewReader(readBuf)
		if err != nil {
			t.Fatalf("Test %d: NewReader failed: %v", i, err)
		}
		decompressed, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("Test %d: ReadAll failed: %v", i, err)
		}
		if err := r.Close(); err != nil {
			t.Fatalf("Test %d: Close reader failed: %v", i, err)
		}

		if !bytes.Equal(decompressed, original) {
			t.Errorf("Test %d: decompressed data doesn't match original", i)
		}
	}
}

func TestReaderInvalidData(t *testing.T) {
	// Try to read non-gzip data
	readBuf := newTestReadCloser([]byte("this is not gzip data"))
	_, err := NewReader(readBuf)
	if err == nil {
		t.Error("Expected error when reading non-gzip data")
	}
}

func TestCompressionRatio(t *testing.T) {
	// Highly compressible data
	original := bytes.Repeat([]byte("a"), 10000)

	compBuf := newTestWriteCloser()
	w, err := NewWriterLevel(compBuf, BestCompression)
	if err != nil {
		t.Fatalf("NewWriterLevel failed: %v", err)
	}
	if _, err := w.Write(original); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Compressed size should be much smaller
	if compBuf.Len() >= len(original)/2 {
		t.Errorf("Compression ratio too low: %d -> %d", len(original), compBuf.Len())
	}
}

func TestMultipleWrites(t *testing.T) {
	compBuf := newTestWriteCloser()
	w, err := NewWriter(compBuf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Multiple writes
	parts := []string{"hello", " ", "world", "!"}
	for _, part := range parts {
		if _, err := w.Write([]byte(part)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read back
	readBuf := newTestReadCloser(compBuf.Bytes())
	r, err := NewReader(readBuf)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	decompressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	expected := "hello world!"
	if string(decompressed) != expected {
		t.Errorf("Decompressed = %q, want %q", decompressed, expected)
	}
}

func TestReaderHeader(t *testing.T) {
	// Compress some data
	compBuf := newTestWriteCloser()
	w, err := NewWriter(compBuf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	if _, err := w.Write([]byte("test")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer failed: %v", err)
	}

	// Create reader and check header
	readBuf := newTestReadCloser(compBuf.Bytes())
	r, err := NewReader(readBuf)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	// Header should be accessible (though may be empty for default writer)
	_ = r.Header()

	_ = r.Close()
}
