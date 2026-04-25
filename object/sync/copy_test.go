package sync

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/plexusone/omnistorage-core/object/backend/memory"
)

func TestCopySingleFile(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	result, err := Copy(ctx, src, dst, "file.txt", "copied.txt", Options{})
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	verifyFile(t, ctx, dst, "copied.txt", "content")
}

func TestCopyDirectory(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "dir/file1.txt", "content1")
	writeFile(t, ctx, src, "dir/file2.txt", "content2")
	writeFile(t, ctx, src, "dir/sub/file3.txt", "content3")

	result, err := Copy(ctx, src, dst, "dir", "backup", Options{})
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if result.Copied != 3 {
		t.Errorf("Copied = %d, want 3", result.Copied)
	}

	verifyFile(t, ctx, dst, "backup/file1.txt", "content1")
	verifyFile(t, ctx, dst, "backup/file2.txt", "content2")
	verifyFile(t, ctx, dst, "backup/sub/file3.txt", "content3")
}

func TestCopyIgnoreExisting(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "new content")
	writeFile(t, ctx, dst, "file.txt", "old content")

	result, err := Copy(ctx, src, dst, "file.txt", "file.txt", Options{IgnoreExisting: true})
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}

	// Original content should be preserved
	verifyFile(t, ctx, dst, "file.txt", "old content")
}

func TestCopyDryRun(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	result, err := Copy(ctx, src, dst, "file.txt", "copied.txt", Options{DryRun: true})
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if !result.DryRun {
		t.Error("DryRun should be true")
	}
	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	// File should NOT exist
	exists, _ := dst.Exists(ctx, "copied.txt")
	if exists {
		t.Error("File should not exist after dry run")
	}
}

func TestCopyFile(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	err := CopyFile(ctx, src, dst, "file.txt", "copied.txt")
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	verifyFile(t, ctx, dst, "copied.txt", "content")
}

func TestCopyBetweenPaths(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	err := CopyBetweenPaths(ctx, src, dst, "file.txt", "copied.txt")
	if err != nil {
		t.Fatalf("CopyBetweenPaths failed: %v", err)
	}

	verifyFile(t, ctx, dst, "copied.txt", "content")
}

func TestCopyToPath(t *testing.T) {
	ctx := context.Background()

	dst := memory.New()
	content := []byte("hello from reader")

	err := CopyToPath(ctx, dst, bytes.NewReader(content), "file.txt", "text/plain")
	if err != nil {
		t.Fatalf("CopyToPath failed: %v", err)
	}

	verifyFile(t, ctx, dst, "file.txt", "hello from reader")
}

func TestCopyFromPath(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	writeFile(t, ctx, src, "file.txt", "hello world")

	var buf bytes.Buffer
	n, err := CopyFromPath(ctx, src, "file.txt", &buf)
	if err != nil {
		t.Fatalf("CopyFromPath failed: %v", err)
	}

	if n != int64(len("hello world")) {
		t.Errorf("Bytes copied = %d, want %d", n, len("hello world"))
	}
	if buf.String() != "hello world" {
		t.Errorf("Content = %q, want %q", buf.String(), "hello world")
	}
}

func TestTreeCopy(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "data/a.txt", "a")
	writeFile(t, ctx, src, "data/b.txt", "b")
	writeFile(t, ctx, src, "data/sub/c.txt", "c")

	result, err := TreeCopy(ctx, src, dst, "data", "backup", Options{})
	if err != nil {
		t.Fatalf("TreeCopy failed: %v", err)
	}

	if result.Copied != 3 {
		t.Errorf("Copied = %d, want 3", result.Copied)
	}
}

func TestCopyWithProgress(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	var progressCalls int
	_, err := CopyWithProgress(ctx, src, dst, "file.txt", "copied.txt", func(currentFile string, bytesCopied int64) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("CopyWithProgress failed: %v", err)
	}

	if progressCalls == 0 {
		t.Error("Expected progress callback to be called")
	}
}

func TestMustCopy(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	// Should not panic
	result := MustCopy(ctx, src, dst, "file.txt", "copied.txt")
	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}
}

func TestMustCopyPanicsOnError(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Write a file and then close the backend to cause an error
	writeFile(t, ctx, src, "file.txt", "content")

	// Close the destination backend to cause a write error
	_ = dst.Close()

	// Try to copy - should panic due to closed backend
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustCopy should panic on error")
		}
	}()

	_ = MustCopy(ctx, src, dst, "file.txt", "copied.txt")
}

func TestCopyPreservesContent(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Create various types of content
	testCases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"small", "hello"},
		{"medium", "hello world this is a medium sized content"},
		{"binary", string([]byte{0, 1, 2, 3, 255, 254, 253})},
		{"unicode", "Hello 世界 🌍"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srcPath := tc.name + ".txt"
			dstPath := tc.name + "_copy.txt"

			writeFile(t, ctx, src, srcPath, tc.content)

			_, err := Copy(ctx, src, dst, srcPath, dstPath, Options{})
			if err != nil {
				t.Fatalf("Copy failed: %v", err)
			}

			// Read back and verify
			r, _ := dst.NewReader(ctx, dstPath)
			data, _ := io.ReadAll(r)
			_ = r.Close()

			if string(data) != tc.content {
				t.Errorf("Content mismatch for %s", tc.name)
			}
		})
	}
}
