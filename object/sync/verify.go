package sync

import (
	"context"
	"fmt"
	"io"
	"path"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

// Verify checks if source and destination are in sync.
//
// Returns true if all files match, false otherwise.
// This is a simplified interface to Check for common use cases.
//
// By default, files are compared by size. Use opts.Checksum for
// content-based verification (slower but more accurate).
func Verify(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (bool, error) {
	result, err := Check(ctx, src, dst, srcPath, dstPath, opts)
	if err != nil {
		return false, err
	}
	return result.InSync(), nil
}

// VerifyFile checks if a single file matches between source and destination.
//
// Returns true if the files are identical, false otherwise.
// Uses checksum comparison for accurate verification.
func VerifyFile(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) (bool, error) {
	// Check if both files exist
	srcExists, err := src.Exists(ctx, srcPath)
	if err != nil {
		return false, err
	}
	if !srcExists {
		return false, fmt.Errorf("source file not found: %s", srcPath)
	}

	dstExists, err := dst.Exists(ctx, dstPath)
	if err != nil {
		return false, err
	}
	if !dstExists {
		return false, nil // Destination doesn't exist, not matching
	}

	// Get file info from both
	srcExt, srcHasExt := omnistorage.AsExtended(src)
	dstExt, dstHasExt := omnistorage.AsExtended(dst)

	// First try size comparison
	if srcHasExt && dstHasExt {
		srcInfo, err := srcExt.Stat(ctx, srcPath)
		if err != nil {
			return false, err
		}
		dstInfo, err := dstExt.Stat(ctx, dstPath)
		if err != nil {
			return false, err
		}

		// Different sizes = definitely different
		if srcInfo.Size() != dstInfo.Size() {
			return false, nil
		}

		// Try hash comparison if available
		srcHash := srcInfo.Hash(omnistorage.HashMD5)
		dstHash := dstInfo.Hash(omnistorage.HashMD5)
		if srcHash != "" && dstHash != "" {
			return srcHash == dstHash, nil
		}
	}

	// Fall back to content comparison
	return compareContent(ctx, src, dst, srcPath, dstPath, "", "")
}

// VerifyChecksum verifies files using content checksum comparison.
//
// This is more accurate than size/time comparison but slower as it
// reads all file contents.
func VerifyChecksum(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) (bool, error) {
	return Verify(ctx, src, dst, srcPath, dstPath, Options{Checksum: true})
}

// VerifyResult contains detailed verification results.
type VerifyResult struct {
	// Verified is true if all files match.
	Verified bool

	// TotalFiles is the total number of files checked.
	TotalFiles int

	// MatchingFiles is the number of files that match.
	MatchingFiles int

	// MismatchedFiles lists files that don't match.
	MismatchedFiles []string

	// MissingInDst lists files missing from destination.
	MissingInDst []string

	// ExtraInDst lists files in destination but not in source.
	ExtraInDst []string

	// Errors contains any errors encountered during verification.
	Errors []FileError
}

// VerifyWithDetails performs verification and returns detailed results.
//
// This is useful when you need to know exactly what differs, not just
// whether everything matches.
func VerifyWithDetails(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (*VerifyResult, error) {
	checkResult, err := Check(ctx, src, dst, srcPath, dstPath, opts)
	if err != nil {
		return nil, err
	}

	result := &VerifyResult{
		Verified:        checkResult.InSync(),
		MatchingFiles:   len(checkResult.Match),
		MismatchedFiles: checkResult.Differ,
		MissingInDst:    checkResult.SrcOnly,
		ExtraInDst:      checkResult.DstOnly,
		Errors:          checkResult.Errors,
	}
	result.TotalFiles = result.MatchingFiles + len(result.MismatchedFiles) +
		len(result.MissingInDst) + len(result.ExtraInDst)

	return result, nil
}

// QuickVerify performs a fast verification using only file metadata.
//
// Compares files by size only, which is very fast but may miss
// files with identical sizes but different content.
func QuickVerify(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) (bool, error) {
	return Verify(ctx, src, dst, srcPath, dstPath, Options{SizeOnly: true})
}

// VerifyAndReport verifies and returns a human-readable report.
func VerifyAndReport(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (string, error) {
	result, err := VerifyWithDetails(ctx, src, dst, srcPath, dstPath, opts)
	if err != nil {
		return "", err
	}

	if result.Verified {
		return fmt.Sprintf("Verified: %d files match", result.MatchingFiles), nil
	}

	report := "Verification failed:\n"
	report += fmt.Sprintf("  Matching: %d files\n", result.MatchingFiles)

	if len(result.MismatchedFiles) > 0 {
		report += fmt.Sprintf("  Mismatched: %d files\n", len(result.MismatchedFiles))
		for _, f := range result.MismatchedFiles {
			report += fmt.Sprintf("    - %s\n", f)
		}
	}

	if len(result.MissingInDst) > 0 {
		report += fmt.Sprintf("  Missing in destination: %d files\n", len(result.MissingInDst))
		for _, f := range result.MissingInDst {
			report += fmt.Sprintf("    - %s\n", f)
		}
	}

	if len(result.ExtraInDst) > 0 {
		report += fmt.Sprintf("  Extra in destination: %d files\n", len(result.ExtraInDst))
		for _, f := range result.ExtraInDst {
			report += fmt.Sprintf("    - %s\n", f)
		}
	}

	return report, nil
}

// VerifyIntegrity checks that a file's content is readable and intact.
//
// This reads the entire file to verify it can be read without errors.
// Useful for detecting corrupted files in storage.
func VerifyIntegrity(ctx context.Context, backend omnistorage.Backend, filePath string) error {
	reader, err := backend.NewReader(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Read entire file to verify integrity
	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		_, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error at file %s: %w", filePath, err)
		}
	}

	return nil
}

// VerifyAllIntegrity checks integrity of all files under a path.
func VerifyAllIntegrity(ctx context.Context, backend omnistorage.Backend, basePath string) ([]string, error) {
	paths, err := backend.List(ctx, basePath)
	if err != nil {
		return nil, err
	}

	var corrupted []string
	for _, p := range paths {
		fullPath := p
		if basePath != "" && len(p) > len(basePath) {
			// Path is already full
		} else if basePath != "" {
			fullPath = path.Join(basePath, p)
		}

		if err := VerifyIntegrity(ctx, backend, fullPath); err != nil {
			corrupted = append(corrupted, p)
		}
	}

	return corrupted, nil
}
