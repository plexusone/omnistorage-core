package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

func TestStat(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("hello world")
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Stat the file
	info, err := backend.Stat(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Path() != "test.txt" {
		t.Errorf("Path() = %q, want %q", info.Path(), "test.txt")
	}
	if info.Size() != int64(len(testData)) {
		t.Errorf("Size() = %d, want %d", info.Size(), len(testData))
	}
	if info.IsDir() {
		t.Error("IsDir() = true, want false")
	}
	if info.ModTime().IsZero() {
		t.Error("ModTime() is zero, want non-zero")
	}
	if info.ContentType() != "text/plain; charset=utf-8" {
		t.Errorf("ContentType() = %q, want %q", info.ContentType(), "text/plain; charset=utf-8")
	}
}

func TestStatDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Stat the directory
	info, err := backend.Stat(ctx, "subdir")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if !info.IsDir() {
		t.Error("IsDir() = false, want true")
	}
}

func TestStatNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	_, err := backend.Stat(ctx, "nonexistent.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Stat error = %v, want ErrNotFound", err)
	}
}

func TestMkdir(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create nested directories
	err := backend.Mkdir(ctx, "a/b/c")
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(filepath.Join(tmpDir, "a", "b", "c"))
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}
}

func TestMkdirIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create directory twice should not error
	if err := backend.Mkdir(ctx, "mydir"); err != nil {
		t.Fatalf("First Mkdir failed: %v", err)
	}
	if err := backend.Mkdir(ctx, "mydir"); err != nil {
		t.Errorf("Second Mkdir failed: %v", err)
	}
}

func TestRmdir(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create and remove directory
	subDir := filepath.Join(tmpDir, "toremove")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("os.Mkdir failed: %v", err)
	}

	err := backend.Rmdir(ctx, "toremove")
	if err != nil {
		t.Fatalf("Rmdir failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("Directory should not exist after Rmdir")
	}
}

func TestRmdirNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := backend.Rmdir(ctx, "nonexistent")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Rmdir error = %v, want ErrNotFound", err)
	}
}

func TestRmdirNotEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create directory with file
	subDir := filepath.Join(tmpDir, "notempty")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("os.Mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("test"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err := backend.Rmdir(ctx, "notempty")
	if err == nil {
		t.Error("Rmdir on non-empty directory should fail")
	}
}

func TestRmdirOnFile(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err := backend.Rmdir(ctx, "file.txt")
	if err == nil {
		t.Error("Rmdir on file should fail")
	}
}

func TestCopy(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	srcData := []byte("copy me")
	if err := os.WriteFile(filepath.Join(tmpDir, "src.txt"), srcData, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Copy file
	err := backend.Copy(ctx, "src.txt", "dst.txt")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Verify destination
	dstData, err := os.ReadFile(filepath.Join(tmpDir, "dst.txt"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(dstData) != string(srcData) {
		t.Errorf("Copied data = %q, want %q", dstData, srcData)
	}

	// Verify source still exists
	if _, err := os.Stat(filepath.Join(tmpDir, "src.txt")); err != nil {
		t.Error("Source file should still exist after copy")
	}
}

func TestCopyToNestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir, CreateDirs: true})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	if err := os.WriteFile(filepath.Join(tmpDir, "src.txt"), []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Copy to nested path
	err := backend.Copy(ctx, "src.txt", "a/b/c/dst.txt")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(filepath.Join(tmpDir, "a", "b", "c", "dst.txt")); err != nil {
		t.Error("Destination file should exist")
	}
}

func TestCopyNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := backend.Copy(ctx, "nonexistent.txt", "dst.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Copy error = %v, want ErrNotFound", err)
	}
}

func TestMove(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	srcData := []byte("move me")
	if err := os.WriteFile(filepath.Join(tmpDir, "src.txt"), srcData, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Move file
	err := backend.Move(ctx, "src.txt", "dst.txt")
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// Verify destination
	dstData, err := os.ReadFile(filepath.Join(tmpDir, "dst.txt"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(dstData) != string(srcData) {
		t.Errorf("Moved data = %q, want %q", dstData, srcData)
	}

	// Verify source is gone
	if _, err := os.Stat(filepath.Join(tmpDir, "src.txt")); !os.IsNotExist(err) {
		t.Error("Source file should not exist after move")
	}
}

func TestMoveToNestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir, CreateDirs: true})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	if err := os.WriteFile(filepath.Join(tmpDir, "src.txt"), []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Move to nested path
	err := backend.Move(ctx, "src.txt", "x/y/z/dst.txt")
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(filepath.Join(tmpDir, "x", "y", "z", "dst.txt")); err != nil {
		t.Error("Destination file should exist")
	}

	// Verify source is gone
	if _, err := os.Stat(filepath.Join(tmpDir, "src.txt")); !os.IsNotExist(err) {
		t.Error("Source file should not exist after move")
	}
}

func TestMoveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := backend.Move(ctx, "nonexistent.txt", "dst.txt")
	if err != omnistorage.ErrNotFound {
		t.Errorf("Move error = %v, want ErrNotFound", err)
	}
}

func TestFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
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

func TestExtendedBackendInterface(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	// Verify backend implements ExtendedBackend
	var _ omnistorage.ExtendedBackend = backend

	// Test AsExtended helper
	ext, ok := omnistorage.AsExtended(backend)
	if !ok {
		t.Error("AsExtended returned false for file backend")
	}
	if ext == nil {
		t.Error("AsExtended returned nil for file backend")
	}
}

func TestStatAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})

	ctx := context.Background()

	// Close backend
	if err := backend.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations should fail
	_, err := backend.Stat(ctx, "test.txt")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Stat after Close error = %v, want ErrBackendClosed", err)
	}

	err = backend.Mkdir(ctx, "test")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Mkdir after Close error = %v, want ErrBackendClosed", err)
	}

	err = backend.Rmdir(ctx, "test")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Rmdir after Close error = %v, want ErrBackendClosed", err)
	}

	err = backend.Copy(ctx, "src", "dst")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Copy after Close error = %v, want ErrBackendClosed", err)
	}

	err = backend.Move(ctx, "src", "dst")
	if err != omnistorage.ErrBackendClosed {
		t.Errorf("Move after Close error = %v, want ErrBackendClosed", err)
	}
}

func TestExtendedContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	backend := New(Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := backend.Stat(ctx, "test.txt")
	if err != context.Canceled {
		t.Errorf("Stat with cancelled context error = %v, want context.Canceled", err)
	}

	err = backend.Mkdir(ctx, "test")
	if err != context.Canceled {
		t.Errorf("Mkdir with cancelled context error = %v, want context.Canceled", err)
	}

	err = backend.Rmdir(ctx, "test")
	if err != context.Canceled {
		t.Errorf("Rmdir with cancelled context error = %v, want context.Canceled", err)
	}

	err = backend.Copy(ctx, "src", "dst")
	if err != context.Canceled {
		t.Errorf("Copy with cancelled context error = %v, want context.Canceled", err)
	}

	err = backend.Move(ctx, "src", "dst")
	if err != context.Canceled {
		t.Errorf("Move with cancelled context error = %v, want context.Canceled", err)
	}
}
