# Rclone Feature Parity

This document tracks omnistorage's feature parity with [rclone](https://rclone.org/), the popular cloud storage sync tool that inspired omnistorage's sync package.

## Overview

Omnistorage aims to provide rclone-like functionality as a Go library. While rclone is a CLI tool with 70+ backends, omnistorage focuses on providing a clean programmatic API for the most common sync operations.

**Current Parity: ~95% of core features**

## Feature Comparison

### Core Operations

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Sync (mirror) | `rclone sync` | `sync.Sync()` | ✅ Complete |
| Copy (no delete) | `rclone copy` | `sync.Copy()` | ✅ Complete |
| Move | `rclone move` | `sync.Move()` | ✅ Complete |
| Check/Verify | `rclone check` | `sync.Check()`, `sync.Verify()` | ✅ Complete |
| List files | `rclone ls` | `backend.List()` | ✅ Complete |
| Delete | `rclone delete` | `backend.Delete()` | ✅ Complete |
| Mkdir | `rclone mkdir` | `ext.Mkdir()` | ✅ Complete |
| Rmdir | `rclone rmdir` | `ext.Rmdir()` | ✅ Complete |

### Comparison Methods

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Size + ModTime | Default | Default | ✅ Complete |
| Checksum | `--checksum` | `Options{Checksum: true}` | ✅ Complete |
| Size only | `--size-only` | `Options{SizeOnly: true}` | ✅ Complete |
| Ignore time | `--ignore-times` | `Options{IgnoreTime: true}` | ✅ Complete |
| Ignore size | `--ignore-size` | `Options{IgnoreSize: true}` | ✅ Complete |

### Safety & Control

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Dry run | `--dry-run` | `Options{DryRun: true}` | ✅ Complete |
| Progress callbacks | `-P/--progress` | `Options{Progress: func()}` | ✅ Complete |
| Context cancellation | Ctrl+C | `context.Context` | ✅ Complete |
| Max errors | `--max-errors` | `Options{MaxErrors: N}` | ✅ Complete |
| Skip existing | `--ignore-existing` | `Options{IgnoreExisting: true}` | ✅ Complete |

### Server-Side Operations

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Server-side copy | Auto-detected | Auto via `Features().Copy` | ✅ Complete |
| Server-side move | Auto-detected | Auto via `Features().Move` | ✅ Complete |

### Transfer Controls

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Parallel transfers | `--transfers N` | `Options{Concurrency: N}` | ✅ Complete |
| Bandwidth limiting | `--bwlimit` | `Options{BandwidthLimit: N}` | ✅ Complete |
| Retry on error | `--retries` | `Options{Retry: &RetryConfig{}}` | ✅ Complete |
| Check-first mode | `--check-first` | Default behavior | ✅ Complete |

### Filtering

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Include patterns | `--include` | `Options{Filter: ...}` | ✅ Complete |
| Exclude patterns | `--exclude` | `Options{Filter: ...}` | ✅ Complete |
| Min/max size | `--min-size/--max-size` | `filter.MinSize/MaxSize` | ✅ Complete |
| Min/max age | `--min-age/--max-age` | `filter.MinAge/MaxAge` | ✅ Complete |
| Filter from file | `--filter-from` | `filter.FromFile()` | ✅ Complete |
| Delete excluded | `--delete-excluded` | `Options{DeleteExcluded: true}` | ✅ Complete |

### Advanced Features

| Feature | rclone | omnistorage | Status |
|---------|--------|-------------|--------|
| Bidirectional sync | `rclone bisync` | - | ❌ Not planned for v1.0 |
| Interactive mode | `-i` | - | ❌ Not applicable (library) |
| Metadata preservation | `--metadata` | `Options{PreserveMetadata: ...}` | ✅ Complete |
| Deduplication | `rclone dedupe` | - | ❌ Not implemented |

## Usage Examples

### rclone vs omnistorage

**Sync with delete:**
```bash
# rclone
rclone sync /local/path s3:bucket/path --delete-during

# omnistorage
result, _ := sync.Sync(ctx, localBackend, s3Backend, "path", "path", sync.Options{
    DeleteExtra: true,
})
```

**Copy with progress:**
```bash
# rclone
rclone copy /local/path s3:bucket/path -P

# omnistorage
result, _ := sync.Copy(ctx, src, dst, "path", "path", sync.Options{
    Progress: func(p sync.Progress) {
        fmt.Printf("%s: %d/%d files\n", p.Phase, p.FilesTransferred, p.TotalFiles)
    },
})
```

**Dry run with filters:**
```bash
# rclone
rclone sync /local s3:bucket --dry-run --include "*.json" --exclude "*.tmp"

# omnistorage
result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    DryRun: true,
    Filter: filter.New(
        filter.Include("*.json"),
        filter.Exclude("*.tmp"),
    ),
})
```

**Checksum verification:**
```bash
# rclone
rclone check /local s3:bucket --checksum

# omnistorage
result, _ := sync.Check(ctx, src, dst, "", "", sync.Options{
    Checksum: true,
})
```

**Bandwidth limiting:**
```bash
# rclone
rclone sync /local s3:bucket --bwlimit 1M

# omnistorage
result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    BandwidthLimit: 1024 * 1024, // 1MB/s
})
```

**Retry on failure:**
```bash
# rclone
rclone sync /local s3:bucket --retries 3

# omnistorage
retryConfig := sync.DefaultRetryConfig()
retryConfig.MaxRetries = 3
result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Retry: &retryConfig,
})
```

**Preserve metadata:**
```bash
# rclone
rclone sync /local s3:bucket --metadata

# omnistorage
result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    PreserveMetadata: &sync.MetadataOptions{
        ContentType:    true,
        CustomMetadata: true,
    },
})
```

## Implementation Priority

### High Priority (v0.2.0)
1. ~~Parallel transfers~~ ✅
2. ~~Filtering system~~ ✅
3. ~~Move in sync package~~ ✅

### Medium Priority (v0.3.0)
1. ~~Bandwidth limiting~~ ✅
2. ~~Retry/resume support~~ ✅
3. ~~Extended metadata preservation~~ ✅

### Low Priority (v1.0+)
1. Deduplication
2. Bidirectional sync

## References

- [rclone sync documentation](https://rclone.org/commands/rclone_sync/)
- [rclone copy documentation](https://rclone.org/commands/rclone_copy/)
- [rclone filtering documentation](https://rclone.org/filtering/)
- [rclone global flags](https://rclone.org/flags/)
