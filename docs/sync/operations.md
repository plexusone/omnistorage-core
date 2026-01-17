# Sync Operations

This page covers all sync operations in detail.

## Sync

Make destination match source, including deletes.

```go
result, err := sync.Sync(ctx, srcBackend, dstBackend, "src/", "dst/", sync.Options{
    DeleteExtra: true,  // Delete files in dst not in src
})
```

### Behavior

1. Lists all files in source and destination
2. Copies new/modified files from source to destination
3. Optionally deletes files in destination not in source
4. Returns detailed results

### Result

```go
type Result struct {
    Copied    int   // Files copied
    Updated   int   // Files updated
    Deleted   int   // Files deleted
    Skipped   int   // Files skipped
    Errors    int   // Error count
    BytesCopied int64 // Total bytes transferred
}
```

## Copy

Copy files without deleting extras.

```go
// Copy a directory
result, err := sync.Copy(ctx, src, dst, "data/", "backup/", sync.Options{})

// Copy a single file
err := sync.CopyFile(ctx, src, dst, "file.txt", "file_copy.txt")
```

### Convenience Functions

```go
// Copy between paths (same or different backends)
sync.CopyBetweenPaths(ctx, srcBackend, "src/path", dstBackend, "dst/path", opts)

// Copy to a path on same backend
sync.CopyToPath(ctx, backend, "src/file.txt", "dst/file.txt")

// Copy from path with path transformation
sync.CopyFromPath(ctx, src, dst, "source/", "dest/", opts)

// Copy preserving full tree structure
sync.TreeCopy(ctx, src, dst, "source/", "dest/", opts)
```

### Copy with Progress

```go
result, err := sync.CopyWithProgress(ctx, src, dst, "data/", "backup/",
    func(file string, bytes int64) {
        fmt.Printf("Copying %s: %d bytes\n", file, bytes)
    })
```

## Move

Move files from source to destination (copy + delete source).

```go
result, err := sync.Move(ctx, src, dst, "old/", "new/", sync.Options{})
```

### Behavior

1. Copies files to destination
2. Deletes source files after successful copy
3. Uses server-side move when available

## Check

Compare files between backends and report differences.

```go
result, err := sync.Check(ctx, src, dst, "data/", "backup/", sync.Options{
    Checksum: true,  // Compare by checksum
})

fmt.Printf("Match: %d\n", len(result.Match))
fmt.Printf("Differ: %d\n", len(result.Differ))
fmt.Printf("SrcOnly: %d\n", len(result.SrcOnly))
fmt.Printf("DstOnly: %d\n", len(result.DstOnly))
```

### CheckResult

```go
type CheckResult struct {
    Match   []string // Files that match
    Differ  []string // Files that differ
    SrcOnly []string // Files only in source
    DstOnly []string // Files only in destination
}
```

### Diff

Get human-readable differences:

```go
diff := sync.Diff(ctx, src, dst, "data/", "backup/", sync.Options{})
for _, d := range diff {
    fmt.Println(d)
}
```

## Verify

Verify files match between backends.

```go
// Simple verify (returns bool)
inSync, err := sync.Verify(ctx, src, dst, "data/", "backup/", sync.Options{})
if inSync {
    fmt.Println("All files match")
}

// Verify single file
match, err := sync.VerifyFile(ctx, src, dst, "file.txt", sync.Options{})

// Verify with checksum
match, err := sync.VerifyChecksum(ctx, src, dst, "file.txt")
```

### Detailed Verification

```go
// Get detailed results
details, err := sync.VerifyWithDetails(ctx, src, dst, "data/", "backup/", sync.Options{})
fmt.Printf("Total: %d, Match: %d, Differ: %d\n",
    details.Total, details.Matched, details.Different)

// Human-readable report
report, err := sync.VerifyAndReport(ctx, src, dst, "data/", "backup/", sync.Options{})
fmt.Println(report)
```

### Integrity Verification

Verify file integrity (checksum validation):

```go
// Verify single file integrity
valid, err := sync.VerifyIntegrity(ctx, backend, "file.txt", expectedHash)

// Verify all files
results, err := sync.VerifyAllIntegrity(ctx, backend, "data/", hashMap)
```

## Comparison Methods

Control how files are compared:

```go
sync.Options{
    // Default: Size + ModTime
    // Files match if size and modification time are equal

    Checksum: true,
    // Compare by checksum (MD5/SHA256)
    // Slower but more accurate

    SizeOnly: true,
    // Compare by size only
    // Fast but less accurate

    IgnoreTime: true,
    // Ignore modification time differences

    IgnoreSize: true,
    // Ignore size differences (use with Checksum)
}
```

## Dry Run

Preview changes without making them:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    DryRun: true,
})
// result shows what WOULD happen, but no changes are made
```

## Progress Tracking

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Progress: func(p sync.Progress) {
        fmt.Printf("[%s] %d/%d files, %d/%d bytes\n",
            p.Phase,
            p.FilesTransferred, p.TotalFiles,
            p.BytesTransferred, p.TotalBytes)
    },
})
```

### Progress Fields

```go
type Progress struct {
    Phase            string // "scanning", "copying", "deleting"
    CurrentFile      string // Current file being processed
    TotalFiles       int    // Total files to process
    FilesTransferred int    // Files completed
    TotalBytes       int64  // Total bytes to transfer
    BytesTransferred int64  // Bytes transferred
}
```
