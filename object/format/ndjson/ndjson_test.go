package ndjson

import (
	"bytes"
	"io"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
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
	w := NewWriter(buf)

	records := []string{
		`{"name":"alice","age":30}`,
		`{"name":"bob","age":25}`,
		`{"name":"charlie","age":35}`,
	}

	for _, record := range records {
		if err := w.Write([]byte(record)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	expected := `{"name":"alice","age":30}
{"name":"bob","age":25}
{"name":"charlie","age":35}
`
	if buf.String() != expected {
		t.Errorf("Written content = %q, want %q", buf.String(), expected)
	}

	if !buf.closed {
		t.Error("Underlying writer should be closed")
	}
}

func TestWriterFlush(t *testing.T) {
	buf := newTestWriteCloser()
	w := NewWriter(buf)

	if err := w.Write([]byte(`{"test":true}`)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Data should be in buffer after flush
	if buf.Len() == 0 {
		t.Error("Buffer should have data after flush")
	}

	_ = w.Close()
}

func TestWriterClosed(t *testing.T) {
	buf := newTestWriteCloser()
	w := NewWriter(buf)

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Write after close should fail
	err := w.Write([]byte(`{"test":true}`))
	if err != omnistorage.ErrWriterClosed {
		t.Errorf("Write after Close: error = %v, want %v", err, omnistorage.ErrWriterClosed)
	}

	// Flush after close should fail
	err = w.Flush()
	if err != omnistorage.ErrWriterClosed {
		t.Errorf("Flush after Close: error = %v, want %v", err, omnistorage.ErrWriterClosed)
	}
}

func TestWriterWriteJSON(t *testing.T) {
	buf := newTestWriteCloser()
	w := NewWriter(buf)

	// WriteJSON should trim trailing whitespace
	if err := w.WriteJSON([]byte(`{"test":true}  `)); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
	if err := w.WriteJSON([]byte(`{"test":false}` + "\n")); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	expected := `{"test":true}
{"test":false}
`
	if buf.String() != expected {
		t.Errorf("Written content = %q, want %q", buf.String(), expected)
	}
}

func TestReaderBasic(t *testing.T) {
	data := []byte(`{"name":"alice","age":30}
{"name":"bob","age":25}
{"name":"charlie","age":35}
`)
	buf := newTestReadCloser(data)
	r := NewReader(buf)

	expected := []string{
		`{"name":"alice","age":30}`,
		`{"name":"bob","age":25}`,
		`{"name":"charlie","age":35}`,
	}

	for i, exp := range expected {
		record, err := r.Read()
		if err != nil {
			t.Fatalf("Read %d failed: %v", i, err)
		}
		if string(record) != exp {
			t.Errorf("Read %d = %q, want %q", i, string(record), exp)
		}
	}

	// Should return EOF after all records
	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF, got: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !buf.closed {
		t.Error("Underlying reader should be closed")
	}
}

func TestReaderSkipsEmptyLines(t *testing.T) {
	data := []byte(`{"first":1}

{"second":2}

{"third":3}
`)
	buf := newTestReadCloser(data)
	r := NewReader(buf)

	expected := []string{
		`{"first":1}`,
		`{"second":2}`,
		`{"third":3}`,
	}

	for i, exp := range expected {
		record, err := r.Read()
		if err != nil {
			t.Fatalf("Read %d failed: %v", i, err)
		}
		if string(record) != exp {
			t.Errorf("Read %d = %q, want %q", i, string(record), exp)
		}
	}

	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF, got: %v", err)
	}

	_ = r.Close()
}

func TestReaderClosed(t *testing.T) {
	data := []byte(`{"test":true}`)
	buf := newTestReadCloser(data)
	r := NewReader(buf)

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read after close should fail
	_, err := r.Read()
	if err != omnistorage.ErrReaderClosed {
		t.Errorf("Read after Close: error = %v, want %v", err, omnistorage.ErrReaderClosed)
	}
}

func TestReaderEmpty(t *testing.T) {
	data := []byte(``)
	buf := newTestReadCloser(data)
	r := NewReader(buf)

	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF for empty input, got: %v", err)
	}

	_ = r.Close()
}

func TestReaderOnlyEmptyLines(t *testing.T) {
	data := []byte(`



`)
	buf := newTestReadCloser(data)
	r := NewReader(buf)

	_, err := r.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF for only empty lines, got: %v", err)
	}

	_ = r.Close()
}

func TestRoundTrip(t *testing.T) {
	records := []string{
		`{"id":1,"name":"alice"}`,
		`{"id":2,"name":"bob"}`,
		`{"id":3,"name":"charlie"}`,
	}

	// Write records
	writeBuf := newTestWriteCloser()
	w := NewWriter(writeBuf)
	for _, record := range records {
		if err := w.Write([]byte(record)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer failed: %v", err)
	}

	// Read records back
	readBuf := newTestReadCloser(writeBuf.Bytes())
	r := NewReader(readBuf)

	var readRecords []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		readRecords = append(readRecords, string(record))
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close reader failed: %v", err)
	}

	// Verify
	if len(readRecords) != len(records) {
		t.Fatalf("Read %d records, want %d", len(readRecords), len(records))
	}

	for i, record := range records {
		if readRecords[i] != record {
			t.Errorf("Record %d = %q, want %q", i, readRecords[i], record)
		}
	}
}

func TestWriterSize(t *testing.T) {
	buf := newTestWriteCloser()
	w := NewWriterSize(buf, 1024)

	if err := w.Write([]byte(`{"test":true}`)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	expected := `{"test":true}
`
	if buf.String() != expected {
		t.Errorf("Written content = %q, want %q", buf.String(), expected)
	}
}

func TestReaderSize(t *testing.T) {
	data := []byte(`{"test":true}
`)
	buf := newTestReadCloser(data)
	r := NewReaderSize(buf, 1024)

	record, err := r.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(record) != `{"test":true}` {
		t.Errorf("Read = %q, want %q", string(record), `{"test":true}`)
	}

	_ = r.Close()
}

func TestReaderRecordCopy(t *testing.T) {
	data := []byte(`{"first":1}
{"second":2}
`)
	buf := newTestReadCloser(data)
	r := NewReader(buf)

	// Read first record
	record1, err := r.Read()
	if err != nil {
		t.Fatalf("Read 1 failed: %v", err)
	}

	// Store original value
	original := string(record1)

	// Read second record (this would overwrite if not copied)
	_, err = r.Read()
	if err != nil {
		t.Fatalf("Read 2 failed: %v", err)
	}

	// First record should still be valid
	if string(record1) != original {
		t.Errorf("First record modified after second read: %q, want %q", string(record1), original)
	}

	_ = r.Close()
}
