package sync

import (
	"context"
	"io"
	"log/slog"
	"path"
	gosync "sync"
	"sync/atomic"
	"time"

	omnistorage "github.com/plexusone/omnistorage-core/object"
	"github.com/plexusone/omnistorage-core/object/sync/filter"
)

// syncContext holds shared state for a sync operation.
type syncContext struct {
	opts        Options
	rateLimiter *tokenBucket
	logger      *slog.Logger
}

// Sync synchronizes files from source to destination.
//
// By default, it copies new and updated files but does not delete extra files
// in the destination. Set Options.DeleteExtra to true to remove files from
// destination that don't exist in source (making destination a mirror of source).
//
// Both backends should support List operation. If the source backend implements
// ExtendedBackend with Stat, it will be used for more accurate file comparison.
func Sync(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (*Result, error) {
	startTime := time.Now()
	result := &Result{DryRun: opts.DryRun}

	// Set default concurrency
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}

	// Get logger
	logger := opts.logger()

	// Create sync context with shared state
	sctx := &syncContext{
		opts:        opts,
		rateLimiter: newTokenBucket(opts.BandwidthLimit),
		logger:      logger,
	}

	logger.Info("starting sync",
		slog.String("src_path", srcPath),
		slog.String("dst_path", dstPath),
		slog.Bool("delete_extra", opts.DeleteExtra),
		slog.Bool("dry_run", opts.DryRun),
		slog.Int("concurrency", opts.Concurrency),
	)

	// Scan source files
	if opts.Progress != nil {
		opts.Progress(Progress{Phase: PhaseScanning, CurrentFile: srcPath})
	}

	logger.Debug("scanning source files", slog.String("path", srcPath))
	srcFiles, err := listFiles(ctx, src, srcPath, opts)
	if err != nil {
		logger.Error("failed to list source files", slog.String("path", srcPath), slog.Any("error", err))
		return nil, err
	}
	logger.Debug("source scan complete", slog.Int("files", len(srcFiles)))

	// Scan destination files
	logger.Debug("scanning destination files", slog.String("path", dstPath))
	dstFiles, err := listFiles(ctx, dst, dstPath, opts)
	if err != nil {
		logger.Error("failed to list destination files", slog.String("path", dstPath), slog.Any("error", err))
		return nil, err
	}
	logger.Debug("destination scan complete", slog.Int("files", len(dstFiles)))

	// Build destination file map for quick lookup
	dstMap := make(map[string]FileInfo)
	for _, f := range dstFiles {
		dstMap[f.Path] = f
	}

	// Compare and determine actions
	if opts.Progress != nil {
		opts.Progress(Progress{Phase: PhaseComparing, TotalFiles: len(srcFiles)})
	}

	type copyAction struct {
		file     FileInfo
		isUpdate bool // true if updating existing file, false if new
	}

	var toCopy []copyAction
	var toDelete []string

	for _, srcFile := range srcFiles {
		if srcFile.IsDir {
			continue // Skip directories, they're created as needed
		}

		dstFile, exists := dstMap[srcFile.Path]
		if !exists {
			// New file
			toCopy = append(toCopy, copyAction{file: srcFile, isUpdate: false})
		} else if !dstFile.IsDir && NeedsUpdate(srcFile, dstFile, opts) {
			// File needs update
			if !opts.IgnoreExisting {
				toCopy = append(toCopy, copyAction{file: srcFile, isUpdate: true})
			} else {
				result.Skipped++
			}
		} else {
			result.Skipped++
		}
		delete(dstMap, srcFile.Path)
	}

	// Remaining files in dstMap exist only in destination
	if opts.DeleteExtra {
		for p, f := range dstMap {
			if !f.IsDir {
				toDelete = append(toDelete, p)
			}
		}
	}

	// Calculate total bytes to transfer
	var totalBytes int64
	for _, action := range toCopy {
		totalBytes += action.file.Size
	}

	// Copy files using worker pool for parallel transfers
	if opts.Progress != nil {
		opts.Progress(Progress{
			Phase:      PhaseTransferring,
			TotalFiles: len(toCopy),
			TotalBytes: totalBytes,
		})
	}

	var bytesTransferred atomic.Int64
	var filesTransferred atomic.Int32
	var copied atomic.Int32
	var updated atomic.Int32
	var errorsMu gosync.Mutex

	// Use worker pool for parallel transfers
	workCh := make(chan copyAction, len(toCopy))
	var wg gosync.WaitGroup

	// Context for cancellation
	copyCtx, cancelCopy := context.WithCancel(ctx)
	defer cancelCopy()

	// Start workers
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for action := range workCh {
				select {
				case <-copyCtx.Done():
					return
				default:
				}

				srcFullPath := path.Join(srcPath, action.file.Path)
				dstFullPath := path.Join(dstPath, action.file.Path)

				if opts.Progress != nil {
					opts.Progress(Progress{
						Phase:            PhaseTransferring,
						CurrentFile:      action.file.Path,
						FilesTransferred: int(filesTransferred.Load()),
						TotalFiles:       len(toCopy),
						BytesTransferred: bytesTransferred.Load(),
						TotalBytes:       totalBytes,
					})
				}

				if !opts.DryRun {
					err := copyFileWithContext(copyCtx, sctx, src, dst, srcFullPath, dstFullPath)
					if err != nil {
						errorsMu.Lock()
						result.Errors = append(result.Errors, FileError{
							Path: action.file.Path,
							Op:   "copy",
							Err:  err,
						})
						shouldStop := opts.MaxErrors > 0 && len(result.Errors) >= opts.MaxErrors
						errorsMu.Unlock()
						if shouldStop {
							cancelCopy()
							return
						}
						continue
					}
				}

				// Determine if this was a new file or update
				if action.isUpdate {
					updated.Add(1)
				} else {
					copied.Add(1)
				}
				bytesTransferred.Add(action.file.Size)
				filesTransferred.Add(1)
			}
		}()
	}

	// Send work to workers
sendLoop:
	for _, action := range toCopy {
		select {
		case <-copyCtx.Done():
			break sendLoop
		case workCh <- action:
		}
	}
	close(workCh)

	// Wait for workers to finish
	wg.Wait()

	result.Copied = int(copied.Load())
	result.Updated = int(updated.Load())
	result.BytesTransferred = bytesTransferred.Load()

	// Check if context was cancelled
	if ctx.Err() != nil {
		result.Duration = time.Since(startTime)
		return result, ctx.Err()
	}

	// Delete extra files
	if opts.DeleteExtra && len(toDelete) > 0 {
		if opts.Progress != nil {
			opts.Progress(Progress{
				Phase:      PhaseDeleting,
				TotalFiles: len(toDelete),
			})
		}

		for _, p := range toDelete {
			select {
			case <-ctx.Done():
				result.Duration = time.Since(startTime)
				return result, ctx.Err()
			default:
			}

			dstFullPath := path.Join(dstPath, p)

			if opts.Progress != nil {
				opts.Progress(Progress{
					Phase:        PhaseDeleting,
					CurrentFile:  p,
					FilesDeleted: result.Deleted,
					TotalFiles:   len(toDelete),
				})
			}

			if !opts.DryRun {
				if err := dst.Delete(ctx, dstFullPath); err != nil {
					result.Errors = append(result.Errors, FileError{
						Path: p,
						Op:   "delete",
						Err:  err,
					})
					if opts.MaxErrors > 0 && len(result.Errors) >= opts.MaxErrors {
						result.Duration = time.Since(startTime)
						return result, nil
					}
					continue
				}
			}
			result.Deleted++
		}
	}

	if opts.Progress != nil {
		opts.Progress(Progress{
			Phase:            PhaseComplete,
			FilesTransferred: result.Copied + result.Updated,
			BytesTransferred: result.BytesTransferred,
			FilesDeleted:     result.Deleted,
			Errors:           len(result.Errors),
		})
	}

	result.Duration = time.Since(startTime)

	logger.Info("sync complete",
		slog.Int("copied", result.Copied),
		slog.Int("updated", result.Updated),
		slog.Int("deleted", result.Deleted),
		slog.Int("skipped", result.Skipped),
		slog.Int("errors", len(result.Errors)),
		slog.Int64("bytes_transferred", result.BytesTransferred),
		slog.Duration("duration", result.Duration),
	)

	return result, nil
}

// CopyDir copies all files from source to destination recursively.
// Unlike Sync, it never deletes files from destination.
// This is equivalent to Sync with DeleteExtra=false.
func CopyDir(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) (*Result, error) {
	return Sync(ctx, src, dst, srcPath, dstPath, Options{
		DeleteExtra: false,
	})
}

// Move moves files from source to destination.
//
// This is like Sync but also deletes files from source after successful copy.
// Use with caution as source files are permanently deleted.
func Move(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string, opts Options) (*Result, error) {
	startTime := time.Now()

	// First, do a sync without deleting from destination
	opts.DeleteExtra = false
	result, err := Sync(ctx, src, dst, srcPath, dstPath, opts)
	if err != nil {
		return result, err
	}

	// If dry run, don't delete source files
	if opts.DryRun {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Delete successfully copied files from source
	srcFiles, err := listFiles(ctx, src, srcPath, opts)
	if err != nil {
		return result, err
	}

	for _, f := range srcFiles {
		if f.IsDir {
			continue
		}

		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		default:
		}

		srcFullPath := path.Join(srcPath, f.Path)

		// Verify the file was copied successfully before deleting
		dstFullPath := path.Join(dstPath, f.Path)
		dstExists, err := dst.Exists(ctx, dstFullPath)
		if err != nil {
			result.Errors = append(result.Errors, FileError{
				Path: f.Path,
				Op:   "verify",
				Err:  err,
			})
			continue
		}

		if !dstExists {
			// File wasn't copied, don't delete source
			continue
		}

		// Delete from source
		if err := src.Delete(ctx, srcFullPath); err != nil {
			result.Errors = append(result.Errors, FileError{
				Path: f.Path,
				Op:   "delete-source",
				Err:  err,
			})
			if opts.MaxErrors > 0 && len(result.Errors) >= opts.MaxErrors {
				break
			}
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// MoveFile moves a single file from source to destination.
// The source file is deleted after successful copy.
func MoveFile(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) error {
	// First try server-side move if both backends are the same and support it
	if src == dst {
		if ext, ok := omnistorage.AsExtended(src); ok && ext.Features().Move {
			return ext.Move(ctx, srcPath, dstPath)
		}
	}

	// Fall back to copy + delete
	if err := copyFile(ctx, src, dst, srcPath, dstPath); err != nil {
		return err
	}

	return src.Delete(ctx, srcPath)
}

// listFiles lists all files under the given path and returns FileInfo for each.
func listFiles(ctx context.Context, backend omnistorage.Backend, basePath string, opts Options) ([]FileInfo, error) {
	paths, err := backend.List(ctx, basePath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo

	// Try to get extended backend for better file info
	extBackend, hasExt := omnistorage.AsExtended(backend)

	for _, p := range paths {
		// Make path relative to basePath
		relPath := p
		if basePath != "" && len(p) > len(basePath) {
			relPath = p[len(basePath):]
			if len(relPath) > 0 && relPath[0] == '/' {
				relPath = relPath[1:]
			}
		}

		fi := FileInfo{Path: relPath}

		if hasExt {
			info, err := extBackend.Stat(ctx, p)
			if err == nil {
				fi.Size = info.Size()
				fi.ModTime = info.ModTime()
				fi.IsDir = info.IsDir()
				if opts.Checksum {
					fi.Hash = info.Hash(omnistorage.HashMD5)
				}
			}
		}

		// Apply filter if present
		if opts.Filter != nil && !fi.IsDir {
			filterInfo := filter.FileInfo{
				Path:    fi.Path,
				Size:    fi.Size,
				ModTime: fi.ModTime,
				IsDir:   fi.IsDir,
			}
			if !opts.Filter.Match(filterInfo) {
				continue
			}
		}

		files = append(files, fi)
	}

	return files, nil
}

// copyFileWithContext copies a single file with rate limiting, retry, and metadata support.
func copyFileWithContext(ctx context.Context, sctx *syncContext, src, dst omnistorage.Backend, srcPath, dstPath string) error {
	// Wrap with retry if configured
	if sctx.opts.Retry != nil && sctx.opts.Retry.MaxRetries > 0 {
		return retryOperation(ctx, *sctx.opts.Retry, func() error {
			return copyFileSingle(ctx, sctx, src, dst, srcPath, dstPath)
		})
	}
	return copyFileSingle(ctx, sctx, src, dst, srcPath, dstPath)
}

// copyFileSingle performs a single copy attempt with rate limiting and metadata.
func copyFileSingle(ctx context.Context, sctx *syncContext, src, dst omnistorage.Backend, srcPath, dstPath string) error {
	// First try server-side copy if both backends are the same and support it
	// Note: Server-side copy skips rate limiting as no data flows through client
	if src == dst {
		if ext, ok := omnistorage.AsExtended(src); ok && ext.Features().Copy {
			return ext.Copy(ctx, srcPath, dstPath)
		}
	}

	// Fall back to read/write copy
	reader, err := src.NewReader(ctx, srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	// Apply rate limiting if configured
	var finalReader io.Reader = reader
	if sctx.rateLimiter != nil {
		finalReader = newRateLimitedReader(reader, sctx.rateLimiter)
	}

	// Build writer options based on metadata settings
	writerOpts := buildWriterOptions(ctx, src, srcPath, sctx.opts.PreserveMetadata)

	writer, err := dst.NewWriter(ctx, dstPath, writerOpts...)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, finalReader)
	if err != nil {
		_ = writer.Close()
		return err
	}

	return writer.Close()
}

// buildWriterOptions builds WriterOptions based on source file metadata.
func buildWriterOptions(ctx context.Context, src omnistorage.Backend, srcPath string, metaOpts *MetadataOptions) []omnistorage.WriterOption {
	var opts []omnistorage.WriterOption

	ext, hasExt := omnistorage.AsExtended(src)
	if !hasExt {
		return opts
	}

	info, err := ext.Stat(ctx, srcPath)
	if err != nil {
		return opts
	}

	// Default: always preserve content-type
	preserveContentType := true
	preserveCustomMetadata := false

	if metaOpts != nil {
		preserveContentType = metaOpts.ContentType
		preserveCustomMetadata = metaOpts.CustomMetadata
	}

	// Preserve content type
	if preserveContentType {
		if ct := info.ContentType(); ct != "" {
			opts = append(opts, omnistorage.WithContentType(ct))
		}
	}

	// Preserve custom metadata
	if preserveCustomMetadata {
		if meta := info.Metadata(); len(meta) > 0 {
			opts = append(opts, omnistorage.WithMetadata(meta))
		}
	}

	return opts
}

// copyFile copies a single file from source to destination (legacy function).
// Use copyFileWithContext for full feature support.
func copyFile(ctx context.Context, src, dst omnistorage.Backend, srcPath, dstPath string) error {
	// Create a minimal sync context for backward compatibility
	sctx := &syncContext{opts: Options{}}
	return copyFileSingle(ctx, sctx, src, dst, srcPath, dstPath)
}
