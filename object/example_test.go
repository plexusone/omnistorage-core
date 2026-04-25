package object_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
	"github.com/plexusone/omnistorage-core/object/backend/file"
	"github.com/plexusone/omnistorage-core/object/compress/gzip"
	"github.com/plexusone/omnistorage-core/object/format/ndjson"
)

// TestIntegrationFileNDJSON demonstrates writing and reading NDJSON records
// using the file backend.
func TestIntegrationFileNDJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file backend
	backend := file.New(file.Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write NDJSON records
	w, err := backend.NewWriter(ctx, "records.ndjson")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	ndjsonWriter := ndjson.NewWriter(w)
	records := []string{
		`{"id":1,"name":"alice"}`,
		`{"id":2,"name":"bob"}`,
		`{"id":3,"name":"charlie"}`,
	}

	for _, record := range records {
		if err := ndjsonWriter.Write([]byte(record)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := ndjsonWriter.Close(); err != nil {
		t.Fatalf("Close writer failed: %v", err)
	}

	// Read NDJSON records back
	r, err := backend.NewReader(ctx, "records.ndjson")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	ndjsonReader := ndjson.NewReader(r)
	var readRecords []string

	for {
		record, err := ndjsonReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		readRecords = append(readRecords, string(record))
	}

	if err := ndjsonReader.Close(); err != nil {
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

// TestIntegrationFileGzipNDJSON demonstrates the full stack:
// file backend -> gzip compression -> NDJSON format
func TestIntegrationFileGzipNDJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file backend
	backend := file.New(file.Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write: file -> gzip -> ndjson
	fileWriter, err := backend.NewWriter(ctx, "records.ndjson.gz")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	gzipWriter, err := gzip.NewWriter(fileWriter)
	if err != nil {
		t.Fatalf("gzip.NewWriter failed: %v", err)
	}

	// Wrap gzip writer to implement WriteCloser for ndjson
	ndjsonWriter := ndjson.NewWriter(&gzipWriteCloser{gzipWriter})

	records := []string{
		`{"id":1,"type":"user","data":{"name":"alice","age":30}}`,
		`{"id":2,"type":"user","data":{"name":"bob","age":25}}`,
		`{"id":3,"type":"user","data":{"name":"charlie","age":35}}`,
	}

	for _, record := range records {
		if err := ndjsonWriter.Write([]byte(record)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if err := ndjsonWriter.Close(); err != nil {
		t.Fatalf("Close ndjson writer failed: %v", err)
	}

	// Verify file was created and is compressed
	compressedPath := filepath.Join(tmpDir, "records.ndjson.gz")
	info, err := os.Stat(compressedPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	t.Logf("Compressed file size: %d bytes", info.Size())

	// Read: file -> gzip -> ndjson
	fileReader, err := backend.NewReader(ctx, "records.ndjson.gz")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	gzipReader, err := gzip.NewReader(fileReader)
	if err != nil {
		t.Fatalf("gzip.NewReader failed: %v", err)
	}

	ndjsonReader := ndjson.NewReader(&gzipReadCloser{gzipReader})
	var readRecords []string

	for {
		record, err := ndjsonReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		readRecords = append(readRecords, string(record))
	}

	if err := ndjsonReader.Close(); err != nil {
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

// TestIntegrationRegistry demonstrates using the backend registry.
func TestIntegrationRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	// Open backend by name using registry
	backend, err := omnistorage.Open("file", map[string]string{
		"root": tmpDir,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write a file
	w, err := backend.NewWriter(ctx, "test.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	if _, err := w.Write([]byte("hello from registry")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify with Exists
	exists, err := backend.Exists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("File should exist")
	}

	// Read it back
	r, err := backend.NewReader(ctx, "test.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close reader failed: %v", err)
	}

	if string(data) != "hello from registry" {
		t.Errorf("Read = %q, want %q", data, "hello from registry")
	}
}

// TestIntegrationBackendsList verifies registered backends.
func TestIntegrationBackendsList(t *testing.T) {
	backends := omnistorage.Backends()

	// File backend should be registered
	found := false
	for _, name := range backends {
		if name == "file" {
			found = true
			break
		}
	}

	if !found {
		t.Error("file backend should be registered")
	}
}

// gzipWriteCloser wraps gzip.Writer to implement io.WriteCloser for ndjson.
type gzipWriteCloser struct {
	*gzip.Writer
}

func (g *gzipWriteCloser) Write(p []byte) (n int, err error) {
	return g.Writer.Write(p)
}

func (g *gzipWriteCloser) Close() error {
	return g.Writer.Close()
}

// gzipReadCloser wraps gzip.Reader to implement io.ReadCloser for ndjson.
type gzipReadCloser struct {
	*gzip.Reader
}

func (g *gzipReadCloser) Read(p []byte) (n int, err error) {
	return g.Reader.Read(p)
}

func (g *gzipReadCloser) Close() error {
	return g.Reader.Close()
}
