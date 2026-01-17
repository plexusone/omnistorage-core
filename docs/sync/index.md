# Sync Engine Overview

The sync package provides rclone-like file synchronization between storage backends.

## Features

- **Sync** - Make destination match source (with optional deletes)
- **Copy** - Copy files without deleting extras
- **Move** - Move files from source to destination
- **Check** - Verify files match between backends
- **Filtering** - Include/exclude patterns, size/age filters
- **Transfer Controls** - Bandwidth limiting, parallel transfers, retry

## Quick Example

```go
import "github.com/grokify/omnistorage/sync"

srcBackend := file.New(file.Config{Root: "/local"})
dstBackend, _ := s3.New(s3.Config{Bucket: "my-bucket"})

// Sync local to S3
result, err := sync.Sync(ctx, srcBackend, dstBackend, "data/", "backup/", sync.Options{
    DeleteExtra: true,  // Delete files in dst not in src
    Progress: func(p sync.Progress) {
        fmt.Printf("%s: %d/%d files\n", p.Phase, p.FilesTransferred, p.TotalFiles)
    },
})

fmt.Printf("Copied: %d, Updated: %d, Deleted: %d\n",
    result.Copied, result.Updated, result.Deleted)
```

## Core Operations

| Operation | Function | Description |
|-----------|----------|-------------|
| Sync | `sync.Sync()` | Make dst match src (like `rclone sync`) |
| Copy | `sync.Copy()` | Copy without deleting extras |
| Move | `sync.Move()` | Move files (copy + delete source) |
| Check | `sync.Check()` | Compare and report differences |
| Verify | `sync.Verify()` | Verify files match |

## Options

```go
sync.Options{
    DeleteExtra:      true,   // Delete extra files in destination
    DryRun:           true,   // Report without making changes
    Checksum:         true,   // Compare by checksum
    SizeOnly:         true,   // Compare by size only
    IgnoreExisting:   true,   // Skip existing files
    Concurrency:      4,      // Parallel transfers
    BandwidthLimit:   1<<20,  // 1 MB/s rate limit
    MaxErrors:        10,     // Stop after N errors
    Progress:         func(Progress){}, // Progress callback
    Filter:           filter, // Include/exclude filter
    Retry:            &RetryConfig{},   // Retry configuration
    PreserveMetadata: &MetadataOptions{}, // Metadata preservation
}
```

## rclone Parity

The sync package implements ~95% of rclone's core sync features:

- All core operations (sync, copy, move, check)
- All comparison methods (size, modtime, checksum)
- Filtering (include, exclude, size, age)
- Transfer controls (parallel, bandwidth, retry)

See [rclone Parity](rclone-parity.md) for detailed comparison.

## Documentation

- [Operations](operations.md) - Detailed operation reference
- [Filtering](filtering.md) - Include/exclude patterns
- [Transfer Controls](transfer-controls.md) - Bandwidth, concurrency, retry
- [rclone Parity](rclone-parity.md) - Feature comparison with rclone
