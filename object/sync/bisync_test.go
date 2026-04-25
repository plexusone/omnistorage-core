package sync

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/plexusone/omnistorage-core/object/backend/memory"
)

func TestBisyncNewFiles(t *testing.T) {
	ctx := context.Background()

	// Create two memory backends
	backend1 := memory.New()
	backend2 := memory.New()

	// Add file only in backend1
	w1, _ := backend1.NewWriter(ctx, "path1/file1.txt")
	_, _ = w1.Write([]byte("content from path1"))
	_ = w1.Close()

	// Add file only in backend2
	w2, _ := backend2.NewWriter(ctx, "path2/file2.txt")
	_, _ = w2.Write([]byte("content from path2"))
	_ = w2.Close()

	// Run bisync
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", BisyncOptions{
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Verify results
	if result.CopiedToPath2 != 1 {
		t.Errorf("Expected 1 file copied to path2, got %d", result.CopiedToPath2)
	}
	if result.CopiedToPath1 != 1 {
		t.Errorf("Expected 1 file copied to path1, got %d", result.CopiedToPath1)
	}

	// Verify files exist on both sides
	exists1, _ := backend1.Exists(ctx, "path1/file2.txt")
	if !exists1 {
		t.Error("file2.txt should exist in backend1 after bisync")
	}

	exists2, _ := backend2.Exists(ctx, "path2/file1.txt")
	if !exists2 {
		t.Error("file1.txt should exist in backend2 after bisync")
	}
}

func TestBisyncConflictNewerWins(t *testing.T) {
	ctx := context.Background()

	backend1 := memory.New()
	backend2 := memory.New()

	// Add same file to both backends with different content AND sizes
	// to ensure conflict detection even if timestamps aren't preserved
	w1, _ := backend1.NewWriter(ctx, "path1/shared.txt")
	_, _ = w1.Write([]byte("old")) // shorter
	_ = w1.Close()

	// Wait a bit to ensure different timestamps
	time.Sleep(50 * time.Millisecond)

	// Backend2 has newer version with longer content
	w2, _ := backend2.NewWriter(ctx, "path2/shared.txt")
	_, _ = w2.Write([]byte("newer content here")) // longer
	_ = w2.Close()

	// Run bisync with ConflictNewerWins
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", BisyncOptions{
		ConflictStrategy: ConflictNewerWins,
	})
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Should have 1 conflict (due to different sizes)
	if len(result.Conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(result.Conflicts))
	}

	// Newer (backend2) should win - content should be in backend1
	r1, _ := backend1.NewReader(ctx, "path1/shared.txt")
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r1)
	_ = r1.Close()

	if buf.String() != "newer content here" {
		t.Errorf("Expected 'newer content here' in backend1, got '%s'", buf.String())
	}
}

func TestBisyncConflictSourceWins(t *testing.T) {
	ctx := context.Background()

	backend1 := memory.New()
	backend2 := memory.New()

	// Add same file to both
	w1, _ := backend1.NewWriter(ctx, "path1/shared.txt")
	_, _ = w1.Write([]byte("source content"))
	_ = w1.Close()

	w2, _ := backend2.NewWriter(ctx, "path2/shared.txt")
	_, _ = w2.Write([]byte("dest content"))
	_ = w2.Close()

	// Run bisync with ConflictSourceWins
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", BisyncOptions{
		ConflictStrategy: ConflictSourceWins,
	})
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Should have 1 conflict
	if len(result.Conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(result.Conflicts))
	}

	// Source (backend1) should win - content should be in backend2
	r2, _ := backend2.NewReader(ctx, "path2/shared.txt")
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r2)
	_ = r2.Close()

	if buf.String() != "source content" {
		t.Errorf("Expected 'source content' in backend2, got '%s'", buf.String())
	}
}

func TestBisyncConflictSkip(t *testing.T) {
	ctx := context.Background()

	backend1 := memory.New()
	backend2 := memory.New()

	// Add same file to both with different sizes
	w1, _ := backend1.NewWriter(ctx, "path1/shared.txt")
	_, _ = w1.Write([]byte("content1"))
	_ = w1.Close()

	w2, _ := backend2.NewWriter(ctx, "path2/shared.txt")
	_, _ = w2.Write([]byte("different content2"))
	_ = w2.Close()

	// Run bisync with ConflictSkip
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", BisyncOptions{
		ConflictStrategy: ConflictSkip,
	})
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Should have 1 conflict with skip resolution
	if len(result.Conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(result.Conflicts))
	}

	if result.Conflicts[0].Resolution != "skipped" {
		t.Errorf("Expected resolution 'skipped', got '%s'", result.Conflicts[0].Resolution)
	}

	// Both files should be unchanged
	r1, _ := backend1.NewReader(ctx, "path1/shared.txt")
	buf1 := new(bytes.Buffer)
	_, _ = buf1.ReadFrom(r1)
	_ = r1.Close()

	if buf1.String() != "content1" {
		t.Errorf("Backend1 content should be unchanged, got '%s'", buf1.String())
	}

	r2, _ := backend2.NewReader(ctx, "path2/shared.txt")
	buf2 := new(bytes.Buffer)
	_, _ = buf2.ReadFrom(r2)
	_ = r2.Close()

	if buf2.String() != "different content2" {
		t.Errorf("Backend2 content should be unchanged, got '%s'", buf2.String())
	}
}

func TestBisyncDryRun(t *testing.T) {
	ctx := context.Background()

	backend1 := memory.New()
	backend2 := memory.New()

	// Add file only in backend1
	w1, _ := backend1.NewWriter(ctx, "path1/file1.txt")
	_, _ = w1.Write([]byte("content"))
	_ = w1.Close()

	// Run bisync with DryRun
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", BisyncOptions{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Should report 1 file to copy
	if result.CopiedToPath2 != 1 {
		t.Errorf("Expected 1 file to copy to path2, got %d", result.CopiedToPath2)
	}

	// But file should NOT actually exist in backend2
	exists, _ := backend2.Exists(ctx, "path2/file1.txt")
	if exists {
		t.Error("file1.txt should NOT exist in backend2 during dry run")
	}
}

func TestBisyncInSync(t *testing.T) {
	ctx := context.Background()

	backend1 := memory.New()
	backend2 := memory.New()

	// Add identical files to both backends
	content := []byte("identical content")

	w1, _ := backend1.NewWriter(ctx, "path1/file.txt")
	_, _ = w1.Write(content)
	_ = w1.Close()

	w2, _ := backend2.NewWriter(ctx, "path2/file.txt")
	_, _ = w2.Write(content)
	_ = w2.Close()

	// Run bisync
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", DefaultBisyncOptions())
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Files should be skipped (already in sync)
	if result.Skipped != 1 {
		t.Errorf("Expected 1 file skipped, got %d", result.Skipped)
	}

	if result.CopiedToPath1 != 0 || result.CopiedToPath2 != 0 {
		t.Errorf("Expected no files copied, got to_path1=%d, to_path2=%d",
			result.CopiedToPath1, result.CopiedToPath2)
	}
}

func TestBisyncConflictError(t *testing.T) {
	ctx := context.Background()

	backend1 := memory.New()
	backend2 := memory.New()

	// Add same file to both with different sizes
	w1, _ := backend1.NewWriter(ctx, "path1/shared.txt")
	_, _ = w1.Write([]byte("content1"))
	_ = w1.Close()

	w2, _ := backend2.NewWriter(ctx, "path2/shared.txt")
	_, _ = w2.Write([]byte("different content2"))
	_ = w2.Close()

	// Run bisync with ConflictError
	result, err := Bisync(ctx, backend1, backend2, "path1", "path2", BisyncOptions{
		ConflictStrategy: ConflictError,
	})
	if err != nil {
		t.Fatalf("Bisync failed: %v", err)
	}

	// Should have 1 conflict with error
	if len(result.Conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(result.Conflicts))
	}

	if result.Conflicts[0].Error == nil {
		t.Error("Expected error in conflict")
	}

	// Should also be recorded in result.Errors
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
}

func TestConflictStrategyConstants(t *testing.T) {
	// Verify conflict strategy constants are distinct
	strategies := []ConflictStrategy{
		ConflictNewerWins,
		ConflictLargerWins,
		ConflictSourceWins,
		ConflictDestWins,
		ConflictKeepBoth,
		ConflictSkip,
		ConflictError,
	}

	seen := make(map[ConflictStrategy]bool)
	for _, s := range strategies {
		if seen[s] {
			t.Errorf("Duplicate conflict strategy value: %d", s)
		}
		seen[s] = true
	}
}

func TestDefaultBisyncOptions(t *testing.T) {
	opts := DefaultBisyncOptions()

	if opts.ConflictStrategy != ConflictNewerWins {
		t.Errorf("Expected default ConflictStrategy to be ConflictNewerWins, got %d", opts.ConflictStrategy)
	}

	if opts.ConflictSuffix != ".conflict" {
		t.Errorf("Expected default ConflictSuffix to be '.conflict', got '%s'", opts.ConflictSuffix)
	}

	if opts.Concurrency != 4 {
		t.Errorf("Expected default Concurrency to be 4, got %d", opts.Concurrency)
	}

	if opts.DryRun != false {
		t.Error("Expected default DryRun to be false")
	}

	if opts.DeleteMissing != false {
		t.Error("Expected default DeleteMissing to be false")
	}
}

func TestBisyncResultMethods(t *testing.T) {
	result := &BisyncResult{
		CopiedToPath1:    2,
		CopiedToPath2:    3,
		UpdatedInPath1:   1,
		UpdatedInPath2:   2,
		DeletedFromPath1: 1,
		DeletedFromPath2: 0,
	}

	if result.TotalCopied() != 5 {
		t.Errorf("Expected TotalCopied to be 5, got %d", result.TotalCopied())
	}

	if result.TotalUpdated() != 3 {
		t.Errorf("Expected TotalUpdated to be 3, got %d", result.TotalUpdated())
	}

	if result.TotalDeleted() != 1 {
		t.Errorf("Expected TotalDeleted to be 1, got %d", result.TotalDeleted())
	}

	if !result.Success() {
		t.Error("Expected Success to be true when no errors")
	}

	result.Errors = append(result.Errors, FileError{Path: "test", Op: "test", Err: nil})
	if result.Success() {
		t.Error("Expected Success to be false when errors exist")
	}
}
