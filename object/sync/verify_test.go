package sync

import (
	"context"
	"strings"
	"testing"

	"github.com/plexusone/omnistorage-core/object/backend/memory"
)

func TestVerifyInSync(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")
	writeFile(t, ctx, dst, "file.txt", "content")

	verified, err := Verify(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !verified {
		t.Error("Should be verified as in sync")
	}
}

func TestVerifyNotInSync(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")
	// dst is empty

	verified, err := Verify(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if verified {
		t.Error("Should not be verified as in sync")
	}
}

func TestVerifyFileSame(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "same content")
	writeFile(t, ctx, dst, "file.txt", "same content")

	same, err := VerifyFile(ctx, src, dst, "file.txt", "file.txt")
	if err != nil {
		t.Fatalf("VerifyFile failed: %v", err)
	}

	if !same {
		t.Error("Files should be the same")
	}
}

func TestVerifyFileDifferent(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content A")
	writeFile(t, ctx, dst, "file.txt", "content B")

	same, err := VerifyFile(ctx, src, dst, "file.txt", "file.txt")
	if err != nil {
		t.Fatalf("VerifyFile failed: %v", err)
	}

	if same {
		t.Error("Files should be different")
	}
}

func TestVerifyFileMissing(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")
	// dst doesn't have the file

	same, err := VerifyFile(ctx, src, dst, "file.txt", "file.txt")
	if err != nil {
		t.Fatalf("VerifyFile failed: %v", err)
	}

	if same {
		t.Error("Should return false when dst file is missing")
	}
}

func TestVerifyFileSrcMissing(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, dst, "file.txt", "content")
	// src doesn't have the file

	_, err := VerifyFile(ctx, src, dst, "file.txt", "file.txt")
	if err == nil {
		t.Error("Should return error when src file is missing")
	}
}

func TestVerifyChecksum(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Same size but different content
	writeFile(t, ctx, src, "file.txt", "aaaa")
	writeFile(t, ctx, dst, "file.txt", "bbbb")

	verified, err := VerifyChecksum(ctx, src, dst, "", "")
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}

	if verified {
		t.Error("Should not verify - content is different")
	}
}

func TestQuickVerify(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "same")
	writeFile(t, ctx, dst, "file.txt", "same")

	verified, err := QuickVerify(ctx, src, dst, "", "")
	if err != nil {
		t.Fatalf("QuickVerify failed: %v", err)
	}

	if !verified {
		t.Error("Should be verified")
	}
}

func TestVerifyWithDetails(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "same.txt", "same")
	writeFile(t, ctx, dst, "same.txt", "same")
	writeFile(t, ctx, src, "diff.txt", "short")
	writeFile(t, ctx, dst, "diff.txt", "different size content")
	writeFile(t, ctx, src, "src-only.txt", "only in src")
	writeFile(t, ctx, dst, "dst-only.txt", "only in dst")

	result, err := VerifyWithDetails(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("VerifyWithDetails failed: %v", err)
	}

	if result.Verified {
		t.Error("Should not be verified")
	}
	if result.MatchingFiles != 1 {
		t.Errorf("MatchingFiles = %d, want 1", result.MatchingFiles)
	}
	if len(result.MismatchedFiles) != 1 {
		t.Errorf("MismatchedFiles = %d, want 1", len(result.MismatchedFiles))
	}
	if len(result.MissingInDst) != 1 {
		t.Errorf("MissingInDst = %d, want 1", len(result.MissingInDst))
	}
	if len(result.ExtraInDst) != 1 {
		t.Errorf("ExtraInDst = %d, want 1", len(result.ExtraInDst))
	}
	if result.TotalFiles != 4 {
		t.Errorf("TotalFiles = %d, want 4", result.TotalFiles)
	}
}

func TestVerifyAndReportSuccess(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")
	writeFile(t, ctx, dst, "file.txt", "content")

	report, err := VerifyAndReport(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("VerifyAndReport failed: %v", err)
	}

	if !strings.Contains(report, "Verified") {
		t.Errorf("Report should contain 'Verified': %s", report)
	}
}

func TestVerifyAndReportFailure(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	writeFile(t, ctx, src, "file.txt", "content")
	// dst is empty

	report, err := VerifyAndReport(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("VerifyAndReport failed: %v", err)
	}

	if !strings.Contains(report, "failed") {
		t.Errorf("Report should contain 'failed': %s", report)
	}
	if !strings.Contains(report, "Missing in destination") {
		t.Errorf("Report should mention missing files: %s", report)
	}
}

func TestVerifyIntegrity(t *testing.T) {
	ctx := context.Background()

	backend := memory.New()
	writeFile(t, ctx, backend, "file.txt", "content")

	err := VerifyIntegrity(ctx, backend, "file.txt")
	if err != nil {
		t.Errorf("VerifyIntegrity failed: %v", err)
	}
}

func TestVerifyIntegrityMissing(t *testing.T) {
	ctx := context.Background()

	backend := memory.New()

	err := VerifyIntegrity(ctx, backend, "nonexistent.txt")
	if err == nil {
		t.Error("VerifyIntegrity should fail for missing file")
	}
}

func TestVerifyAllIntegrity(t *testing.T) {
	ctx := context.Background()

	backend := memory.New()
	writeFile(t, ctx, backend, "file1.txt", "content1")
	writeFile(t, ctx, backend, "file2.txt", "content2")
	writeFile(t, ctx, backend, "dir/file3.txt", "content3")

	corrupted, err := VerifyAllIntegrity(ctx, backend, "")
	if err != nil {
		t.Fatalf("VerifyAllIntegrity failed: %v", err)
	}

	if len(corrupted) != 0 {
		t.Errorf("Should have no corrupted files, got %v", corrupted)
	}
}

func TestVerifyResultVerified(t *testing.T) {
	result := VerifyResult{
		Verified:      true,
		MatchingFiles: 5,
		TotalFiles:    5,
	}

	if !result.Verified {
		t.Error("Should be verified")
	}
}

func TestVerifyResultNotVerified(t *testing.T) {
	result := VerifyResult{
		Verified:        false,
		MatchingFiles:   3,
		MismatchedFiles: []string{"file.txt"},
		TotalFiles:      4,
	}

	if result.Verified {
		t.Error("Should not be verified")
	}
}

func TestVerifyEmptyDirectories(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Both empty
	verified, err := Verify(ctx, src, dst, "", "", Options{})
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !verified {
		t.Error("Empty directories should verify as in sync")
	}
}

func TestVerifyMultipleFiles(t *testing.T) {
	ctx := context.Background()

	src := memory.New()
	dst := memory.New()

	// Create same files in both
	for i := 0; i < 10; i++ {
		content := strings.Repeat("x", i*100)
		writeFile(t, ctx, src, "file"+string(rune('0'+i))+".txt", content)
		writeFile(t, ctx, dst, "file"+string(rune('0'+i))+".txt", content)
	}

	verified, err := Verify(ctx, src, dst, "", "", Options{SizeOnly: true})
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !verified {
		t.Error("All files should match")
	}
}
