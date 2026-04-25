package zstd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// testWriteCloser wraps a bytes.Buffer to implement io.WriteCloser
type testWriteCloser struct {
	*bytes.Buffer
	closed bool
}

func newTestWriteCloser() *testWriteCloser {
	return &testWriteCloser{Buffer: &bytes.Buffer{}}
}

func (w *testWriteCloser) Close() error {
	w.closed = true
	return nil
}

// testReadCloser wraps a bytes.Reader to implement io.ReadCloser
type testReadCloser struct {
	*bytes.Reader
	closed bool
}

func newTestReadCloser(data []byte) *testReadCloser {
	return &testReadCloser{Reader: bytes.NewReader(data)}
}

func (r *testReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestWriterBasic(t *testing.T) {
	buf := newTestWriteCloser()

	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	data := []byte("hello zstd world")
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

	// Verify data was compressed (should be smaller or have zstd magic bytes)
	if buf.Len() == 0 {
		t.Error("No compressed data written")
	}

	// Verify underlying writer was closed
	if !buf.closed {
		t.Error("Underlying writer not closed")
	}
}

func TestWriterLevel(t *testing.T) {
	levels := []CompressionLevel{
		SpeedFastest,
		SpeedDefault,
		SpeedBetterCompression,
		SpeedBestCompression,
	}

	data := []byte(strings.Repeat("test data for compression level testing ", 100))

	for _, level := range levels {
		t.Run(level.toZstdLevel().String(), func(t *testing.T) {
			buf := newTestWriteCloser()

			w, err := NewWriterLevel(buf, level)
			if err != nil {
				t.Fatalf("NewWriterLevel failed: %v", err)
			}

			if _, err := w.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			if err := w.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			if buf.Len() == 0 {
				t.Error("No compressed data written")
			}

			// Verify it can be decompressed
			r := newTestReadCloser(buf.Bytes())
			zr, err := NewReader(r)
			if err != nil {
				t.Fatalf("NewReader failed: %v", err)
			}

			decompressed, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			_ = zr.Close()

			if !bytes.Equal(decompressed, data) {
				t.Error("Decompressed data doesn't match original")
			}
		})
	}
}

func TestWriterFlush(t *testing.T) {
	buf := newTestWriteCloser()

	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if _, err := w.Write([]byte("test")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	_ = w.Close()
}

func TestWriterClosed(t *testing.T) {
	buf := newTestWriteCloser()

	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	_ = w.Close()

	// Write after close should fail
	_, err = w.Write([]byte("test"))
	if err != io.ErrClosedPipe {
		t.Errorf("Write after Close error = %v, want io.ErrClosedPipe", err)
	}

	// Flush after close should fail
	err = w.Flush()
	if err != io.ErrClosedPipe {
		t.Errorf("Flush after Close error = %v, want io.ErrClosedPipe", err)
	}
}

func TestReaderBasic(t *testing.T) {
	// First compress some data
	data := []byte("hello zstd reader world")
	compressed := compressData(t, data)

	// Then decompress it
	r := newTestReadCloser(compressed)
	zr, err := NewReader(r)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if err := zr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !bytes.Equal(decompressed, data) {
		t.Errorf("Decompressed = %q, want %q", decompressed, data)
	}

	// Verify underlying reader was closed
	if !r.closed {
		t.Error("Underlying reader not closed")
	}
}

func TestReaderClosed(t *testing.T) {
	data := []byte("test")
	compressed := compressData(t, data)

	r := newTestReadCloser(compressed)
	zr, err := NewReader(r)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	_ = zr.Close()

	// Read after close should fail
	buf := make([]byte, 10)
	_, err = zr.Read(buf)
	if err != io.ErrClosedPipe {
		t.Errorf("Read after Close error = %v, want io.ErrClosedPipe", err)
	}
}

func TestRoundTrip(t *testing.T) {
	testData := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello")},
		{"medium", []byte(strings.Repeat("test data ", 1000))},
		{"large", []byte(strings.Repeat("x", 1024*1024))}, // 1MB
		{"json", []byte(`{"key":"value","array":[1,2,3],"nested":{"a":"b"}}`)},
		{"binary", func() []byte {
			b := make([]byte, 256)
			for i := range b {
				b[i] = byte(i)
			}
			return b
		}()},
	}

	for _, tt := range testData {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			buf := newTestWriteCloser()
			w, err := NewWriter(buf)
			if err != nil {
				t.Fatalf("NewWriter failed: %v", err)
			}

			if _, err := w.Write(tt.data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			// Decompress
			r := newTestReadCloser(buf.Bytes())
			zr, err := NewReader(r)
			if err != nil {
				t.Fatalf("NewReader failed: %v", err)
			}

			decompressed, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			_ = zr.Close()

			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("Round trip failed: got %d bytes, want %d bytes", len(decompressed), len(tt.data))
			}
		})
	}
}

func TestCompressionRatio(t *testing.T) {
	// Highly compressible data
	data := []byte(strings.Repeat("abcdefghij", 10000)) // 100KB of repeated text

	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	ratio := float64(buf.Len()) / float64(len(data))
	t.Logf("Compression ratio: %.2f%% (original: %d, compressed: %d)", ratio*100, len(data), buf.Len())

	// Zstd should compress this well - expect at least 90% reduction
	if ratio > 0.1 {
		t.Errorf("Compression ratio %.2f%% is worse than expected for highly compressible data", ratio*100)
	}
}

func TestMultipleWrites(t *testing.T) {
	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Write multiple chunks
	chunks := [][]byte{
		[]byte("first chunk "),
		[]byte("second chunk "),
		[]byte("third chunk"),
	}

	for _, chunk := range chunks {
		if _, err := w.Write(chunk); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Decompress and verify
	r := newTestReadCloser(buf.Bytes())
	zr, err := NewReader(r)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = zr.Close()

	expected := "first chunk second chunk third chunk"
	if string(decompressed) != expected {
		t.Errorf("Decompressed = %q, want %q", decompressed, expected)
	}
}

func TestWriterWithOptions(t *testing.T) {
	buf := newTestWriteCloser()

	// Use custom options
	w, err := NewWriterWithOptions(buf,
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(1),
	)
	if err != nil {
		t.Fatalf("NewWriterWithOptions failed: %v", err)
	}

	data := []byte("test with custom options")
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify it can be decompressed
	r := newTestReadCloser(buf.Bytes())
	zr, err := NewReader(r)
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = zr.Close()

	if !bytes.Equal(decompressed, data) {
		t.Error("Decompressed data doesn't match original")
	}
}

func TestReaderWithOptions(t *testing.T) {
	data := []byte("test with reader options")
	compressed := compressData(t, data)

	r := newTestReadCloser(compressed)
	zr, err := NewReaderWithOptions(r,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxMemory(1024*1024), // 1MB max
	)
	if err != nil {
		t.Fatalf("NewReaderWithOptions failed: %v", err)
	}

	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = zr.Close()

	if !bytes.Equal(decompressed, data) {
		t.Error("Decompressed data doesn't match original")
	}
}

func TestCompressionLevelConversion(t *testing.T) {
	tests := []struct {
		level    CompressionLevel
		expected zstd.EncoderLevel
	}{
		{SpeedFastest, zstd.SpeedFastest},
		{SpeedDefault, zstd.SpeedDefault},
		{SpeedBetterCompression, zstd.SpeedBetterCompression},
		{SpeedBestCompression, zstd.SpeedBestCompression},
		{CompressionLevel(999), zstd.SpeedDefault}, // Unknown level defaults to SpeedDefault
	}

	for _, tt := range tests {
		result := tt.level.toZstdLevel()
		if result != tt.expected {
			t.Errorf("Level %d: got %v, want %v", tt.level, result, tt.expected)
		}
	}
}

func TestCloseIdempotent(t *testing.T) {
	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Close multiple times should not error
	if err := w.Close(); err != nil {
		t.Errorf("First Close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// compressData is a helper to compress data for reader tests
func compressData(t *testing.T, data []byte) []byte {
	t.Helper()
	buf := newTestWriteCloser()
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("compressData: NewWriter failed: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("compressData: Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("compressData: Close failed: %v", err)
	}
	return buf.Bytes()
}

// Benchmark tests

func BenchmarkCompress(b *testing.B) {
	data := []byte(strings.Repeat("benchmark test data ", 10000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := newTestWriteCloser()
		w, _ := NewWriter(buf)
		_, _ = w.Write(data)
		_ = w.Close()
	}
}

func BenchmarkDecompress(b *testing.B) {
	data := []byte(strings.Repeat("benchmark test data ", 10000))
	compressed := func() []byte {
		buf := newTestWriteCloser()
		w, _ := NewWriter(buf)
		_, _ = w.Write(data)
		_ = w.Close()
		return buf.Bytes()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := newTestReadCloser(compressed)
		zr, _ := NewReader(r)
		_, _ = io.ReadAll(zr)
		_ = zr.Close()
	}
}
