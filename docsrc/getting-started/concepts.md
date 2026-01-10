# Concepts

This page explains the core concepts and architecture of omnistorage.

## Layered Architecture

OmniStorage uses a layered architecture where each layer handles a specific concern:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│         (Your code: marshals domain types, logs, etc.)      │
├─────────────────────────────────────────────────────────────┤
│                     Format Layer                             │
│              (NDJSON, length-prefixed, CSV)                 │
│              Handles record framing/delimiting              │
├─────────────────────────────────────────────────────────────┤
│                   Compression Layer                          │
│                  (gzip, zstd, snappy)                       │
│              Optional compression/decompression             │
├─────────────────────────────────────────────────────────────┤
│                    Backend Layer                             │
│        (File, S3, GCS, Channel, Memory, etc.)               │
│              Raw byte transport/storage                      │
└─────────────────────────────────────────────────────────────┘
```

Each layer is independent and composable. You can mix and match:

- File backend + gzip compression + NDJSON format
- S3 backend + zstd compression + raw bytes
- Memory backend + no compression + CSV format

## Interface Composition

OmniStorage uses interface composition to support both simple and advanced use cases:

### Backend (Basic Interface)

The `Backend` interface provides core read/write operations:

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

This is sufficient for most applications.

### ExtendedBackend (Advanced Interface)

The `ExtendedBackend` interface adds metadata and server-side operations:

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

Use `AsExtended()` to check if a backend supports extended operations:

```go
if ext, ok := omnistorage.AsExtended(backend); ok {
    info, _ := ext.Stat(ctx, "file.txt")
    fmt.Printf("Size: %d bytes\n", info.Size())
}
```

## Backend Registration

Backends register themselves using the `Register()` function, typically in `init()`:

```go
// backend/file/backend.go
func init() {
    omnistorage.Register("file", NewFromConfig)
}
```

This allows configuration-driven backend selection:

```go
// Select backend from environment variable
backendType := os.Getenv("STORAGE_BACKEND") // "file", "s3", etc.
backend, _ := omnistorage.Open(backendType, config)
```

## Feature Discovery

Backends advertise their capabilities through the `Features` struct:

```go
type Features struct {
    Copy           bool // Server-side copy
    Move           bool // Server-side move
    Purge          bool // Recursive delete
    SetModTime     bool // Set modification time
    CustomMetadata bool // Custom metadata support
}
```

This allows code to adapt to backend capabilities:

```go
if ext.Features().Copy {
    // Use efficient server-side copy
    ext.Copy(ctx, src, dst)
} else {
    // Fall back to read + write
    omnistorage.CopyPath(ctx, backend, src, backend, dst)
}
```

## ObjectInfo

The `ObjectInfo` interface provides file metadata:

```go
type ObjectInfo interface {
    Name() string
    Size() int64
    ModTime() time.Time
    IsDir() bool
    Hash(HashType) string
    MimeType() string
    Metadata() map[string]string
}
```

## RecordWriter / RecordReader

For streaming record-oriented data (logs, events, NDJSON):

```go
type RecordWriter interface {
    Write(data []byte) error  // Write a single record
    Flush() error             // Flush buffered data
    Close() error
}

type RecordReader interface {
    Read() ([]byte, error)    // Read next record (io.EOF when done)
    Close() error
}
```

## Sync Engine

The sync package provides rclone-like file synchronization:

- **Sync** - Make destination match source (with optional deletes)
- **Copy** - Copy files without deleting extras
- **Move** - Move files from source to destination
- **Check** - Verify files match between backends

See [Sync Engine](../sync/index.md) for details.

## Multi-Writer

The multi package provides fan-out writing to multiple backends:

```go
mw, _ := multi.NewWriter(local, s3, gcs)
w, _ := mw.NewWriter(ctx, "data.json")
w.Write(data)  // Written to all three backends
w.Close()
```

See [Multi-Writer Guide](../guides/multi-writer.md) for details.

## Error Handling

OmniStorage defines standard errors for common cases:

```go
var (
    ErrNotFound        // Path does not exist
    ErrAlreadyExists   // Path already exists
    ErrPermissionDenied // Access denied
    ErrBackendClosed   // Backend has been closed
    ErrInvalidPath     // Invalid path format
    ErrWriterClosed    // Writer has been closed
    ErrReaderClosed    // Reader has been closed
)
```

Use `errors.Is()` to check:

```go
r, err := backend.NewReader(ctx, "missing.txt")
if errors.Is(err, omnistorage.ErrNotFound) {
    log.Println("File not found")
}
```

## Next Steps

- [Backends](../backends/index.md) - Learn about specific backends
- [Sync Engine](../sync/index.md) - File synchronization
- [Reference](../reference/interfaces.md) - Complete API reference
