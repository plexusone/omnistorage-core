# OmniStorage

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

OmniStorage is a unified storage abstraction layer for Go, inspired by [rclone](https://rclone.org/). It provides a single interface for reading and writing to various storage backends with composable layers for compression and record framing.

**[Full Documentation](https://grokify.github.io/omnistorage/)** | [API Reference](https://pkg.go.dev/github.com/grokify/omnistorage)

## Features

- **Single interface** for multiple storage backends (local files, S3, cloud drives, etc.)
- **Composable layers** for compression (gzip, zstd) and formatting (NDJSON)
- **Sync engine** for file synchronization between backends (like `rclone sync`)
- **Extended interface** for metadata, server-side copy/move, and capability discovery
- **Backend registration** allowing external packages to implement backends

## Installation

```bash
go get github.com/grokify/omnistorage
```

## Quick Start

### Basic Read/Write

```go
package main

import (
    "context"
    "io"
    "log"

    "github.com/grokify/omnistorage/backend/file"
)

func main() {
    ctx := context.Background()

    // Create a file backend
    backend := file.New(file.Config{Root: "/data"})
    defer backend.Close()

    // Write a file
    w, err := backend.NewWriter(ctx, "hello.txt")
    if err != nil {
        log.Fatal(err)
    }
    w.Write([]byte("Hello, World!"))
    w.Close()

    // Read it back
    r, err := backend.NewReader(ctx, "hello.txt")
    if err != nil {
        log.Fatal(err)
    }
    data, _ := io.ReadAll(r)
    r.Close()

    log.Println(string(data)) // "Hello, World!"
}
```

### With Compression

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/compress/gzip"
)

// Write compressed data
fileWriter, _ := backend.NewWriter(ctx, "data.txt.gz")
gzipWriter, _ := gzip.NewWriter(fileWriter)
gzipWriter.Write([]byte("compressed content"))
gzipWriter.Close()

// Read compressed data
fileReader, _ := backend.NewReader(ctx, "data.txt.gz")
gzipReader, _ := gzip.NewReader(fileReader)
data, _ := io.ReadAll(gzipReader)
gzipReader.Close()
```

### With NDJSON Format

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/format/ndjson"
)

// Write NDJSON records
w, _ := backend.NewWriter(ctx, "records.ndjson")
ndjsonWriter := ndjson.NewWriter(w)
ndjsonWriter.Write([]byte(`{"id":1,"name":"alice"}`))
ndjsonWriter.Write([]byte(`{"id":2,"name":"bob"}`))
ndjsonWriter.Close()

// Read NDJSON records
r, _ := backend.NewReader(ctx, "records.ndjson")
ndjsonReader := ndjson.NewReader(r)
for {
    record, err := ndjsonReader.Read()
    if err == io.EOF {
        break
    }
    log.Println(string(record))
}
ndjsonReader.Close()
```

### Using the Registry

```go
import "github.com/grokify/omnistorage"

// Open backend by name
backend, _ := omnistorage.Open("file", map[string]string{
    "root": "/data",
})
defer backend.Close()

// List registered backends
backends := omnistorage.Backends() // ["file", "memory", "s3"]
```

## Backends

### File Backend

Local filesystem storage.

```go
import "github.com/grokify/omnistorage/backend/file"

backend := file.New(file.Config{
    Root: "/data",  // Base directory for all operations
})
```

### Memory Backend

In-memory storage for testing.

```go
import "github.com/grokify/omnistorage/backend/memory"

backend := memory.New()
```

### S3 Backend

S3-compatible storage (AWS S3, Cloudflare R2, MinIO, Wasabi, etc.).

```go
import "github.com/grokify/omnistorage/backend/s3"

// AWS S3
backend, _ := s3.New(s3.Config{
    Bucket: "my-bucket",
    Region: "us-east-1",
})

// Cloudflare R2
backend, _ := s3.New(s3.Config{
    Bucket:   "my-bucket",
    Endpoint: "https://<account_id>.r2.cloudflarestorage.com",
    Region:   "auto",
})

// MinIO (local)
backend, _ := s3.New(s3.Config{
    Bucket:       "my-bucket",
    Endpoint:     "http://localhost:9000",
    UsePathStyle: true,
    DisableSSL:   true,
})

// From environment variables
backend, _ := s3.New(s3.ConfigFromEnv())
```

### Google Drive

Google Drive is in a separate repository to keep the core lightweight.

```bash
go get github.com/grokify/omnistorage-google
```

```go
import "github.com/grokify/omnistorage-google/backend/drive"

backend, _ := drive.New(drive.Config{
    CredentialsFile: "credentials.json",
    RootFolder:      "My App Data",
})
```

See [omnistorage-google](https://github.com/grokify/omnistorage-google) for details.

## Sync Operations

The `sync` package provides rclone-like file synchronization.

### Sync (Mirror)

Make destination match source, including deletes.

```go
import "github.com/grokify/omnistorage/sync"

result, err := sync.Sync(ctx, srcBackend, dstBackend, "data/", "backup/", sync.Options{
    DeleteExtra: true,  // Delete files in dst not in src
    DryRun:      false,
})
fmt.Printf("Copied: %d, Updated: %d, Deleted: %d\n",
    result.Copied, result.Updated, result.Deleted)
```

### Copy

Copy files without deleting extras.

```go
// Copy a directory
result, _ := sync.Copy(ctx, src, dst, "data/", "backup/", sync.Options{})

// Copy a single file
err := sync.CopyFile(ctx, src, dst, "file.txt", "file_copy.txt")

// Copy with progress
result, _ := sync.CopyWithProgress(ctx, src, dst, "data/", "backup/",
    func(file string, bytes int64) {
        fmt.Printf("Copying %s: %d bytes\n", file, bytes)
    })
```

### Bisync (Bidirectional Sync)

Two-way synchronization with conflict resolution.

```go
import "github.com/grokify/omnistorage/sync"

result, err := sync.Bisync(ctx, backend1, backend2, "folder1/", "folder2/", sync.BisyncOptions{
    ConflictStrategy: sync.ConflictNewerWins,  // Newer file wins conflicts
    DryRun:           false,
})
fmt.Printf("Copied to path1: %d, Copied to path2: %d, Conflicts: %d\n",
    result.CopiedToPath1, result.CopiedToPath2, len(result.Conflicts))
```

Conflict resolution strategies:

- `ConflictNewerWins` - Newer file overwrites older (default)
- `ConflictLargerWins` - Larger file overwrites smaller
- `ConflictSourceWins` - First backend (backend1) always wins
- `ConflictDestWins` - Second backend (backend2) always wins
- `ConflictKeepBoth` - Keep both files with conflict suffix
- `ConflictSkip` - Skip conflicting files
- `ConflictError` - Record as error, don't resolve

### Check (Verify)

Verify files match between backends.

```go
// Simple check
inSync, _ := sync.Verify(ctx, src, dst, "data/", "backup/", sync.Options{})

// Detailed check
result, _ := sync.Check(ctx, src, dst, "data/", "backup/", sync.Options{})
fmt.Printf("Match: %d, Differ: %d, SrcOnly: %d, DstOnly: %d\n",
    len(result.Match), len(result.Differ), len(result.SrcOnly), len(result.DstOnly))

// Human-readable report
report, _ := sync.VerifyAndReport(ctx, src, dst, "data/", "backup/", sync.Options{})
fmt.Println(report)
```

### Options

```go
sync.Options{
    DeleteExtra:    true,   // Delete extra files in destination
    DryRun:         true,   // Report changes without making them
    Checksum:       true,   // Compare by checksum (slower but accurate)
    SizeOnly:       true,   // Compare by size only (fast)
    IgnoreExisting: true,   // Skip files that exist in destination
    MaxErrors:      10,     // Stop after N errors (0 = stop on first)
    Concurrency:    4,      // Concurrent transfers
    Progress: func(p sync.Progress) {
        fmt.Printf("%s: %d/%d files\n", p.Phase, p.FilesTransferred, p.TotalFiles)
    },
}
```

### Logging

Sync operations support structured logging via `*slog.Logger`.

```go
import (
    "log/slog"
    "os"
    "github.com/grokify/omnistorage/sync"
)

// With custom logger
result, _ := sync.Sync(ctx, src, dst, "data/", "backup/", sync.Options{
    Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })),
})

// Output includes:
// - Sync start/complete with summary
// - File scan progress
// - Copy/delete operations (at debug level)
// - Errors with context
```

When no logger is provided, a null logger is used (no output).

## Extended Interface

Backends may implement `ExtendedBackend` for additional capabilities.

```go
// Check if backend supports extended operations
if ext, ok := omnistorage.AsExtended(backend); ok {
    // Get file metadata
    info, _ := ext.Stat(ctx, "file.txt")
    fmt.Printf("Size: %d, Modified: %s\n", info.Size(), info.ModTime())

    // Server-side copy (no download/upload)
    if ext.Features().Copy {
        ext.Copy(ctx, "source.txt", "dest.txt")
    }

    // Server-side move
    if ext.Features().Move {
        ext.Move(ctx, "old.txt", "new.txt")
    }

    // Directory operations
    ext.Mkdir(ctx, "new-folder")
    ext.Rmdir(ctx, "empty-folder")
}
```

### Feature Discovery

```go
features := ext.Features()
if features.Copy {
    // Backend supports server-side copy
}
if features.Move {
    // Backend supports server-side move
}
```

## Compression

### Gzip

```go
import "github.com/grokify/omnistorage/compress/gzip"

// Write
gzWriter, _ := gzip.NewWriter(writer)
gzWriter.Write(data)
gzWriter.Close()

// Read
gzReader, _ := gzip.NewReader(reader)
data, _ := io.ReadAll(gzReader)
gzReader.Close()
```

### Zstandard

```go
import "github.com/grokify/omnistorage/compress/zstd"

// Write
zstdWriter, _ := zstd.NewWriter(writer)
zstdWriter.Write(data)
zstdWriter.Close()

// Read
zstdReader, _ := zstd.NewReader(reader)
data, _ := io.ReadAll(zstdReader)
zstdReader.Close()
```

## Interfaces

### Backend

The core interface for all storage backends.

```go
type Backend interface {
    NewWriter(ctx context.Context, path string, opts ...WriterOption) (io.WriteCloser, error)
    NewReader(ctx context.Context, path string, opts ...ReaderOption) (io.ReadCloser, error)
    Exists(ctx context.Context, path string) (bool, error)
    Delete(ctx context.Context, path string) error
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}
```

### ExtendedBackend

Extended interface for metadata and server-side operations.

```go
type ExtendedBackend interface {
    Backend
    Stat(ctx context.Context, path string) (ObjectInfo, error)
    Mkdir(ctx context.Context, path string) error
    Rmdir(ctx context.Context, path string) error
    Copy(ctx context.Context, src, dst string) error
    Move(ctx context.Context, src, dst string) error
    Features() Features
}
```

### RecordWriter / RecordReader

For streaming record-oriented data (logs, events, NDJSON).

```go
type RecordWriter interface {
    Write(data []byte) error
    Flush() error
    Close() error
}

type RecordReader interface {
    Read() ([]byte, error)
    Close() error
}
```

## Implementing a Backend

External packages can implement and register backends.

```go
package mybackend

import "github.com/grokify/omnistorage"

func init() {
    omnistorage.Register("mybackend", func(config map[string]string) (omnistorage.Backend, error) {
        return New(ConfigFromMap(config))
    })
}

type Backend struct { /* ... */ }

func (b *Backend) NewWriter(ctx context.Context, path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
    // Implementation
}

// ... implement other Backend methods
```

## Related Projects

- [omnistorage-google](https://github.com/grokify/omnistorage-google) - Google Drive and GCS backends
- [rclone](https://github.com/rclone/rclone) - Inspiration for backend coverage and sync capabilities
- [go-cloud](https://github.com/google/go-cloud) - Google's portable cloud APIs
- [afero](https://github.com/spf13/afero) - Filesystem abstraction

## Roadmap

See [ROADMAP.md](ROADMAP.md) for planned features including:

- Additional cloud backends (GCS, Azure, Dropbox, OneDrive)
- ~~Filtering system (glob patterns, size/age filters)~~ (implemented)
- ~~Transfer controls (bandwidth limiting, parallel transfers)~~ (implemented)
- ~~Bidirectional sync with conflict resolution~~ (implemented)
- ~~Structured logging via slog~~ (implemented)
- Security features (credential management, signed URLs)
- CLI tool

## Contributing

Contributions are welcome! Priority areas:

1. **New backends** - Follow `backend/file` as a template
2. **Tests** - Especially integration tests with real services
3. **Documentation** - Examples, guides, GoDoc improvements
4. **Bug fixes** - Issues labeled `good first issue`

## License

MIT License - see [LICENSE](LICENSE) for details.

 [build-status-svg]: https://github.com/grokify/omnistorage/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/grokify/omnistorage/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/grokify/omnistorage/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/grokify/omnistorage/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/grokify/omnistorage
 [goreport-url]: https://goreportcard.com/report/github.com/grokify/omnistorage
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/grokify/omnistorage
 [docs-godoc-url]: https://pkg.go.dev/github.com/grokify/omnistorage
 [viz-svg]: https://img.shields.io/badge/visualizaton-Go-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=grokify%2Fomnistorage
 [loc-svg]: https://tokei.rs/b1/github/grokify/omnistorage
 [repo-url]: https://github.com/grokify/omnistorage
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/grokify/omnistorage/blob/master/LICENSE