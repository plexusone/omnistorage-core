package object_test

import (
	"context"
	"io"
	"testing"

	omnistorage "github.com/plexusone/omnistorage-core/object"
	"github.com/plexusone/omnistorage-core/object/backend/file"
)

// moveFunc is a function type for move operations.
type moveFunc func(ctx context.Context, src omnistorage.Backend, srcPath string, dst omnistorage.Backend, dstPath string, opts ...omnistorage.WriterOption) error

// testMoveOperation is a helper that tests a move operation function.
func testMoveOperation(t *testing.T, name string, moveFn moveFunc) {
	t.Helper()

	tmpDir := t.TempDir()
	backend := file.New(file.Config{Root: tmpDir})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("backend.Close failed: %v", err)
		}
	})

	ctx := context.Background()

	// Create source file
	w, err := backend.NewWriter(ctx, "src.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	srcData := []byte(name + " test data")
	if _, err := w.Write(srcData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Perform move operation
	if err := moveFn(ctx, backend, "src.txt", backend, "dst.txt"); err != nil {
		t.Fatalf("%s failed: %v", name, err)
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
	if err := r.Close(); err != nil {
		t.Errorf("reader.Close failed: %v", err)
	}

	if string(dstData) != string(srcData) {
		t.Errorf("%s: dst = %q, want %q", name, dstData, srcData)
	}

	// Verify source no longer exists
	exists, err := backend.Exists(ctx, "src.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Errorf("Source should not exist after %s", name)
	}
}

func TestMovePath(t *testing.T) {
	testMoveOperation(t, "MovePath", omnistorage.MovePath)
}

func TestSmartMove(t *testing.T) {
	testMoveOperation(t, "SmartMove", omnistorage.SmartMove)
}

func TestMovePathBetweenBackends(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	backend1 := file.New(file.Config{Root: tmpDir1})
	backend2 := file.New(file.Config{Root: tmpDir2})
	t.Cleanup(func() {
		if err := backend1.Close(); err != nil {
			t.Errorf("backend1.Close failed: %v", err)
		}
		if err := backend2.Close(); err != nil {
			t.Errorf("backend2.Close failed: %v", err)
		}
	})

	ctx := context.Background()

	// Create source file in backend1
	w, err := backend1.NewWriter(ctx, "src.txt")
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	srcData := []byte("cross-backend move")
	if _, err := w.Write(srcData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Move from backend1 to backend2
	if err := omnistorage.MovePath(ctx, backend1, "src.txt", backend2, "dst.txt"); err != nil {
		t.Fatalf("MovePath failed: %v", err)
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
	if err := r.Close(); err != nil {
		t.Errorf("reader.Close failed: %v", err)
	}

	if string(dstData) != string(srcData) {
		t.Errorf("MovePath: dst = %q, want %q", dstData, srcData)
	}

	// Verify source is gone from backend1
	exists, err := backend1.Exists(ctx, "src.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Source should not exist in backend1 after MovePath")
	}
}

func TestMovePathNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := file.New(file.Config{Root: tmpDir})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("backend.Close failed: %v", err)
		}
	})

	ctx := context.Background()

	err := omnistorage.MovePath(ctx, backend, "nonexistent.txt", backend, "dst.txt")
	if !omnistorage.IsNotFound(err) {
		t.Errorf("MovePath error = %v, want ErrNotFound", err)
	}
}
