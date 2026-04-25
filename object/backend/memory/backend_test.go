package memory

import (
	"context"
	"io"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

func TestNewWriter(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	w, err := backend.NewWriter(ctx, "test.txt")
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

	// Verify data was stored
	r, err := backend.NewReader(ctx, "test.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	readData, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(readData) != string(data) {
		t.Errorf("Read data = %q, want %q", readData, data)
	}
}

func TestNewWriterWithContentType(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	w, err := backend.NewWriter(ctx, "test.json", omnistorage.WithContentType("application/json"))
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	_, _ = w.Write([]byte(`{"key":"value"}`))
	_ = w.Close()

	// Verify content type via Stat
	info, err := backend.Stat(ctx, "test.json")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.ContentType() != "application/json" {
		t.Errorf("ContentType = %q, want %q", info.ContentType(), "application/json")
	}
}

func TestNewReader(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write data first
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("hello world"))
	_ = w.Close()

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
		t.Fatalf("Close failed: %v", err)
	}

	if string(data) != "hello world" {
		t.Errorf("Read data = %q, want %q", data, "hello world")
	}
}

func TestNewReaderNotFound(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	_, err := backend.NewReader(ctx, "nonexistent.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("NewReader error = %v, want ErrNotFound", err)
	}
}

func TestNewReaderWithOffset(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write data first
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("hello world"))
	_ = w.Close()

	// Read with offset
	r, err := backend.NewReader(ctx, "test.txt", omnistorage.WithOffset(6))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	data, _ := io.ReadAll(r)
	_ = r.Close()

	if string(data) != "world" {
		t.Errorf("Read data = %q, want %q", data, "world")
	}
}

func TestNewReaderWithLimit(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write data first
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("hello world"))
	_ = w.Close()

	// Read with limit
	r, err := backend.NewReader(ctx, "test.txt", omnistorage.WithLimit(5))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	data, _ := io.ReadAll(r)
	_ = r.Close()

	if string(data) != "hello" {
		t.Errorf("Read data = %q, want %q", data, "hello")
	}
}

func TestNewReaderWithOffsetAndLimit(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write data first
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("hello world"))
	_ = w.Close()

	// Read with offset and limit
	r, err := backend.NewReader(ctx, "test.txt", omnistorage.WithOffset(3), omnistorage.WithLimit(5))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	data, _ := io.ReadAll(r)
	_ = r.Close()

	if string(data) != "lo wo" {
		t.Errorf("Read data = %q, want %q", data, "lo wo")
	}
}

func TestExists(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Check non-existent
	exists, err := backend.Exists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists = true for non-existent file")
	}

	// Create file
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	// Check existing
	exists, err = backend.Exists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists = false for existing file")
	}
}

func TestDelete(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create file
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	// Delete it
	if err := backend.Delete(ctx, "test.txt"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	exists, _ := backend.Exists(ctx, "test.txt")
	if exists {
		t.Error("File should not exist after delete")
	}
}

func TestDeleteIdempotent(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Delete non-existent should not error
	if err := backend.Delete(ctx, "nonexistent.txt"); err != nil {
		t.Errorf("Delete of non-existent file failed: %v", err)
	}
}

func TestList(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create some files
	files := []string{"a.txt", "b.txt", "subdir/c.txt", "subdir/d.txt"}
	for _, f := range files {
		w, _ := backend.NewWriter(ctx, f)
		_, _ = w.Write([]byte("test"))
		_ = w.Close()
	}

	// List all
	paths, err := backend.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(paths) != len(files) {
		t.Errorf("List returned %d paths, want %d", len(paths), len(files))
	}
}

func TestListWithPrefix(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create some files
	files := []string{"a.txt", "subdir/b.txt", "subdir/c.txt"}
	for _, f := range files {
		w, _ := backend.NewWriter(ctx, f)
		_, _ = w.Write([]byte("test"))
		_ = w.Close()
	}

	// List with prefix
	paths, err := backend.List(ctx, "subdir")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("List returned %d paths, want 2", len(paths))
	}
}

func TestClose(t *testing.T) {
	backend := New()

	ctx := context.Background()

	// Close backend
	if err := backend.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations should fail
	_, err := backend.NewWriter(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("NewWriter after Close error = %v, want ErrBackendClosed", err)
	}

	_, err = backend.NewReader(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("NewReader after Close error = %v, want ErrBackendClosed", err)
	}

	_, err = backend.Exists(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Exists after Close error = %v, want ErrBackendClosed", err)
	}

	err = backend.Delete(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Delete after Close error = %v, want ErrBackendClosed", err)
	}

	_, err = backend.List(ctx, "")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("List after Close error = %v, want ErrBackendClosed", err)
	}
}

func TestStat(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create file
	w, _ := backend.NewWriter(ctx, "test.txt", omnistorage.WithContentType("text/plain"))
	_, _ = w.Write([]byte("hello world"))
	_ = w.Close()

	// Stat the file
	info, err := backend.Stat(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Path() != "test.txt" {
		t.Errorf("Path = %q, want %q", info.Path(), "test.txt")
	}
	if info.Size() != 11 {
		t.Errorf("Size = %d, want %d", info.Size(), 11)
	}
	if info.IsDir() {
		t.Error("IsDir = true, want false")
	}
	if info.ModTime().IsZero() {
		t.Error("ModTime is zero")
	}
	if info.ContentType() != "text/plain" {
		t.Errorf("ContentType = %q, want %q", info.ContentType(), "text/plain")
	}
}

func TestStatNotFound(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	_, err := backend.Stat(ctx, "nonexistent.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Stat error = %v, want ErrNotFound", err)
	}
}

func TestMkdir(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create nested directory
	if err := backend.Mkdir(ctx, "a/b/c"); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Verify directories exist
	for _, dir := range []string{"a", "a/b", "a/b/c"} {
		info, err := backend.Stat(ctx, dir)
		if err != nil {
			t.Errorf("Stat(%q) failed: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q should be a directory", dir)
		}
	}
}

func TestMkdirIdempotent(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create twice should not error
	if err := backend.Mkdir(ctx, "mydir"); err != nil {
		t.Fatalf("First Mkdir failed: %v", err)
	}
	if err := backend.Mkdir(ctx, "mydir"); err != nil {
		t.Errorf("Second Mkdir failed: %v", err)
	}
}

func TestRmdir(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create and remove directory
	_ = backend.Mkdir(ctx, "mydir")

	if err := backend.Rmdir(ctx, "mydir"); err != nil {
		t.Fatalf("Rmdir failed: %v", err)
	}

	// Verify it's gone
	_, err := backend.Stat(ctx, "mydir")
	if err != omnistorage.ErrNotFound {
		t.Error("Directory should not exist after Rmdir")
	}
}

func TestRmdirNotFound(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := backend.Rmdir(ctx, "nonexistent")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Rmdir error = %v, want ErrNotFound", err)
	}
}

func TestRmdirNotEmpty(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create directory with file
	_ = backend.Mkdir(ctx, "mydir")
	w, _ := backend.NewWriter(ctx, "mydir/file.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	err := backend.Rmdir(ctx, "mydir")
	if err == nil {
		t.Error("Rmdir on non-empty directory should fail")
	}
}

func TestCopy(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source
	w, _ := backend.NewWriter(ctx, "src.txt")
	srcData := []byte("copy me")
	_, _ = w.Write(srcData)
	_ = w.Close()

	// Copy
	if err := backend.Copy(ctx, "src.txt", "dst.txt"); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Verify destination
	r, _ := backend.NewReader(ctx, "dst.txt")
	dstData, _ := io.ReadAll(r)
	_ = r.Close()

	if string(dstData) != string(srcData) {
		t.Errorf("Copied data = %q, want %q", dstData, srcData)
	}

	// Verify source still exists
	exists, _ := backend.Exists(ctx, "src.txt")
	if !exists {
		t.Error("Source should still exist after copy")
	}
}

func TestCopyNotFound(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := backend.Copy(ctx, "nonexistent.txt", "dst.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Copy error = %v, want ErrNotFound", err)
	}
}

func TestMove(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source
	w, _ := backend.NewWriter(ctx, "src.txt")
	srcData := []byte("move me")
	_, _ = w.Write(srcData)
	_ = w.Close()

	// Move
	if err := backend.Move(ctx, "src.txt", "dst.txt"); err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// Verify destination
	r, _ := backend.NewReader(ctx, "dst.txt")
	dstData, _ := io.ReadAll(r)
	_ = r.Close()

	if string(dstData) != string(srcData) {
		t.Errorf("Moved data = %q, want %q", dstData, srcData)
	}

	// Verify source is gone
	exists, _ := backend.Exists(ctx, "src.txt")
	if exists {
		t.Error("Source should not exist after move")
	}
}

func TestMoveNotFound(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := backend.Move(ctx, "nonexistent.txt", "dst.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Move error = %v, want ErrNotFound", err)
	}
}

func TestFeatures(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	features := backend.Features()

	if !features.Copy {
		t.Error("Features.Copy = false, want true")
	}
	if !features.Move {
		t.Error("Features.Move = false, want true")
	}
	if !features.Mkdir {
		t.Error("Features.Mkdir = false, want true")
	}
	if !features.Rmdir {
		t.Error("Features.Rmdir = false, want true")
	}
	if !features.Stat {
		t.Error("Features.Stat = false, want true")
	}
	if !features.CanStream {
		t.Error("Features.CanStream = false, want true")
	}
	if !features.RangeRead {
		t.Error("Features.RangeRead = false, want true")
	}
	if !features.ListPrefix {
		t.Error("Features.ListPrefix = false, want true")
	}
}

func TestSizeAndCount(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Initially empty
	if backend.Size() != 0 {
		t.Errorf("Initial Size = %d, want 0", backend.Size())
	}
	if backend.Count() != 0 {
		t.Errorf("Initial Count = %d, want 0", backend.Count())
	}

	// Add some files
	w, _ := backend.NewWriter(ctx, "a.txt")
	_, _ = w.Write([]byte("hello")) // 5 bytes
	_ = w.Close()

	w, _ = backend.NewWriter(ctx, "b.txt")
	_, _ = w.Write([]byte("world!")) // 6 bytes
	_ = w.Close()

	if backend.Size() != 11 {
		t.Errorf("Size = %d, want 11", backend.Size())
	}
	if backend.Count() != 2 {
		t.Errorf("Count = %d, want 2", backend.Count())
	}
}

func TestClear(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Add files
	w, _ := backend.NewWriter(ctx, "a.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	w, _ = backend.NewWriter(ctx, "b.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	// Clear
	backend.Clear()

	if backend.Count() != 0 {
		t.Errorf("Count after Clear = %d, want 0", backend.Count())
	}
}

func TestContextCancellation(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := backend.NewWriter(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("NewWriter with cancelled context error = %v, want context.Canceled", err)
	}

	_, err = backend.NewReader(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("NewReader with cancelled context error = %v, want context.Canceled", err)
	}

	_, err = backend.Exists(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("Exists with cancelled context error = %v, want context.Canceled", err)
	}

	err = backend.Delete(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("Delete with cancelled context error = %v, want context.Canceled", err)
	}

	_, err = backend.List(ctx, "")
	if err != context.Canceled {
		t.Errorf("List with cancelled context error = %v, want context.Canceled", err)
	}

	_, err = backend.Stat(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("Stat with cancelled context error = %v, want context.Canceled", err)
	}
}

func TestValidatePath(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Empty path should fail
	_, err := backend.NewWriter(ctx, "")
	if err != omnistorage.ErrInvalidPath {
		t.Errorf("Empty path error = %v, want ErrInvalidPath", err)
	}

	// Path traversal should fail
	_, err = backend.NewWriter(ctx, "../escape.txt")
	if err != omnistorage.ErrInvalidPath {
		t.Errorf("Path traversal error = %v, want ErrInvalidPath", err)
	}
}

func TestNewFromConfig(t *testing.T) {
	backend, err := NewFromConfig(map[string]string{
		"ignored": "value",
	})
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test that it works
	w, err := backend.NewWriter(ctx, "test.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	exists, err := backend.Exists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("File should exist")
	}
}

func TestRegistry(t *testing.T) {
	// Memory backend should be registered
	if !omnistorage.IsRegistered("memory") {
		t.Error("memory backend should be registered")
	}

	// Open via registry
	backend, err := omnistorage.Open("memory", nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test basic operation
	w, err := backend.NewWriter(ctx, "test.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	_, _ = w.Write([]byte("registry test"))
	_ = w.Close()

	r, err := backend.NewReader(ctx, "test.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	data, _ := io.ReadAll(r)
	_ = r.Close()

	if string(data) != "registry test" {
		t.Errorf("Read data = %q, want %q", data, "registry test")
	}
}

func TestExtendedBackendInterface(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	// Verify backend implements ExtendedBackend
	var _ omnistorage.ExtendedBackend = backend

	// Test AsExtended helper
	ext, ok := omnistorage.AsExtended(backend)
	if !ok {
		t.Error("AsExtended returned false for memory backend")
	}
	if ext == nil {
		t.Error("AsExtended returned nil for memory backend")
	}
}

func TestWriterClosed(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	w, _ := backend.NewWriter(ctx, "test.txt")
	_ = w.Close()

	// Write after close should fail
	_, err := w.Write([]byte("test"))
	if err != omnistorage.ErrWriterClosed {
		t.Errorf("Write after Close error = %v, want ErrWriterClosed", err)
	}
}

func TestReaderClosed(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create file
	w, _ := backend.NewWriter(ctx, "test.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	r, _ := backend.NewReader(ctx, "test.txt")
	_ = r.Close()

	// Read after close should fail
	buf := make([]byte, 10)
	_, err := r.Read(buf)
	if err != omnistorage.ErrReaderClosed {
		t.Errorf("Read after Close error = %v, want ErrReaderClosed", err)
	}
}

func TestPathNormalization(t *testing.T) {
	backend := New()
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Write with various path formats
	w, _ := backend.NewWriter(ctx, "/a/b/c.txt")
	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	// Read with normalized path
	r, err := backend.NewReader(ctx, "a/b/c.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	data, _ := io.ReadAll(r)
	_ = r.Close()

	if string(data) != "test" {
		t.Errorf("Data = %q, want %q", data, "test")
	}
}
