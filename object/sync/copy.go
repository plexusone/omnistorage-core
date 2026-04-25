package sync

import (
	"context"
	"io"
	"path"
	"time"

	omnistorage "github.com/plexusone/omnistorage-core/object"
)

// Copy copies files from source to destination.
//
// If srcPath is a file, it copies that single file.
// If srcPath is a directory (or prefix), it copies all files recursively.
//
// Copy never deletes files from the destination. Use Sync with DeleteExtra=true
// for mirror behavior.
//
// Options that affect Copy:
//   - DryRun: report what would be copied without copying
//   - IgnoreExisting: skip files that already exist in destination
//   - Progress: callback for progress updates
func Copy(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (*Result, error) {
	startTime := time.Now()
	result := &Result{DryRun: opts.DryRun}

	// Check if srcPath is a single file or a directory/prefix
	srcExists, err := src.Exists(ctx, srcPath)
	if err != nil {
		return nil, err
	}

	// Try to determine if it's a single file by checking if List returns just this path
	srcPaths, err := src.List(ctx, srcPath)
	if err != nil {
		return nil, err
	}

	isSingleFile := srcExists && len(srcPaths) == 1 && srcPaths[0] == srcPath

	if isSingleFile {
		// Single file copy
		if opts.IgnoreExisting {
			dstExists, err := dst.Exists(ctx, dstPath)
			if err != nil {
				return nil, err
			}
			if dstExists {
				result.Skipped = 1
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}

		if opts.Progress != nil {
			opts.Progress(Progress{
				Phase:       PhaseTransferring,
				CurrentFile: srcPath,
				TotalFiles:  1,
			})
		}

		if !opts.DryRun {
			if err := copyFile(ctx, src, dst, srcPath, dstPath); err != nil {
				result.Errors = append(result.Errors, FileError{
					Path: srcPath,
					Op:   "copy",
					Err:  err,
				})
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}

		result.Copied = 1

		// Get file size for stats
		if ext, ok := omnistorage.AsExtended(src); ok {
			if info, err := ext.Stat(ctx, srcPath); err == nil {
				result.BytesTransferred = info.Size()
			}
		}

		if opts.Progress != nil {
			opts.Progress(Progress{
				Phase:            PhaseComplete,
				FilesTransferred: 1,
				BytesTransferred: result.BytesTransferred,
			})
		}

		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Directory/prefix copy - delegate to Sync without delete
	opts.DeleteExtra = false
	return Sync(ctx, src, dst, srcPath, dstPath, opts)
}

// CopyFile copies a single file from source to destination backend.
//
// This is a convenience function for copying individual files across backends.
// For copying multiple files, use Copy or Sync.
func CopyFile(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) error {
	return copyFile(ctx, src, dst, srcPath, dstPath)
}

// CopyWithProgress copies files with a simple progress callback.
//
// The callback receives the current file path and bytes copied so far.
// This is a convenience wrapper around Copy for simple progress reporting.
func CopyWithProgress(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, progress func(currentFile string, bytesCopied int64)) (*Result, error) {
	opts := Options{
		Progress: func(p Progress) {
			if progress != nil {
				progress(p.CurrentFile, p.BytesTransferred)
			}
		},
	}
	return Copy(ctx, src, dst, srcPath, dstPath, opts)
}

// MustCopy is like Copy but panics on error.
// Useful for scripts and tests where errors are unexpected.
func MustCopy(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) *Result {
	result, err := Copy(ctx, src, dst, srcPath, dstPath, Options{})
	if err != nil {
		panic(err)
	}
	if !result.Success() {
		panic(result.Errors[0])
	}
	return result
}

// copyFileWithReader copies using a provided reader (for internal use).
func copyFileWithReader(ctx context.Context, dst omnistorage.Backend, reader io.Reader, dstPath string, contentType string) error {
	var writerOpts []omnistorage.WriterOption
	if contentType != "" {
		writerOpts = append(writerOpts, omnistorage.WithContentType(contentType))
	}

	writer, err := dst.NewWriter(ctx, dstPath, writerOpts...)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, reader)
	if err != nil {
		_ = writer.Close()
		return err
	}

	return writer.Close()
}

// CopyToPath copies from a reader to a destination path.
// Useful for copying from any io.Reader source to a backend.
func CopyToPath(ctx context.Context, dst omnistorage.Backend, reader io.Reader, dstPath string, contentType string) error {
	return copyFileWithReader(ctx, dst, reader, dstPath, contentType)
}

// CopyFromPath copies from a backend path to a writer.
// Useful for copying from a backend to any io.Writer destination.
func CopyFromPath(ctx context.Context, src omnistorage.Backend, srcPath string, writer io.Writer) (int64, error) {
	reader, err := src.NewReader(ctx, srcPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = reader.Close() }()

	return io.Copy(writer, reader)
}

// CopyBetweenPaths copies between two paths, potentially on different backends.
// If src and dst are the same backend and support server-side copy, it will be used.
func CopyBetweenPaths(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) error {
	// Check for server-side copy on same backend
	if src == dst {
		if ext, ok := omnistorage.AsExtended(src); ok && ext.Features().Copy {
			return ext.Copy(ctx, srcPath, dstPath)
		}
	}

	// Fall back to streaming copy
	return copyFile(ctx, src, dst, srcPath, dstPath)
}

// TreeCopy copies an entire directory tree, preserving structure.
// Unlike Copy which flattens paths relative to srcPath, TreeCopy preserves
// the full directory structure under dstPath.
func TreeCopy(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (*Result, error) {
	startTime := time.Now()
	result := &Result{DryRun: opts.DryRun}

	// List all source files
	srcPaths, err := src.List(ctx, srcPath)
	if err != nil {
		return nil, err
	}

	if opts.Progress != nil {
		opts.Progress(Progress{
			Phase:      PhaseTransferring,
			TotalFiles: len(srcPaths),
		})
	}

	for i, p := range srcPaths {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		default:
		}

		// Preserve relative path structure
		relPath := p
		if len(srcPath) > 0 && len(p) > len(srcPath) {
			relPath = p[len(srcPath):]
			if len(relPath) > 0 && relPath[0] == '/' {
				relPath = relPath[1:]
			}
		}
		fullDstPath := path.Join(dstPath, relPath)

		if opts.Progress != nil {
			opts.Progress(Progress{
				Phase:            PhaseTransferring,
				CurrentFile:      p,
				FilesTransferred: i,
				TotalFiles:       len(srcPaths),
			})
		}

		if opts.IgnoreExisting {
			exists, _ := dst.Exists(ctx, fullDstPath)
			if exists {
				result.Skipped++
				continue
			}
		}

		if !opts.DryRun {
			if err := copyFile(ctx, src, dst, p, fullDstPath); err != nil {
				result.Errors = append(result.Errors, FileError{
					Path: p,
					Op:   "copy",
					Err:  err,
				})
				if opts.MaxErrors > 0 && len(result.Errors) >= opts.MaxErrors {
					result.Duration = time.Since(startTime)
					return result, nil
				}
				continue
			}
		}

		result.Copied++
	}

	if opts.Progress != nil {
		opts.Progress(Progress{
			Phase:            PhaseComplete,
			FilesTransferred: result.Copied,
		})
	}

	result.Duration = time.Since(startTime)
	return result, nil
}
