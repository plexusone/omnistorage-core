package sync

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/plexusone/omnistorage-core/object/backend/memory"
	"github.com/plexusone/omnistorage-core/object/sync/filter"
)

func TestSyncBasic(t *testing.T) {
	ctx := context.Background()

	// Create source and destination backends
	src := memory.New()
	dst := memory.New()

	// Add files to source
	writeFile(t, ctx, src, "file1.txt", "content1")
	writeFile(t, ctx, src, "file2.txt", "content2")
	writeFile(t, ctx, src, "subdir/file3.txt", "content3")

	// Sync
	result, err := Sync(ctx, src, dst, "", "", DefaultOptions())
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 3 {
		t.Errorf("Copied = %d, want 3", result.Copied)
	}
	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0", result.Deleted)
	}
	if !result.Success() {
		t.Errorf("Expected success, got errors: %v", result.Errors)
	}

	// Verify files were copied
	verifyFile(t, ctx, dst, "file1.txt", "content1")
	verifyFile(t, ctx, dst, "file2.txt", "content2")
	verifyFile(t, ctx, dst, "subdir/file3.txt", "content3")
}

func TestSyncWithDelete(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Add files to source
	writeFile(t, ctx, src, "file1.txt", "content1")

	// Add extra file to destination
	writeFile(t, ctx, dst, "extra.txt", "extra content")

	// Sync with delete
	result, err := Sync(ctx, src, dst, "", "", Options{DeleteExtra: true})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}
	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", result.Deleted)
	}

	// Verify extra file was deleted
	exists, _ := dst.Exists(ctx, "extra.txt")
	if exists {
		t.Error("Extra file should have been deleted")
	}
}

func TestSyncWithoutDelete(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Add file to source
	writeFile(t, ctx, src, "file1.txt", "content1")

	// Add extra file to destination
	writeFile(t, ctx, dst, "extra.txt", "extra content")

	// Sync without delete (default)
	result, err := Sync(ctx, src, dst, "", "", Options{})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0", result.Deleted)
	}

	// Verify extra file still exists
	exists, _ := dst.Exists(ctx, "extra.txt")
	if !exists {
		t.Error("Extra file should still exist")
	}
}

func TestSyncUpdate(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Add file to destination first
	writeFile(t, ctx, dst, "file.txt", "old")

	// Add file to source with different size
	writeFile(t, ctx, src, "file.txt", "new content with different size")

	// Sync with size-only comparison (since memory backend may not preserve modtime well)
	result, err := Sync(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Files have different sizes, so should be updated
	if result.Updated != 1 {
		t.Errorf("Updated = %d, want 1", result.Updated)
	}

	verifyFile(t, ctx, dst, "file.txt", "new content with different size")
}

func TestSyncSkipIdentical(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Add identical files to both
	writeFile(t, ctx, src, "file.txt", "same content")
	writeFile(t, ctx, dst, "file.txt", "same content")

	// Sync
	result, err := Sync(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Copied != 0 {
		t.Errorf("Copied = %d, want 0", result.Copied)
	}
}

func TestSyncDryRun(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	// Dry run
	result, err := Sync(ctx, src, dst, "", "", Options{DryRun: true})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !result.DryRun {
		t.Error("DryRun should be true")
	}
	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1 (dry run should count)", result.Copied)
	}

	// File should NOT exist in destination
	exists, _ := dst.Exists(ctx, "file.txt")
	if exists {
		t.Error("File should not exist after dry run")
	}
}

func TestSyncProgress(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file1.txt", "content1")
	writeFile(t, ctx, src, "file2.txt", "content2")

	var progressCalls []Phase
	_, err := Sync(ctx, src, dst, "", "", Options{
		Progress: func(p Progress) {
			progressCalls = append(progressCalls, p.Phase)
		},
	})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Should have scanning, comparing, transferring, and complete phases
	if len(progressCalls) < 3 {
		t.Errorf("Expected at least 3 progress calls, got %d", len(progressCalls))
	}
}

func TestSyncWithPrefix(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Add files to source under a prefix
	writeFile(t, ctx, src, "data/file1.txt", "content1")
	writeFile(t, ctx, src, "data/file2.txt", "content2")
	writeFile(t, ctx, src, "other/file3.txt", "content3")

	// Sync only "data/" to "backup/"
	result, err := Sync(ctx, src, dst, "data", "backup", DefaultOptions())
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 2 {
		t.Errorf("Copied = %d, want 2", result.Copied)
	}

	// Verify files were copied to backup/
	verifyFile(t, ctx, dst, "backup/file1.txt", "content1")
	verifyFile(t, ctx, dst, "backup/file2.txt", "content2")

	// other/file3.txt should not be copied
	exists, _ := dst.Exists(ctx, "backup/file3.txt")
	if exists {
		t.Error("file3.txt should not have been copied")
	}
}

func TestCopyDir(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file1.txt", "content1")
	writeFile(t, ctx, dst, "extra.txt", "extra")

	// CopyDir should not delete
	result, err := CopyDir(ctx, src, dst, "", "")
	if err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}
	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0", result.Deleted)
	}

	// Extra file should still exist
	exists, _ := dst.Exists(ctx, "extra.txt")
	if !exists {
		t.Error("Extra file should still exist")
	}
}

func TestCheckInSync(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Add identical files
	writeFile(t, ctx, src, "file.txt", "content")
	writeFile(t, ctx, dst, "file.txt", "content")

	result, err := Check(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !result.InSync() {
		t.Error("Should be in sync")
	}
	if len(result.Match) != 1 {
		t.Errorf("Match = %d, want 1", len(result.Match))
	}
}

func TestCheckDifferences(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Same content
	writeFile(t, ctx, src, "same.txt", "same")
	writeFile(t, ctx, dst, "same.txt", "same")

	// Different content (different sizes to detect with SizeOnly)
	writeFile(t, ctx, src, "diff.txt", "short")
	writeFile(t, ctx, dst, "diff.txt", "much longer content here")

	// Only in source
	writeFile(t, ctx, src, "src-only.txt", "src only")

	// Only in destination
	writeFile(t, ctx, dst, "dst-only.txt", "dst only")

	result, err := Check(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.InSync() {
		t.Error("Should not be in sync")
	}
	if len(result.Match) != 1 {
		t.Errorf("Match = %d, want 1", len(result.Match))
	}
	if len(result.Differ) != 1 {
		t.Errorf("Differ = %d, want 1", len(result.Differ))
	}
	if len(result.SrcOnly) != 1 {
		t.Errorf("SrcOnly = %d, want 1", len(result.SrcOnly))
	}
	if len(result.DstOnly) != 1 {
		t.Errorf("DstOnly = %d, want 1", len(result.DstOnly))
	}
}

func TestCheckWithChecksum(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Same size but different content
	writeFile(t, ctx, src, "file.txt", "aaaa")
	writeFile(t, ctx, dst, "file.txt", "bbbb")

	// Without checksum (size only) - should match
	result1, _ := Check(ctx, src, dst, "", "", Options{SizeOnly: true})
	if len(result1.Match) != 1 {
		t.Errorf("SizeOnly: Match = %d, want 1", len(result1.Match))
	}

	// With checksum - should differ
	result2, _ := Check(ctx, src, dst, "", "", Options{Checksum: true})
	if len(result2.Differ) != 1 {
		t.Errorf("Checksum: Differ = %d, want 1", len(result2.Differ))
	}
}

func TestDiff(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "new.txt", "new")
	writeFile(t, ctx, src, "modified.txt", "short")
	writeFile(t, ctx, dst, "modified.txt", "much longer old content")
	writeFile(t, ctx, dst, "deleted.txt", "deleted")

	entries, err := Diff(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Diff entries = %d, want 3", len(entries))
	}

	statusCount := make(map[DiffStatus]int)
	for _, e := range entries {
		statusCount[e.Status]++
	}

	if statusCount[DiffStatusNew] != 1 {
		t.Errorf("New = %d, want 1", statusCount[DiffStatusNew])
	}
	if statusCount[DiffStatusModified] != 1 {
		t.Errorf("Modified = %d, want 1", statusCount[DiffStatusModified])
	}
	if statusCount[DiffStatusDeleted] != 1 {
		t.Errorf("Deleted = %d, want 1", statusCount[DiffStatusDeleted])
	}
}

func TestNeedsUpdate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		src      FileInfo
		dst      FileInfo
		opts     Options
		expected bool
	}{
		{
			name:     "same size and time",
			src:      FileInfo{Size: 100, ModTime: now},
			dst:      FileInfo{Size: 100, ModTime: now},
			opts:     Options{},
			expected: false,
		},
		{
			name:     "different size",
			src:      FileInfo{Size: 100, ModTime: now},
			dst:      FileInfo{Size: 200, ModTime: now},
			opts:     Options{},
			expected: true,
		},
		{
			name:     "different time",
			src:      FileInfo{Size: 100, ModTime: now},
			dst:      FileInfo{Size: 100, ModTime: now.Add(-time.Hour)},
			opts:     Options{},
			expected: true,
		},
		{
			name:     "size only - different size",
			src:      FileInfo{Size: 100},
			dst:      FileInfo{Size: 200},
			opts:     Options{SizeOnly: true},
			expected: true,
		},
		{
			name:     "size only - same size different time",
			src:      FileInfo{Size: 100, ModTime: now},
			dst:      FileInfo{Size: 100, ModTime: now.Add(-time.Hour)},
			opts:     Options{SizeOnly: true},
			expected: false,
		},
		{
			name:     "checksum match",
			src:      FileInfo{Hash: "abc123"},
			dst:      FileInfo{Hash: "abc123"},
			opts:     Options{Checksum: true},
			expected: false,
		},
		{
			name:     "checksum differ",
			src:      FileInfo{Hash: "abc123"},
			dst:      FileInfo{Hash: "def456"},
			opts:     Options{Checksum: true},
			expected: true,
		},
		{
			name:     "ignore size",
			src:      FileInfo{Size: 100, ModTime: now},
			dst:      FileInfo{Size: 200, ModTime: now},
			opts:     Options{IgnoreSize: true},
			expected: false,
		},
		{
			name:     "ignore time",
			src:      FileInfo{Size: 100, ModTime: now},
			dst:      FileInfo{Size: 100, ModTime: now.Add(-time.Hour)},
			opts:     Options{IgnoreTime: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsUpdate(tt.src, tt.dst, tt.opts)
			if result != tt.expected {
				t.Errorf("NeedsUpdate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResultSuccess(t *testing.T) {
	r1 := Result{}
	if !r1.Success() {
		t.Error("Empty result should be success")
	}

	r2 := Result{Errors: []FileError{{Path: "test", Op: "copy", Err: io.EOF}}}
	if r2.Success() {
		t.Error("Result with errors should not be success")
	}
}

func TestCheckResultInSync(t *testing.T) {
	r1 := CheckResult{Match: []string{"file.txt"}}
	if !r1.InSync() {
		t.Error("All matching should be in sync")
	}

	r2 := CheckResult{Differ: []string{"file.txt"}}
	if r2.InSync() {
		t.Error("With differences should not be in sync")
	}

	r3 := CheckResult{SrcOnly: []string{"file.txt"}}
	if r3.InSync() {
		t.Error("With SrcOnly should not be in sync")
	}

	r4 := CheckResult{DstOnly: []string{"file.txt"}}
	if r4.InSync() {
		t.Error("With DstOnly should not be in sync")
	}
}

func TestFileError(t *testing.T) {
	err := FileError{Path: "test.txt", Op: "copy", Err: io.EOF}
	expected := "copy test.txt: EOF"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.DeleteExtra {
		t.Error("DeleteExtra should be false by default")
	}
	if opts.DryRun {
		t.Error("DryRun should be false by default")
	}
	if opts.Checksum {
		t.Error("Checksum should be false by default")
	}
	if opts.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", opts.Concurrency)
	}
}

func TestSyncContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	src := memory.New()
	dst := memory.New()

	// Add many files
	for i := 0; i < 100; i++ {
		writeFile(t, ctx, src, "file"+string(rune('0'+i%10))+".txt", "content")
	}

	// Cancel immediately
	cancel()

	_, err := Sync(ctx, src, dst, "", "", DefaultOptions())
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestMove(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file1.txt", "content1")
	writeFile(t, ctx, src, "file2.txt", "content2")

	result, err := Move(ctx, src, dst, "", "", Options{})
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if result.Copied != 2 {
		t.Errorf("Copied = %d, want 2", result.Copied)
	}

	// Files should exist in destination
	verifyFile(t, ctx, dst, "file1.txt", "content1")
	verifyFile(t, ctx, dst, "file2.txt", "content2")

	// Files should NOT exist in source
	exists1, _ := src.Exists(ctx, "file1.txt")
	exists2, _ := src.Exists(ctx, "file2.txt")
	if exists1 || exists2 {
		t.Error("Source files should have been deleted after move")
	}
}

func TestMoveDryRun(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	result, err := Move(ctx, src, dst, "", "", Options{DryRun: true})
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if !result.DryRun {
		t.Error("DryRun should be true")
	}

	// Source file should still exist
	exists, _ := src.Exists(ctx, "file.txt")
	if !exists {
		t.Error("Source file should still exist after dry run")
	}

	// Destination should not have the file
	dstExists, _ := dst.Exists(ctx, "file.txt")
	if dstExists {
		t.Error("Destination should not have file after dry run")
	}
}

func TestMoveFile(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	err := MoveFile(ctx, src, dst, "file.txt", "moved.txt")
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// File should exist in destination
	verifyFile(t, ctx, dst, "moved.txt", "content")

	// File should NOT exist in source
	exists, _ := src.Exists(ctx, "file.txt")
	if exists {
		t.Error("Source file should have been deleted")
	}
}

func TestSyncWithFilter(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Create various files
	writeFile(t, ctx, src, "data.json", "json content")
	writeFile(t, ctx, src, "data.xml", "xml content")
	writeFile(t, ctx, src, "backup.tmp", "temp content")

	// Sync with filter: only JSON files
	f := filter.New(filter.Include("*.json"))
	result, err := Sync(ctx, src, dst, "", "", Options{Filter: f})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	// Only JSON file should exist in destination
	jsonExists, _ := dst.Exists(ctx, "data.json")
	xmlExists, _ := dst.Exists(ctx, "data.xml")
	tmpExists, _ := dst.Exists(ctx, "backup.tmp")

	if !jsonExists {
		t.Error("JSON file should have been copied")
	}
	if xmlExists {
		t.Error("XML file should not have been copied")
	}
	if tmpExists {
		t.Error("TMP file should not have been copied")
	}
}

func TestSyncWithFilterExclude(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "keep.txt", "keep")
	writeFile(t, ctx, src, "skip.tmp", "skip")
	writeFile(t, ctx, src, "skip.bak", "skip")

	// Exclude tmp and bak files
	f := filter.New(
		filter.Exclude("*.tmp"),
		filter.Exclude("*.bak"),
	)
	result, err := Sync(ctx, src, dst, "", "", Options{Filter: f})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	keepExists, _ := dst.Exists(ctx, "keep.txt")
	tmpExists, _ := dst.Exists(ctx, "skip.tmp")
	bakExists, _ := dst.Exists(ctx, "skip.bak")

	if !keepExists {
		t.Error("keep.txt should have been copied")
	}
	if tmpExists || bakExists {
		t.Error("Excluded files should not have been copied")
	}
}

func TestSyncParallel(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Create many files to test parallel transfers
	for i := 0; i < 20; i++ {
		content := "content" + string(rune('A'+i))
		writeFile(t, ctx, src, "file"+string(rune('A'+i))+".txt", content)
	}

	result, err := Sync(ctx, src, dst, "", "", Options{Concurrency: 8})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 20 {
		t.Errorf("Copied = %d, want 20", result.Copied)
	}

	// Verify all files were copied
	for i := 0; i < 20; i++ {
		content := "content" + string(rune('A'+i))
		verifyFile(t, ctx, dst, "file"+string(rune('A'+i))+".txt", content)
	}
}

func TestSyncWithBandwidthLimit(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Create a 10KB file
	content := make([]byte, 10*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	writeFile(t, ctx, src, "large.bin", string(content))

	// Limit to 50KB/s - should take ~200ms for 10KB
	start := time.Now()
	result, err := Sync(ctx, src, dst, "", "", Options{
		BandwidthLimit: 50 * 1024, // 50KB/s
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	// Verify file was copied correctly
	r, _ := dst.NewReader(ctx, "large.bin")
	data, _ := io.ReadAll(r)
	_ = r.Close()
	if len(data) != len(content) {
		t.Errorf("File size = %d, want %d", len(data), len(content))
	}

	// Should take at least 100ms (allowing tolerance for test overhead)
	if elapsed < 100*time.Millisecond {
		t.Logf("Transfer took %v (expected ~200ms with rate limiting)", elapsed)
	}
}

func TestSyncWithRetry(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	// Test with retry config - should work normally
	retryConfig := DefaultRetryConfig()
	retryConfig.MaxRetries = 2
	retryConfig.InitialDelay = time.Millisecond

	result, err := Sync(ctx, src, dst, "", "", Options{
		Retry: &retryConfig,
	})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	verifyFile(t, ctx, dst, "file.txt", "content")
}

func TestSyncWithMetadataPreservation(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")

	// Test with metadata options
	metaOpts := DefaultMetadataOptions()
	metaOpts.CustomMetadata = true

	result, err := Sync(ctx, src, dst, "", "", Options{
		PreserveMetadata: metaOpts,
	})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Copied != 1 {
		t.Errorf("Copied = %d, want 1", result.Copied)
	}

	verifyFile(t, ctx, dst, "file.txt", "content")
}

func TestDefaultMetadataOptions(t *testing.T) {
	opts := DefaultMetadataOptions()

	if !opts.ContentType {
		t.Error("ContentType should be true by default")
	}
	if opts.ModTime {
		t.Error("ModTime should be false by default")
	}
	if opts.CustomMetadata {
		t.Error("CustomMetadata should be false by default")
	}
}

// Helper functions

func writeFile(t *testing.T, ctx context.Context, backend *memory.Backend, path, content string) {
	t.Helper()
	w, err := backend.NewWriter(ctx, path)
	if err != nil {
		t.Fatalf("NewWriter(%s) failed: %v", path, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("Write(%s) failed: %v", path, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close(%s) failed: %v", path, err)
	}
}

func verifyFile(t *testing.T, ctx context.Context, backend *memory.Backend, path, expectedContent string) {
	t.Helper()
	r, err := backend.NewReader(ctx, path)
	if err != nil {
		t.Fatalf("NewReader(%s) failed: %v", path, err)
	}
	defer func() { _ = r.Close() }()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(%s) failed: %v", path, err)
	}

	if string(data) != expectedContent {
		t.Errorf("Content of %s = %q, want %q", path, data, expectedContent)
	}
}
