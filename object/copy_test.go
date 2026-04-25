package object_test

import (
	"context"
	"io"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
	"github.com/plexusone/omnistorage-core/object/backend/file"
)

func TestCopyPath(t *testing.T) {
	tmpDir := t.TempDir()
	backend := file.New(file.Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	w, err := backend.NewWriter(ctx, "src.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	srcData := []byte("copy me please")
	if _, err := w.Write(srcData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Copy using CopyPath
	err = omnistorage.CopyPath(ctx, backend, "src.txt", backend, "dst.txt")
	if err != nil {
		t.Fatalf("CopyPath failed: %v", err)
	}

	// Verify destination
	r, err := backend.NewReader(ctx, "dst.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	dstData, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(dstData) != string(srcData) {
		t.Errorf("CopyPath: dst = %q, want %q", dstData, srcData)
	}

	// Verify source still exists
	exists, err := backend.Exists(ctx, "src.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Source should still exist after CopyPath")
	}
}

func TestCopyPathBetweenBackends(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	backend1 := file.New(file.Config{Root: tmpDir1})
	backend2 := file.New(file.Config{Root: tmpDir2})
	defer func() { _ = backend1.Close() }()
	defer func() { _ = backend2.Close() }()

	ctx := context.Background()

	// Create source file in backend1
	w, err := backend1.NewWriter(ctx, "src.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	srcData := []byte("cross-backend copy")
	if _, err := w.Write(srcData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Copy from backend1 to backend2
	err = omnistorage.CopyPath(ctx, backend1, "src.txt", backend2, "dst.txt")
	if err != nil {
		t.Fatalf("CopyPath failed: %v", err)
	}

	// Verify destination in backend2
	r, err := backend2.NewReader(ctx, "dst.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	dstData, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(dstData) != string(srcData) {
		t.Errorf("CopyPath: dst = %q, want %q", dstData, srcData)
	}
}

func TestCopyPathNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := file.New(file.Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	err := omnistorage.CopyPath(ctx, backend, "nonexistent.txt", backend, "dst.txt")
	if !omnistorage.IsNotFound(err) {
		t.Errorf("CopyPath error = %v, want ErrNotFound", err)
	}
}

func TestCopyPathWithHash(t *testing.T) {
	tmpDir := t.TempDir()
	backend := file.New(file.Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	w, err := backend.NewWriter(ctx, "src.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	srcData := []byte("hello world")
	if _, err := w.Write(srcData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Copy with hash verification
	hash, err := omnistorage.CopyPathWithHash(ctx, backend, "src.txt", backend, "dst.txt", omnistorage.HashMD5)
	if err != nil {
		t.Fatalf("CopyPathWithHash failed: %v", err)
	}

	// Verify hash
	expectedHash := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	if hash != expectedHash {
		t.Errorf("CopyPathWithHash hash = %q, want %q", hash, expectedHash)
	}
}

func TestSmartCopy(t *testing.T) {
	tmpDir := t.TempDir()
	backend := file.New(file.Config{Root: tmpDir})
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Create source file
	w, err := backend.NewWriter(ctx, "src.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	srcData := []byte("smart copy test")
	if _, err := w.Write(srcData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// SmartCopy should use server-side copy for file backend
	err = omnistorage.SmartCopy(ctx, backend, "src.txt", backend, "dst.txt")
	if err != nil {
		t.Fatalf("SmartCopy failed: %v", err)
	}

	// Verify destination
	r, err := backend.NewReader(ctx, "dst.txt")
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	dstData, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	_ = r.Close()

	if string(dstData) != string(srcData) {
		t.Errorf("SmartCopy: dst = %q, want %q", dstData, srcData)
	}
}
