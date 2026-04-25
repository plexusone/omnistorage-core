package sync

import (
	"context"
	"io"
	"path"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

// Check compares files between source and destination backends.
//
// It returns a CheckResult containing:
//   - Match: files that are identical in both
//   - Differ: files that exist in both but have different content
//   - SrcOnly: files that exist only in source
//   - DstOnly: files that exist only in destination
//
// By default, files are compared by size and modification time.
// Set opts.Checksum to true for content-based comparison (slower but more accurate).
func Check(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (*CheckResult, error) {
	result := &CheckResult{}

	// List source files
	srcFiles, err := listFiles(ctx, src, srcPath, opts)
	if err != nil {
		return nil, err
	}

	// List destination files
	dstFiles, err := listFiles(ctx, dst, dstPath, opts)
	if err != nil {
		return nil, err
	}

	// Build destination file map
	dstMap := make(map[string]FileInfo)
	for _, f := range dstFiles {
		if !f.IsDir {
			dstMap[f.Path] = f
		}
	}

	// Compare files
	for _, srcFile := range srcFiles {
		if srcFile.IsDir {
			continue
		}

		dstFile, exists := dstMap[srcFile.Path]
		if !exists {
			result.SrcOnly = append(result.SrcOnly, srcFile.Path)
			continue
		}

		delete(dstMap, srcFile.Path)

		// Compare the files
		same, err := filesMatch(ctx, src, dst, srcFile, dstFile, srcPath, dstPath, opts)
		if err != nil {
			result.Errors = append(result.Errors, FileError{
				Path: srcFile.Path,
				Op:   "compare",
				Err:  err,
			})
			continue
		}

		if same {
			result.Match = append(result.Match, srcFile.Path)
		} else {
			result.Differ = append(result.Differ, srcFile.Path)
		}
	}

	// Remaining files exist only in destination
	for p := range dstMap {
		result.DstOnly = append(result.DstOnly, p)
	}

	return result, nil
}

// filesMatch determines if two files are the same.
func filesMatch(ctx context.Context, src, dst omnistorage.Backend, srcFile, dstFile FileInfo, srcBasePath, dstBasePath string, opts Options) (bool, error) {
	// Quick checks first

	// Size check (unless ignored)
	if !opts.IgnoreSize && srcFile.Size != dstFile.Size {
		return false, nil
	}

	// If we have hashes, use them
	if opts.Checksum && srcFile.Hash != "" && dstFile.Hash != "" {
		return srcFile.Hash == dstFile.Hash, nil
	}

	// Time check (unless ignored or checksum mode)
	if !opts.IgnoreTime && !opts.Checksum {
		timeDiff := srcFile.ModTime.Sub(dstFile.ModTime)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		// Allow 1 second tolerance
		if timeDiff > 1e9 {
			return false, nil
		}
	}

	// If checksum mode and we don't have hashes, compute them by reading content
	if opts.Checksum {
		return compareContent(ctx, src, dst, srcFile.Path, dstFile.Path, srcBasePath, dstBasePath)
	}

	// Size and time match
	return true, nil
}

// compareContent compares two files by reading their content.
func compareContent(ctx context.Context, src, dst omnistorage.Backend, srcRelPath, dstRelPath, srcBasePath, dstBasePath string) (bool, error) {
	srcFullPath := path.Join(srcBasePath, srcRelPath)
	dstFullPath := path.Join(dstBasePath, dstRelPath)

	srcReader, err := src.NewReader(ctx, srcFullPath)
	if err != nil {
		return false, err
	}
	defer func() { _ = srcReader.Close() }()

	dstReader, err := dst.NewReader(ctx, dstFullPath)
	if err != nil {
		return false, err
	}
	defer func() { _ = dstReader.Close() }()

	// Compare content in chunks
	srcBuf := make([]byte, 64*1024) // 64KB chunks
	dstBuf := make([]byte, 64*1024)

	for {
		srcN, srcErr := io.ReadFull(srcReader, srcBuf)
		dstN, dstErr := io.ReadFull(dstReader, dstBuf)

		// Compare what we read
		if srcN != dstN {
			return false, nil
		}

		for i := 0; i < srcN; i++ {
			if srcBuf[i] != dstBuf[i] {
				return false, nil
			}
		}

		// Check for EOF
		srcEOF := srcErr == io.EOF || srcErr == io.ErrUnexpectedEOF
		dstEOF := dstErr == io.EOF || dstErr == io.ErrUnexpectedEOF

		if srcEOF && dstEOF {
			return true, nil // Both ended at the same point
		}
		if srcEOF != dstEOF {
			return false, nil // One ended before the other
		}

		// Handle other errors
		if srcErr != nil && !srcEOF {
			return false, srcErr
		}
		if dstErr != nil && !dstEOF {
			return false, dstErr
		}
	}
}

// Diff returns a human-readable summary of differences between backends.
func Diff(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) ([]DiffEntry, error) {
	checkResult, err := Check(ctx, src, dst, srcPath, dstPath, opts)
	if err != nil {
		return nil, err
	}

	var entries []DiffEntry

	for _, p := range checkResult.SrcOnly {
		entries = append(entries, DiffEntry{
			Path:   p,
			Status: DiffStatusNew,
		})
	}

	for _, p := range checkResult.Differ {
		entries = append(entries, DiffEntry{
			Path:   p,
			Status: DiffStatusModified,
		})
	}

	for _, p := range checkResult.DstOnly {
		entries = append(entries, DiffEntry{
			Path:   p,
			Status: DiffStatusDeleted,
		})
	}

	return entries, nil
}

// DiffEntry represents a single difference between backends.
type DiffEntry struct {
	Path   string
	Status DiffStatus
}

// DiffStatus represents the type of difference.
type DiffStatus string

const (
	// DiffStatusNew indicates a file exists only in source.
	DiffStatusNew DiffStatus = "new"

	// DiffStatusModified indicates a file differs between source and destination.
	DiffStatusModified DiffStatus = "modified"

	// DiffStatusDeleted indicates a file exists only in destination.
	DiffStatusDeleted DiffStatus = "deleted"
)
