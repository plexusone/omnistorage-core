// Package sync provides file synchronization between omnistorage backends.
//
// Inspired by rclone, this package provides three main operations:
//
//   - Sync: Make destination match source (including deletes)
//   - Bisync: Two-way sync with conflict resolution
//   - CopyDir: Copy files recursively (no deletes)
//   - Check: Verify files match between backends
//
// Basic usage:
//
//	result, err := sync.Sync(ctx, srcBackend, dstBackend, "data/", "backup/", sync.Options{
//	    DeleteExtra: true,
//	    DryRun:      false,
//	})
//	fmt.Printf("Copied: %d, Deleted: %d\n", result.Copied, result.Deleted)
//
// With logging:
//
//	result, err := sync.Sync(ctx, srcBackend, dstBackend, "data/", "backup/", sync.Options{
//	    Logger: slog.Default(),
//	})
package sync

import (
	"log/slog"
	"time"

	"github.com/grokify/mogo/log/slogutil"
	"github.com/grokify/omnistorage/sync/filter"
	"github.com/grokify/oscompat/tsync"
)

// Options configures sync behavior.
type Options struct {
	// DeleteExtra deletes files in destination that don't exist in source.
	// When false, behaves like CopyDir (only adds/updates, never deletes).
	DeleteExtra bool

	// DryRun reports what would be done without making changes.
	DryRun bool

	// Checksum uses file checksums (when available) for comparison
	// instead of just size and modification time.
	// This is slower but more accurate.
	Checksum bool

	// IgnoreExisting skips files that already exist in destination.
	// Useful for resuming interrupted syncs.
	IgnoreExisting bool

	// IgnoreSize ignores size when comparing files.
	// Only compares modification time (or checksum if Checksum is true).
	IgnoreSize bool

	// IgnoreTime ignores modification time when comparing files.
	// Only compares size (or checksum if Checksum is true).
	IgnoreTime bool

	// SizeOnly compares files by size only, ignoring modification time.
	SizeOnly bool

	// Progress is called with progress updates during sync.
	// Can be nil if progress updates aren't needed.
	Progress func(Progress)

	// MaxErrors is the maximum number of errors before aborting.
	// 0 means abort on first error.
	MaxErrors int

	// Concurrency is the number of concurrent file transfers.
	// Default is 4.
	Concurrency int

	// Filter specifies which files to include/exclude from sync.
	// If nil, all files are included.
	Filter *filter.Filter

	// DeleteExcluded deletes files from destination that match exclude filters.
	// Only applies when DeleteExtra is true.
	DeleteExcluded bool

	// BandwidthLimit is the maximum bytes per second for transfers.
	// 0 means unlimited. The limit is shared across all concurrent transfers.
	// Example: 1048576 for 1MB/s, or use filter.MB constant.
	BandwidthLimit int64

	// Retry configures retry behavior for failed file operations.
	// If nil or MaxRetries is 0, operations are not retried.
	Retry *RetryConfig

	// PreserveMetadata controls which metadata to preserve during sync.
	// If nil, only content-type is preserved (default behavior).
	PreserveMetadata *MetadataOptions

	// Logger is used for structured logging during sync operations.
	// If nil, a null logger is used (no logging).
	Logger *slog.Logger
}

// logger returns the configured logger or a null logger if none is set.
func (o Options) logger() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slogutil.Null()
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		DeleteExtra: false,
		DryRun:      false,
		Checksum:    false,
		Concurrency: 4,
		MaxErrors:   0,
	}
}

// Progress represents the current state of a sync operation.
type Progress struct {
	// Phase indicates the current sync phase.
	Phase Phase

	// CurrentFile is the file currently being processed.
	CurrentFile string

	// BytesTransferred is the number of bytes transferred so far.
	BytesTransferred int64

	// TotalBytes is the total bytes to transfer (if known).
	TotalBytes int64

	// FilesTransferred is the number of files transferred so far.
	FilesTransferred int

	// TotalFiles is the total number of files to transfer.
	TotalFiles int

	// FilesDeleted is the number of files deleted so far.
	FilesDeleted int

	// Errors is the number of errors encountered so far.
	Errors int
}

// Phase represents a phase of the sync operation.
type Phase string

const (
	// PhaseScanning indicates the sync is scanning for files.
	PhaseScanning Phase = "scanning"

	// PhaseComparing indicates the sync is comparing files.
	PhaseComparing Phase = "comparing"

	// PhaseTransferring indicates the sync is transferring files.
	PhaseTransferring Phase = "transferring"

	// PhaseDeleting indicates the sync is deleting extra files.
	PhaseDeleting Phase = "deleting"

	// PhaseComplete indicates the sync is complete.
	PhaseComplete Phase = "complete"
)

// Result contains the results of a sync operation.
type Result struct {
	// Copied is the number of files copied.
	Copied int

	// Updated is the number of files updated (overwritten).
	Updated int

	// Deleted is the number of files deleted from destination.
	Deleted int

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

// Success returns true if sync completed without errors.
func (r *Result) Success() bool {
	return len(r.Errors) == 0
}

// FileError represents an error that occurred for a specific file.
type FileError struct {
	Path string
	Op   string // "copy", "delete", "stat", etc.
	Err  error
}

func (e FileError) Error() string {
	return e.Op + " " + e.Path + ": " + e.Err.Error()
}

// CheckResult contains the results of a check operation.
type CheckResult struct {
	// Match lists files that match between source and destination.
	Match []string

	// Differ lists files that exist in both but have different content.
	Differ []string

	// SrcOnly lists files that exist only in source.
	SrcOnly []string

	// DstOnly lists files that exist only in destination.
	DstOnly []string

	// Errors contains any errors that occurred during checking.
	Errors []FileError
}

// InSync returns true if source and destination are in sync.
func (r *CheckResult) InSync() bool {
	return len(r.Differ) == 0 && len(r.SrcOnly) == 0 && len(r.DstOnly) == 0
}

// FileInfo represents a file for sync comparison.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	Hash    string // MD5 or other hash if available
	IsDir   bool
}

// MetadataOptions configures which metadata to preserve during sync.
type MetadataOptions struct {
	// ContentType preserves the MIME content type.
	// Default is true (always preserved).
	ContentType bool

	// ModTime preserves the modification time.
	// Requires backend support (Features().SetModTime).
	ModTime bool

	// CustomMetadata preserves backend-specific custom metadata.
	// For S3, this includes user-defined metadata headers.
	CustomMetadata bool
}

// DefaultMetadataOptions returns metadata options with sensible defaults.
func DefaultMetadataOptions() *MetadataOptions {
	return &MetadataOptions{
		ContentType:    true,
		ModTime:        false, // Requires backend support, off by default
		CustomMetadata: false, // Can be expensive, off by default
	}
}

// NeedsUpdate returns true if dst should be updated to match src.
func NeedsUpdate(src, dst FileInfo, opts Options) bool {
	// If size-only mode, just compare sizes
	if opts.SizeOnly {
		return src.Size != dst.Size
	}

	// If using checksums and both have hashes, compare them
	if opts.Checksum && src.Hash != "" && dst.Hash != "" {
		return src.Hash != dst.Hash
	}

	// Compare size (unless ignored)
	if !opts.IgnoreSize && src.Size != dst.Size {
		return true
	}

	// Compare modification time (unless ignored)
	// Uses oscompat/tsync for cross-platform timestamp comparison
	// with appropriate tolerance for filesystem precision differences.
	if !opts.IgnoreTime && !tsync.Equal(src.ModTime, dst.ModTime) {
		return true
	}

	return false
}
