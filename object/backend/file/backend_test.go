package file

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

func TestNewWriter(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test writing a file
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

	// Verify file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("File content = %q, want %q", string(content), "hello world")
	}
}

func TestNewWriterCreatesDirs(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir, CreateDirs: true})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test writing to nested path
	w, err := backend.NewWriter(ctx, "a/b/c/test.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	if _, err := w.Write([]byte("nested")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify nested file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, "a", "b", "c", "test.txt"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "nested" {
		t.Errorf("File content = %q, want %q", string(content), "nested")
	}
}

func TestNewReader(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test reading the file
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
		t.Errorf("Read data = %q, want %q", string(data), "hello world")
	}
}

func TestNewReaderNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	_, err := backend.NewReader(ctx, "nonexistent.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("NewReader error = %v, want %v", err, omnistorage.ErrNotFound)
	}
}

func TestNewReaderWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test reading with offset
	r, err := backend.NewReader(ctx, "test.txt", omnistorage.WithOffset(6))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(data) != "world" {
		t.Errorf("Read data = %q, want %q", string(data), "world")
	}
}

func TestNewReaderWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test reading with limit
	r, err := backend.NewReader(ctx, "test.txt", omnistorage.WithLimit(5))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(data) != "hello" {
		t.Errorf("Read data = %q, want %q", string(data), "hello")
	}
}

func TestNewReaderWithOffsetAndLimit(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test reading with offset and limit
	r, err := backend.NewReader(ctx, "test.txt", omnistorage.WithOffset(3), omnistorage.WithLimit(5))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(data) != "lo wo" {
		t.Errorf("Read data = %q, want %q", string(data), "lo wo")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test non-existent file
	exists, err := backend.Exists(ctx, "nonexistent.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists = true for non-existent file, want false")
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test existing file
	exists, err = backend.Exists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists = false for existing file, want true")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Delete the file
	if err := backend.Delete(ctx, "test.txt"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file no longer exists
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should not exist after delete")
	}
}

func TestDeleteIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Delete non-existent file should not error
	if err := backend.Delete(ctx, "nonexistent.txt"); err != nil {
		t.Errorf("Delete of non-existent file failed: %v", err)
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create test files
	files := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
		"subdir/nested/file4.txt",
	}

	for _, f := range files {
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	// List all files
	paths, err := backend.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(paths) != len(files) {
		t.Errorf("List returned %d paths, want %d", len(paths), len(files))
	}

	// Check all expected files are present
	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}

	for _, f := range files {
		if !pathSet[f] {
			t.Errorf("Expected file %q not found in list", f)
		}
	}
}

func TestListWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create test files
	files := []string{
		"file1.txt",
		"subdir/file2.txt",
		"subdir/file3.txt",
	}

	for _, f := range files {
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	// List files with prefix
	paths, err := backend.List(ctx, "subdir")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("List returned %d paths, want 2", len(paths))
	}
}

func TestListEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// List empty directory
	paths, err := backend.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("List returned %d paths, want 0", len(paths))
	}
}

func TestListNonExistentPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// List with non-existent prefix should return empty
	paths, err := backend.List(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("List returned %d paths, want 0", len(paths))
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})

	ctx := context.Background()

	// Close backend
	if err := backend.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// All operations should fail after close
	_, err := backend.NewWriter(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("NewWriter after Close: error = %v, want %v", err, omnistorage.ErrBackendClosed)
	}

	_, err = backend.NewReader(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("NewReader after Close: error = %v, want %v", err, omnistorage.ErrBackendClosed)
	}

	_, err = backend.Exists(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Exists after Close: error = %v, want %v", err, omnistorage.ErrBackendClosed)
	}

	err = backend.Delete(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Delete after Close: error = %v, want %v", err, omnistorage.ErrBackendClosed)
	}

	_, err = backend.List(ctx, "")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("List after Close: error = %v, want %v", err, omnistorage.ErrBackendClosed)
	}
}

func TestValidatePath(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Empty path should fail
	_, err := backend.NewWriter(ctx, "")
	if err != omnistorage.ErrInvalidPath {
		t.Errorf("Empty path: error = %v, want %v", err, omnistorage.ErrInvalidPath)
	}

	// Path traversal should fail
	_, err = backend.NewWriter(ctx, "../escape.txt")
	if err != omnistorage.ErrInvalidPath {
		t.Errorf("Path traversal: error = %v, want %v", err, omnistorage.ErrInvalidPath)
	}

	_, err = backend.NewWriter(ctx, "foo/../../escape.txt")
	if err != omnistorage.ErrInvalidPath {
		t.Errorf("Nested path traversal: error = %v, want %v", err, omnistorage.ErrInvalidPath)
	}
}

func TestContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Operations should fail with cancelled context
	_, err := backend.NewWriter(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("NewWriter with cancelled context: error = %v, want %v", err, context.Canceled)
	}

	_, err = backend.NewReader(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("NewReader with cancelled context: error = %v, want %v", err, context.Canceled)
	}

	_, err = backend.Exists(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("Exists with cancelled context: error = %v, want %v", err, context.Canceled)
	}

	err = backend.Delete(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("Delete with cancelled context: error = %v, want %v", err, context.Canceled)
	}

	_, err = backend.List(ctx, "")
	if err != context.Canceled {
		t.Errorf("List with cancelled context: error = %v, want %v", err, context.Canceled)
	}
}

func TestNewFromConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with default config
	backend, err := NewFromConfig(map[string]string{
		"root": tmpDir,
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

func TestNewFromConfigCreateDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with create_dirs=false
	backend, err := NewFromConfig(map[string]string{
		"root":        tmpDir,
		"create_dirs": "false",
	})
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Writing to nested path should fail when create_dirs is false
	_, err = backend.NewWriter(ctx, "nested/test.txt")
	if err == nil {
		t.Error("Expected error when creating nested path with create_dirs=false")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Root != "." {
		t.Errorf("DefaultConfig Root = %q, want %q", config.Root, ".")
	}
	if !config.CreateDirs {
		t.Error("DefaultConfig CreateDirs = false, want true")
	}
	if config.DirPermissions != 0755 {
		t.Errorf("DefaultConfig DirPermissions = %o, want %o", config.DirPermissions, 0755)
	}
	if config.FilePermissions != 0644 {
		t.Errorf("DefaultConfig FilePermissions = %o, want %o", config.FilePermissions, 0644)
	}
}
