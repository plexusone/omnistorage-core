package sync

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	omnistorage "github.com/plexusone/omnistorage-core/object"
	"github.com/plexusone/omnistorage-core/object/sync/filter"
)

// ConflictStrategy defines how to handle files changed on both sides.
type ConflictStrategy int

const (
	// ConflictNewerWins keeps the file with the newer modification time.
	ConflictNewerWins ConflictStrategy = iota

	// ConflictLargerWins keeps the file with the larger size.
	ConflictLargerWins

	// ConflictSourceWins always keeps the source (path1) version.
	ConflictSourceWins

	// ConflictDestWins always keeps the destination (path2) version.
	ConflictDestWins

	// ConflictKeepBoth keeps both files, renaming one with a conflict suffix.
	ConflictKeepBoth

	// ConflictSkip skips conflicting files without copying either.
	ConflictSkip

	// ConflictError reports an error for each conflict.
	ConflictError
)

// BisyncOptions configures bidirectional sync behavior.
type BisyncOptions struct {
	// ConflictStrategy determines how to handle files changed on both sides.
	// Default is ConflictNewerWins.
	ConflictStrategy ConflictStrategy

	// ConflictSuffix is appended to filenames when using ConflictKeepBoth.
	// Default is ".conflict".
	ConflictSuffix string

	// DryRun reports what would be done without making changes.
	DryRun bool

	// Checksum uses file checksums for comparison instead of size/time.
	Checksum bool

	// DeleteMissing deletes files that don't exist on the other side.
	// Use with caution - this can result in data loss if a file was
	// intentionally deleted on one side but still exists on the other.
	DeleteMissing bool

	// Progress is called with progress updates during sync.
	Progress func(Progress)

	// MaxErrors is the maximum number of errors before aborting.
	MaxErrors int

	// Concurrency is the number of concurrent file transfers.
	// Default is 4.
	Concurrency int

	// Filter specifies which files to include/exclude.
	Filter *filter.Filter

	// BandwidthLimit is the maximum bytes per second for transfers.
	BandwidthLimit int64

	// Retry configures retry behavior for failed operations.
	Retry *RetryConfig

	// PreserveMetadata controls which metadata to preserve.
	PreserveMetadata *MetadataOptions

	// Logger for structured logging. If nil, no logging is performed.
	Logger *slog.Logger
}

// DefaultBisyncOptions returns BisyncOptions with sensible defaults.
func DefaultBisyncOptions() BisyncOptions {
	return BisyncOptions{
		ConflictStrategy: ConflictNewerWins,
		ConflictSuffix:   ".conflict",
		DryRun:           false,
		Checksum:         false,
		DeleteMissing:    false,
		Concurrency:      4,
		MaxErrors:        0,
	}
}

// BisyncResult contains the results of a bidirectional sync.
type BisyncResult struct {
	// CopiedToPath2 is files copied from path1 to path2.
	CopiedToPath2 int

	// CopiedToPath1 is files copied from path2 to path1.
	CopiedToPath1 int

	// UpdatedInPath2 is files updated in path2 from path1.
	UpdatedInPath2 int

	// UpdatedInPath1 is files updated in path1 from path2.
	UpdatedInPath1 int

	// DeletedFromPath1 is files deleted from path1.
	DeletedFromPath1 int

	// DeletedFromPath2 is files deleted from path2.
	DeletedFromPath2 int

	// Conflicts is the list of conflicting files.
	Conflicts []Conflict

	// Skipped is the number of files skipped (already in sync).
	Skipped int

	// Errors contains any errors that occurred.
	Errors []FileError

	// BytesTransferred is the total bytes transferred.
	BytesTransferred int64

	// Duration is how long the sync took.
	Duration time.Duration

	// DryRun indicates if this was a dry run.
	DryRun bool
}

// Success returns true if bisync completed without errors.
func (r *BisyncResult) Success() bool {
	return len(r.Errors) == 0
}

// TotalCopied returns the total number of files copied in both directions.
func (r *BisyncResult) TotalCopied() int {
	return r.CopiedToPath1 + r.CopiedToPath2
}

// TotalUpdated returns the total number of files updated in both directions.
func (r *BisyncResult) TotalUpdated() int {
	return r.UpdatedInPath1 + r.UpdatedInPath2
}

// TotalDeleted returns the total number of files deleted from both sides.
func (r *BisyncResult) TotalDeleted() int {
	return r.DeletedFromPath1 + r.DeletedFromPath2
}

// Conflict represents a file that was changed on both sides.
type Conflict struct {
	// Path is the relative path of the conflicting file.
	Path string

	// Path1Info is the file info from path1.
	Path1Info FileInfo

	// Path2Info is the file info from path2.
	Path2Info FileInfo

	// Resolution describes how the conflict was resolved.
	Resolution string

	// Error is set if the conflict could not be resolved.
	Error error
}

// Bisync performs bidirectional synchronization between two backends.
//
// Unlike unidirectional Sync, Bisync propagates changes in both directions:
//   - Files only in path1 are copied to path2
//   - Files only in path2 are copied to path1
//   - Files changed on both sides are handled according to ConflictStrategy
//
// This is useful for syncing two directories that may both have changes,
// such as syncing between a local folder and cloud storage where edits
// can happen on either side.
func Bisync(ctx context.Context, backend1, backend2 omnistorage.Backend, path1, path2 string, opts BisyncOptions) (*BisyncResult, error) {
	startTime := time.Now()
	result := &BisyncResult{DryRun: opts.DryRun}

	// Set defaults
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.ConflictSuffix == "" {
		opts.ConflictSuffix = ".conflict"
	}

	// Get logger
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	logger.Info("starting bisync",
		slog.String("path1", path1),
		slog.String("path2", path2),
		slog.Bool("dry_run", opts.DryRun),
		slog.Bool("delete_missing", opts.DeleteMissing),
		slog.Int("concurrency", opts.Concurrency),
	)

	// Scan both sides
	syncOpts := Options{
		Checksum:    opts.Checksum,
		Filter:      opts.Filter,
		Concurrency: opts.Concurrency,
		Logger:      logger,
	}

	if opts.Progress != nil {
		opts.Progress(Progress{Phase: PhaseScanning, CurrentFile: path1})
	}

	logger.Debug("scanning path1", slog.String("path", path1))
	files1, err := listFiles(ctx, backend1, path1, syncOpts)
	if err != nil {
		logger.Error("failed to scan path1", slog.Any("error", err))
		return nil, fmt.Errorf("scanning path1: %w", err)
	}
	logger.Debug("path1 scan complete", slog.Int("files", len(files1)))

	logger.Debug("scanning path2", slog.String("path", path2))
	files2, err := listFiles(ctx, backend2, path2, syncOpts)
	if err != nil {
		logger.Error("failed to scan path2", slog.Any("error", err))
		return nil, fmt.Errorf("scanning path2: %w", err)
	}
	logger.Debug("path2 scan complete", slog.Int("files", len(files2)))

	// Build maps for comparison
	map1 := make(map[string]FileInfo)
	for _, f := range files1 {
		if !f.IsDir {
			map1[f.Path] = f
		}
	}

	map2 := make(map[string]FileInfo)
	for _, f := range files2 {
		if !f.IsDir {
			map2[f.Path] = f
		}
	}

	if opts.Progress != nil {
		opts.Progress(Progress{Phase: PhaseComparing, TotalFiles: len(map1) + len(map2)})
	}

	// Categorize files
	type action struct {
		file      FileInfo
		direction string // "to1", "to2", "conflict", "delete1", "delete2"
		otherFile *FileInfo
	}

	var actions []action
	processed := make(map[string]bool)

	// Process files from path1
	for p, f1 := range map1 {
		processed[p] = true

		f2, existsIn2 := map2[p]
		if !existsIn2 {
			// File only in path1 - copy to path2
			actions = append(actions, action{file: f1, direction: "to2"})
		} else {
			// File exists in both - check if changed
			if NeedsUpdate(f1, f2, syncOpts) || NeedsUpdate(f2, f1, syncOpts) {
				// Both sides have the file but they differ
				actions = append(actions, action{file: f1, direction: "conflict", otherFile: &f2})
			} else {
				// Files are in sync
				result.Skipped++
			}
		}
	}

	// Process files only in path2
	for p, f2 := range map2 {
		if processed[p] {
			continue
		}

		// File only in path2 - copy to path1
		actions = append(actions, action{file: f2, direction: "to1"})
	}

	logger.Info("comparison complete",
		slog.Int("total_actions", len(actions)),
		slog.Int("skipped", result.Skipped),
	)

	// Create sync context for file operations
	sctx := &syncContext{
		opts: Options{
			DryRun:           opts.DryRun,
			BandwidthLimit:   opts.BandwidthLimit,
			Retry:            opts.Retry,
			PreserveMetadata: opts.PreserveMetadata,
			Logger:           logger,
		},
		rateLimiter: newTokenBucket(opts.BandwidthLimit),
		logger:      logger,
	}

	// Process actions
	if opts.Progress != nil {
		opts.Progress(Progress{Phase: PhaseTransferring, TotalFiles: len(actions)})
	}

	// copyDirection handles copying a file in the specified direction.
	// Returns error if copy failed.
	copyDirection := func(act action, toPath2 bool) error {
		var srcBase, dstBase string
		var srcBackend, dstBackend omnistorage.Backend
		var destName string
		var copiedCounter *int

		if toPath2 {
			srcBase, dstBase = path1, path2
			srcBackend, dstBackend = backend1, backend2
			destName = "path2"
			copiedCounter = &result.CopiedToPath2
		} else {
			srcBase, dstBase = path2, path1
			srcBackend, dstBackend = backend2, backend1
			destName = "path1"
			copiedCounter = &result.CopiedToPath1
		}

		srcPath := path.Join(srcBase, act.file.Path)
		dstPath := path.Join(dstBase, act.file.Path)

		logger.Debug("copying to "+destName,
			slog.String("file", act.file.Path),
			slog.Int64("size", act.file.Size),
		)

		if !opts.DryRun {
			if err := copyFileWithContext(ctx, sctx, srcBackend, dstBackend, srcPath, dstPath); err != nil {
				logger.Error("copy to "+destName+" failed", slog.String("file", act.file.Path), slog.Any("error", err))
				result.Errors = append(result.Errors, FileError{Path: act.file.Path, Op: "copy-to-" + destName, Err: err})
				return err
			}
		}
		*copiedCounter++
		result.BytesTransferred += act.file.Size
		return nil
	}

	// maxErrorsReached checks if we've hit the error limit.
	maxErrorsReached := func() bool {
		return opts.MaxErrors > 0 && len(result.Errors) >= opts.MaxErrors
	}

actionLoop:
	for i, act := range actions {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		default:
		}

		if opts.Progress != nil {
			opts.Progress(Progress{
				Phase:            PhaseTransferring,
				CurrentFile:      act.file.Path,
				FilesTransferred: i,
				TotalFiles:       len(actions),
			})
		}

		switch act.direction {
		case "to2":
			if err := copyDirection(act, true); err != nil {
				if maxErrorsReached() {
					break actionLoop
				}
				continue
			}

		case "to1":
			if err := copyDirection(act, false); err != nil {
				if maxErrorsReached() {
					break actionLoop
				}
				continue
			}

		case "conflict":
			// Handle conflict
			conflict := Conflict{
				Path:      act.file.Path,
				Path1Info: act.file,
				Path2Info: *act.otherFile,
			}

			resolution, copyDir, err := resolveConflict(ctx, sctx, backend1, backend2, path1, path2, act.file, *act.otherFile, opts)
			conflict.Resolution = resolution
			conflict.Error = err

			if err != nil {
				logger.Warn("conflict resolution failed",
					slog.String("file", act.file.Path),
					slog.String("resolution", resolution),
					slog.Any("error", err),
				)
				result.Errors = append(result.Errors, FileError{Path: act.file.Path, Op: "conflict", Err: err})
			} else {
				logger.Debug("conflict resolved",
					slog.String("file", act.file.Path),
					slog.String("resolution", resolution),
				)

				if !opts.DryRun {
					switch copyDir {
					case "to1":
						result.UpdatedInPath1++
						result.BytesTransferred += act.otherFile.Size
					case "to2":
						result.UpdatedInPath2++
						result.BytesTransferred += act.file.Size
					case "both":
						result.UpdatedInPath1++
						result.UpdatedInPath2++
						result.BytesTransferred += act.file.Size + act.otherFile.Size
					}
				}
			}

			result.Conflicts = append(result.Conflicts, conflict)
		}
	}

	// Handle deletions if enabled
	if opts.DeleteMissing {
		// Delete files that only exist on one side (they were deleted on the other)
		// This is dangerous and should be used carefully
		logger.Warn("delete_missing is enabled - this may cause data loss")

		// For now, skip deletion logic as it requires tracking previous state
		// to know if a file was deleted vs never existed
	}

	if opts.Progress != nil {
		opts.Progress(Progress{
			Phase:            PhaseComplete,
			FilesTransferred: result.TotalCopied() + result.TotalUpdated(),
			BytesTransferred: result.BytesTransferred,
			Errors:           len(result.Errors),
		})
	}

	result.Duration = time.Since(startTime)

	logger.Info("bisync complete",
		slog.Int("copied_to_path1", result.CopiedToPath1),
		slog.Int("copied_to_path2", result.CopiedToPath2),
		slog.Int("updated_in_path1", result.UpdatedInPath1),
		slog.Int("updated_in_path2", result.UpdatedInPath2),
		slog.Int("conflicts", len(result.Conflicts)),
		slog.Int("errors", len(result.Errors)),
		slog.Int64("bytes_transferred", result.BytesTransferred),
		slog.Duration("duration", result.Duration),
	)

	return result, nil
}

// resolveConflict resolves a conflict between two files based on the strategy.
// Returns the resolution description, the direction to copy ("to1", "to2", "both", or ""),
// and any error.
func resolveConflict(
	ctx context.Context,
	sctx *syncContext,
	backend1, backend2 omnistorage.Backend,
	path1, path2 string,
	file1, file2 FileInfo,
	opts BisyncOptions,
) (string, string, error) {
	srcPath1 := path.Join(path1, file1.Path)
	srcPath2 := path.Join(path2, file2.Path)

	switch opts.ConflictStrategy {
	case ConflictNewerWins:
		if file1.ModTime.After(file2.ModTime) {
			// path1 is newer, copy to path2
			if !opts.DryRun {
				if err := copyFileWithContext(ctx, sctx, backend1, backend2, srcPath1, srcPath2); err != nil {
					return "newer-wins:path1", "", err
				}
			}
			return "newer-wins:path1", "to2", nil
		}
		// path2 is newer, copy to path1
		if !opts.DryRun {
			if err := copyFileWithContext(ctx, sctx, backend2, backend1, srcPath2, srcPath1); err != nil {
				return "newer-wins:path2", "", err
			}
		}
		return "newer-wins:path2", "to1", nil

	case ConflictLargerWins:
		if file1.Size > file2.Size {
			if !opts.DryRun {
				if err := copyFileWithContext(ctx, sctx, backend1, backend2, srcPath1, srcPath2); err != nil {
					return "larger-wins:path1", "", err
				}
			}
			return "larger-wins:path1", "to2", nil
		}
		if !opts.DryRun {
			if err := copyFileWithContext(ctx, sctx, backend2, backend1, srcPath2, srcPath1); err != nil {
				return "larger-wins:path2", "", err
			}
		}
		return "larger-wins:path2", "to1", nil

	case ConflictSourceWins:
		if !opts.DryRun {
			if err := copyFileWithContext(ctx, sctx, backend1, backend2, srcPath1, srcPath2); err != nil {
				return "source-wins", "", err
			}
		}
		return "source-wins", "to2", nil

	case ConflictDestWins:
		if !opts.DryRun {
			if err := copyFileWithContext(ctx, sctx, backend2, backend1, srcPath2, srcPath1); err != nil {
				return "dest-wins", "", err
			}
		}
		return "dest-wins", "to1", nil

	case ConflictKeepBoth:
		// Rename the older file with conflict suffix
		conflictPath := file1.Path + opts.ConflictSuffix
		if file1.ModTime.Before(file2.ModTime) {
			// path1 is older, rename it and copy path2 to path1
			conflictDst := path.Join(path1, conflictPath)
			if !opts.DryRun {
				// Move path1's version to conflict name
				if err := copyFileWithContext(ctx, sctx, backend1, backend1, srcPath1, conflictDst); err != nil {
					return "keep-both", "", err
				}
				// Copy path2's version to path1
				if err := copyFileWithContext(ctx, sctx, backend2, backend1, srcPath2, srcPath1); err != nil {
					return "keep-both", "", err
				}
				// Copy path1's original to path2 as conflict
				conflictDst2 := path.Join(path2, conflictPath)
				if err := copyFileWithContext(ctx, sctx, backend1, backend2, path.Join(path1, conflictPath), conflictDst2); err != nil {
					return "keep-both", "", err
				}
			}
		} else {
			// path2 is older, rename it and copy path1 to path2
			conflictDst := path.Join(path2, conflictPath)
			if !opts.DryRun {
				// Move path2's version to conflict name
				if err := copyFileWithContext(ctx, sctx, backend2, backend2, srcPath2, conflictDst); err != nil {
					return "keep-both", "", err
				}
				// Copy path1's version to path2
				if err := copyFileWithContext(ctx, sctx, backend1, backend2, srcPath1, srcPath2); err != nil {
					return "keep-both", "", err
				}
				// Copy path2's original to path1 as conflict
				conflictDst1 := path.Join(path1, conflictPath)
				if err := copyFileWithContext(ctx, sctx, backend2, backend1, path.Join(path2, conflictPath), conflictDst1); err != nil {
					return "keep-both", "", err
				}
			}
		}
		return "keep-both", "both", nil

	case ConflictSkip:
		return "skipped", "", nil

	case ConflictError:
		return "error", "", fmt.Errorf("conflict detected for %s: path1 mod=%v size=%d, path2 mod=%v size=%d",
			file1.Path, file1.ModTime, file1.Size, file2.ModTime, file2.Size)

	default:
		return "unknown-strategy", "", fmt.Errorf("unknown conflict strategy: %d", opts.ConflictStrategy)
	}
}
